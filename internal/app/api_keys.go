package app

import (
	"net/http"
	"time"

	"pagliaro-precificador/internal/auth"
	"pagliaro-precificador/internal/domain"
	"pagliaro-precificador/internal/id"
)

type createAPIKeyRequest struct {
	Name string `json:"nome"`
	Role string `json:"papel"`
}

type createAPIKeyResponse struct {
	Record domain.APIKey `json:"registro"`
	APIKey string        `json:"api_key"`
}

// createAPIKey godoc
// @Summary Criar API key
// @Description Cria chave admin ou erp. O segredo completo aparece apenas nesta resposta.
// @Tags Admin - API Keys
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param payload body createAPIKeyRequest true "Dados da chave"
// @Success 201 {object} createAPIKeyResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 422 {object} errorResponse
// @Router /v1/admin/api-keys [post]
func (a *App) createAPIKey(w http.ResponseWriter, r *http.Request) {
	var req createAPIKeyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "API_KEY_VALIDATION_ERROR", "nome deve ser informado")
		return
	}
	plainKey, hash, prefix, err := auth.GenerateAPIKey(req.Role)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "API_KEY_VALIDATION_ERROR", err.Error())
		return
	}
	apiKey := domain.APIKey{
		ID:        id.New("key"),
		Name:      req.Name,
		Role:      req.Role,
		KeyHash:   hash,
		KeyPrefix: prefix,
		Active:    true,
		CreatedAt: time.Now().UTC(),
	}
	if err := a.store.SaveAPIKey(r.Context(), apiKey); err != nil {
		writeError(w, http.StatusInternalServerError, "API_KEY_SAVE_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, createAPIKeyResponse{Record: apiKey, APIKey: plainKey})
}

// listAPIKeys godoc
// @Summary Listar API keys
// @Tags Admin - API Keys
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} apiKeyListResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Router /v1/admin/api-keys [get]
func (a *App) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := a.store.ListAPIKeys(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "API_KEY_LIST_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": keys})
}

// revokeAPIKey godoc
// @Summary Revogar API key
// @Tags Admin - API Keys
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "ID da API key"
// @Success 200 {object} domain.APIKey
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /v1/admin/api-keys/{id}/revogar [post]
func (a *App) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	key, err := a.store.RevokeAPIKey(r.Context(), r.PathValue("id"))
	if err != nil {
		handleStorageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, key)
}
