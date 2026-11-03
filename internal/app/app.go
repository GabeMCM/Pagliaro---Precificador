package app

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/swaggo/http-swagger"

	"pagliaro-precificador/internal/auth"
	"pagliaro-precificador/internal/config"
	"pagliaro-precificador/internal/domain"
	"pagliaro-precificador/internal/ibge"
	"pagliaro-precificador/internal/id"
	"pagliaro-precificador/internal/pricing"
	"pagliaro-precificador/internal/storage"
)

type App struct {
	cfg          config.Config
	store        storage.Store
	ibgeProvider *ibge.Provider
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	var store storage.Store
	if cfg.UseMemoryDB {
		log.Printf("DATABASE_URL ausente; usando storage em memória para desenvolvimento")
		store = storage.NewMemoryStore()
	} else {
		postgresStore, err := storage.NewPostgresStore(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, err
		}
		store = postgresStore
	}

	return &App{
		cfg:          cfg,
		store:        store,
		ibgeProvider: ibge.NewProvider(),
	}, nil
}

func (a *App) Close() {
	a.store.Close()
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /docs", a.redirectDocs)
	mux.HandleFunc("GET /docs/", httpSwagger.Handler(httpSwagger.URL("/openapi.json")))
	mux.HandleFunc("GET /openapi.json", a.openapi)
	mux.HandleFunc("GET /health", a.health)
	mux.Handle("POST /v1/admin/api-keys", a.requireRole("admin", http.HandlerFunc(a.createAPIKey)))
	mux.Handle("GET /v1/admin/api-keys", a.requireRole("admin", http.HandlerFunc(a.listAPIKeys)))
	mux.Handle("POST /v1/admin/api-keys/{id}/revogar", a.requireRole("admin", http.HandlerFunc(a.revokeAPIKey)))
	mux.Handle("GET /v1/admin/logs/requisicoes", a.requireRole("admin", http.HandlerFunc(a.listRequestLogs)))
	mux.Handle("POST /v1/admin/tabelas/simular", a.requireRole("admin", http.HandlerFunc(a.simulateTable)))
	mux.Handle("POST /v1/admin/tabelas/gerar-json", a.requireRole("admin", http.HandlerFunc(a.generateTableArtifact)))
	mux.Handle("POST /v1/admin/tabelas/gerar", a.requireRole("admin", http.HandlerFunc(a.generateTable)))
	mux.Handle("GET /v1/admin/tabelas", a.requireRole("admin", http.HandlerFunc(a.listTables)))
	mux.Handle("GET /v1/admin/tabelas/ativa", a.requireRole("admin", http.HandlerFunc(a.getActiveTable)))
	mux.Handle("GET /v1/admin/tabelas/{id}", a.requireRole("admin", http.HandlerFunc(a.getTable)))
	mux.Handle("POST /v1/admin/tabelas/{id}/aprovar", a.requireRole("admin", http.HandlerFunc(a.approveTable)))
	mux.Handle("POST /v1/admin/tabelas/{id}/ativar", a.requireRole("admin", http.HandlerFunc(a.activateTable)))
	mux.Handle("POST /v1/admin/tabelas/{id}/arquivar", a.requireRole("admin", http.HandlerFunc(a.archiveTable)))
	mux.Handle("POST /v1/erp/orcamentos/calcular", a.requireRole("erp", http.HandlerFunc(a.calculateQuoteFromArtifact)))
	mux.Handle("POST /v1/erp/orcamentos", a.requireRole("erp", http.HandlerFunc(a.createQuote)))
	mux.Handle("GET /v1/erp/orcamentos/{id}", a.requireRole("erp", http.HandlerFunc(a.getQuote)))
	return a.withRequestLog(mux)
}

// health godoc
// @Summary Health check
// @Tags Sistema
// @Produce json
// @Success 200 {object} healthResponse
// @Router /health [get]
func (a *App) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (a *App) redirectDocs(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/docs/index.html", http.StatusFound)
}

