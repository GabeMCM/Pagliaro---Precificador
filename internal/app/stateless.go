package app

import (
	"fmt"
	"net/http"
	"strings"

	"pagliaro-precificador/internal/pricing"
)

// generateTableArtifact godoc
// @Summary Gerar tabela JSON stateless
// @Description Baixa dados oficiais do IBGE, calcula a tabela e retorna um JSON autocontido para download.
// @Tags Admin - Tabelas
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param payload body pricing.GenerateTableRequest true "Parâmetros de geração da tabela"
// @Success 200 {object} pricing.TableArtifact
// @Header 200 {string} Content-Disposition "attachment; filename=tabela-precos.json"
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 422 {object} errorResponse
// @Failure 502 {object} errorResponse
// @Router /v1/admin/tabelas/gerar-json [post]
func (a *App) generateTableArtifact(w http.ResponseWriter, r *http.Request) {
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
	result, err := pricing.GenerateTable(req, ibgeData, metadata)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "PRICING_TABLE_VALIDATION_ERROR", err.Error())
		return
	}
	artifact, err := pricing.BuildTableArtifact(req, result, ibgeData, a.cfg.TableSignatureSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TABLE_ARTIFACT_ERROR", err.Error())
		return
	}

	filename := safeJSONFilename(req.Version)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	writeJSON(w, http.StatusOK, artifact)
}

// calculateQuoteFromArtifact godoc
// @Summary Calcular orçamento com tabela JSON
// @Description Valida hash/assinatura da tabela enviada no payload e calcula o orçamento sem depender de banco.
// @Tags ERP - Orçamentos
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param payload body pricing.StatelessQuoteRequest true "Tabela JSON e integrantes do orçamento"
// @Success 200 {object} pricing.QuoteResult
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 422 {object} errorResponse
// @Router /v1/erp/orcamentos/calcular [post]
func (a *App) calculateQuoteFromArtifact(w http.ResponseWriter, r *http.Request) {
	var req pricing.StatelessQuoteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	result, err := pricing.GenerateQuoteFromArtifact(req, a.cfg.TableSignatureSecret)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "QUOTE_VALIDATION_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func safeJSONFilename(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "tabela-precos.json"
	}
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		" ", "-",
		`"`, "",
		"'", "",
	)
	return "tabela-precos-" + replacer.Replace(version) + ".json"
}
