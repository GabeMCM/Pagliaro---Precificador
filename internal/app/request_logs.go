package app

import (
	"net/http"
	"strconv"
)

// listRequestLogs godoc
// @Summary Listar logs de requisições
// @Tags Admin - Auditoria
// @Produce json
// @Security ApiKeyAuth
// @Param limite query int false "Quantidade máxima de registros, até 500"
// @Success 200 {object} requestLogListResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Router /v1/admin/logs/requisicoes [get]
func (a *App) listRequestLogs(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limite"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_QUERY", "limite deve ser numérico")
			return
		}
		limit = parsed
	}
	logs, err := a.store.ListRequestLogs(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "REQUEST_LOG_LIST_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": logs})
}