// generateTable godoc
// @Summary Gerar tabela stateful
// @Description Baixa dados oficiais do IBGE, calcula a tabela e salva como draft no storage configurado.
// @Tags Admin - Tabelas
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param payload body pricing.GenerateTableRequest true "Parâmetros de geração da tabela"
// @Success 201 {object} domain.PricingTable
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 422 {object} errorResponse
// @Failure 502 {object} errorResponse
// @Router /v1/admin/tabelas/gerar [post]
func (a *App) generateTable(w http.ResponseWriter, r *http.Request) {
	var req pricing.GenerateTableRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req = pricing.NormalizeTableRequestYears(req)

	ibgeData, metadata, err := a.ibgeProvider.Fetch(
		r.Context(),
		req.CalculationYear,
		req.PopulationRevisionYear,
		req.MortalityTableYear,
		req.IBGE.PopulationURL,
		req.IBGE.MortalityURL,
	)
	if err != nil {
		writeError(w, http.StatusBadGateway, "IBGE_IMPORT_ERROR", err.Error())
		return
	}
	if err := a.store.SaveIBGEData(r.Context(), req.CalculationYear, ibgeData); err != nil {
		writeError(w, http.StatusInternalServerError, "IBGE_SAVE_ERROR", err.Error())
		return
	}

	result, err := pricing.GenerateTable(req, ibgeData, metadata)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "PRICING_TABLE_VALIDATION_ERROR", err.Error())
		return
	}

	now := time.Now().UTC()
	table := domain.PricingTable{
		ID:        id.New("tbl"),
		Version:   req.Version,
		IBGEYear:  req.CalculationYear,
		Status:    domain.TableDraft,
		Request:   req,
		Result:    result,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := a.store.SavePricingTable(r.Context(), table); err != nil {
		writeError(w, http.StatusInternalServerError, "PRICING_TABLE_SAVE_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, table)
}

// listTables godoc
// @Summary Listar tabelas
// @Tags Admin - Tabelas
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} pricingTableListResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Router /v1/admin/tabelas [get]
func (a *App) listTables(w http.ResponseWriter, r *http.Request) {
	tables, err := a.store.ListPricingTables(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PRICING_TABLE_LIST_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": tables})
}

// getTable godoc
// @Summary Consultar tabela por ID
// @Tags Admin - Tabelas
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "ID da tabela"
// @Success 200 {object} domain.PricingTable
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /v1/admin/tabelas/{id} [get]
func (a *App) getTable(w http.ResponseWriter, r *http.Request) {
	table, err := a.store.GetPricingTable(r.Context(), r.PathValue("id"))
	if err != nil {
		handleStorageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, table)
}

// getActiveTable godoc
// @Summary Consultar tabela ativa
// @Tags Admin - Tabelas
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} domain.PricingTable
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /v1/admin/tabelas/ativa [get]
func (a *App) getActiveTable(w http.ResponseWriter, r *http.Request) {
	table, err := a.store.GetActivePricingTable(r.Context())
	if err != nil {
		handleStorageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, table)
}

// approveTable godoc
// @Summary Aprovar tabela
// @Tags Admin - Tabelas
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "ID da tabela"
// @Success 200 {object} domain.PricingTable
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 422 {object} errorResponse
// @Router /v1/admin/tabelas/{id}/aprovar [post]
func (a *App) approveTable(w http.ResponseWriter, r *http.Request) {
	current, err := a.store.GetPricingTable(r.Context(), r.PathValue("id"))
	if err != nil {
		handleStorageError(w, err)
		return
	}
	if current.Status != domain.TableDraft {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_TABLE_STATUS", "somente tabelas draft podem ser aprovadas")
		return
	}
	table, err := a.store.SetPricingTableStatus(r.Context(), r.PathValue("id"), domain.TableApproved)
	if err != nil {
		handleStorageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, table)
}

// archiveTable godoc
// @Summary Arquivar tabela
// @Tags Admin - Tabelas
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "ID da tabela"
// @Success 200 {object} domain.PricingTable
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /v1/admin/tabelas/{id}/arquivar [post]
func (a *App) archiveTable(w http.ResponseWriter, r *http.Request) {
	table, err := a.store.SetPricingTableStatus(r.Context(), r.PathValue("id"), domain.TableArchived)
	if err != nil {
		handleStorageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, table)
}

// activateTable godoc
// @Summary Ativar tabela
// @Description Ativa uma tabela aprovada e arquiva a tabela ativa anterior.
// @Tags Admin - Tabelas
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "ID da tabela"
// @Success 200 {object} domain.PricingTable
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 422 {object} errorResponse
// @Router /v1/admin/tabelas/{id}/ativar [post]
func (a *App) activateTable(w http.ResponseWriter, r *http.Request) {
	current, err := a.store.GetPricingTable(r.Context(), r.PathValue("id"))
	if err != nil {
		handleStorageError(w, err)
		return
	}
	if current.Status != domain.TableApproved {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_TABLE_STATUS", "somente tabelas aprovadas podem ser ativadas")
		return
	}
	table, err := a.store.SetPricingTableStatus(r.Context(), r.PathValue("id"), domain.TableActive)
	if err != nil {
		handleStorageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, table)
}

// createQuote godoc
// @Summary Gerar orçamento stateful
// @Description Usa a tabela ativa salva no storage e persiste o orçamento por erp_orcamento_id.
// @Tags ERP - Orçamentos
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param payload body pricing.QuoteRequest true "Parâmetros do orçamento"
// @Success 201 {object} domain.QuoteRecord
// @Success 200 {object} domain.QuoteRecord
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 409 {object} errorResponse
// @Failure 422 {object} errorResponse
// @Router /v1/erp/orcamentos [post]
func (a *App) createQuote(w http.ResponseWriter, r *http.Request) {
	var req pricing.QuoteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	requestHash, err := hashJSON(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "REQUEST_HASH_ERROR", err.Error())
		return
	}
	if req.ERPQuoteID != "" {
		existing, err := a.store.GetQuoteByERPQuoteID(r.Context(), req.ERPQuoteID)
		if err == nil {
			if existing.RequestHash != "" && existing.RequestHash != requestHash {
				writeError(w, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "erp_orcamento_id já existe com payload diferente")
				return
			}
			writeJSON(w, http.StatusOK, existing)
			return
		}
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			handleStorageError(w, err)
			return
		}
	}

	table, err := a.store.GetActivePricingTable(r.Context())
	if err != nil {
		handleStorageError(w, err)
		return
	}
	ibgeData, err := a.store.GetIBGEData(r.Context(), table.IBGEYear)
	if err != nil {
		handleStorageError(w, err)
		return
	}
	result, err := pricing.GenerateQuote(req, table, ibgeData.MortalityQXByAge)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "QUOTE_VALIDATION_ERROR", err.Error())
		return
	}

	quote := domain.QuoteRecord{
		ID:          id.New("qte"),
		ERPQuoteID:  req.ERPQuoteID,
		RequestHash: requestHash,
		Request:     req,
		Result:      result,
		CreatedAt:   time.Now().UTC(),
	}
	if err := a.store.SaveQuote(r.Context(), quote); err != nil {
		writeError(w, http.StatusInternalServerError, "QUOTE_SAVE_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, quote)
}

// getQuote godoc
// @Summary Consultar orçamento
// @Tags ERP - Orçamentos
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "ID do orçamento"
// @Success 200 {object} domain.QuoteRecord
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /v1/erp/orcamentos/{id} [get]
func (a *App) getQuote(w http.ResponseWriter, r *http.Request) {
	quote, err := a.store.GetQuote(r.Context(), r.PathValue("id"))
	if err != nil {
		handleStorageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, quote)
}

func (a *App) requireRole(role string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		if key == "" {
			auth := r.Header.Get("Authorization")
			key = strings.TrimPrefix(auth, "Bearer ")
		}
		if key == "" {
			writeError(w, http.StatusUnauthorized, "MISSING_API_KEY", "API key não informada")
			return
		}
		if key == a.cfg.AdminAPIKey {
			a.markAuth(r, "bootstrap_admin", auth.RoleAdmin)
			next.ServeHTTP(w, r)
			return
		}
		if role == "erp" && key == a.cfg.ERPAPIKey {
			a.markAuth(r, "bootstrap_erp", auth.RoleERP)
			next.ServeHTTP(w, r)
			return
		}
		apiKey, err := a.store.GetAPIKeyByHash(r.Context(), auth.HashAPIKey(key))
		if err == nil && apiKey.Active && (apiKey.Role == auth.RoleAdmin || apiKey.Role == role) {
			a.markAuth(r, apiKey.ID, apiKey.Role)
			_ = a.store.TouchAPIKey(r.Context(), apiKey.ID)
			next.ServeHTTP(w, r)
			return
		}
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusInternalServerError, "API_KEY_LOOKUP_ERROR", err.Error())
			return
		}
		writeError(w, http.StatusForbidden, "FORBIDDEN", "API key sem permissão para esta rota")
	})
}

func (a *App) markAuth(r *http.Request, apiKeyID string, role string) {
	meta := currentRequestMeta(r.Context())
	if meta == nil {
		return
	}
	meta.APIKeyID = apiKeyID
	meta.Role = role
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
			"details": []string{},
		},
	})
}

func handleStorageError(w http.ResponseWriter, err error) {
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "registro não encontrado")
		return
	}
	writeError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
}
