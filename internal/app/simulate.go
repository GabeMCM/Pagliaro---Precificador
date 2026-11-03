package app

import (
	"net/http"

	"pagliaro-precificador/internal/pricing"
)

type simulateTableRequest struct {
	pricing.GenerateTableRequest
	Scenarios []pricing.MutualityRequest `json:"cenarios,omitempty"`
}

type simulateTableResponse struct {
	Items []simulateTableItem `json:"items"`
}

type simulateTableItem struct {
	Mutuality pricing.MutualityRequest `json:"mutualidade"`
	Result    any                      `json:"result"`
}

// simulateTable godoc
// @Summary Simular cenários de tabela
// @Description Calcula cenários de mutualidade sem salvar tabela no storage.
// @Tags Admin - Tabelas
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param payload body simulateTableRequest true "Parâmetros de simulação"
// @Success 200 {object} simulateTableResponseDoc
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 422 {object} errorResponse
// @Failure 502 {object} errorResponse
// @Router /v1/admin/tabelas/simular [post]
func (a *App) simulateTable(w http.ResponseWriter, r *http.Request) {
	var req simulateTableRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	base := pricing.NormalizeTableRequestYears(req.GenerateTableRequest)
	if len(req.Scenarios) == 0 {
		req.Scenarios = []pricing.MutualityRequest{
			{Method: pricing.MutualitySimple, Factor: 0.30},
			{Method: pricing.MutualityPopulation, Factor: 0.30},
			{Method: pricing.MutualityMortality, Factor: 0.30},
			{Method: pricing.MutualityPopulation, Factor: 0.35},
			{Method: pricing.MutualityMortality, Factor: 0.35},
		}
	}

	ibgeData, metadata, err := a.ibgeProvider.Fetch(
		r.Context(),
		base.CalculationYear,
		base.PopulationRevisionYear,
		base.MortalityTableYear,
		base.IBGE.PopulationURL,
		base.IBGE.MortalityURL,
	)
	if err != nil {
		writeError(w, http.StatusBadGateway, "IBGE_IMPORT_ERROR", err.Error())
		return
	}

	response := simulateTableResponse{Items: make([]simulateTableItem, 0, len(req.Scenarios))}
	for _, scenario := range req.Scenarios {
		scenarioReq := base
		scenarioReq.Mutuality = scenario
		result, err := pricing.GenerateTable(scenarioReq, ibgeData, metadata)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "SIMULATION_VALIDATION_ERROR", err.Error())
			return
		}
		response.Items = append(response.Items, simulateTableItem{
			Mutuality: scenario,
			Result:    result,
		})
	}
	writeJSON(w, http.StatusOK, response)
}
