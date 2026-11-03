package app

import (
	"net/http"

	swaggerDocs "pagliaro-precificador/docs"
)

// openapi godoc
// @Summary Contrato OpenAPI gerado
// @Tags Sistema
// @Produce json
// @Success 200 {object} object
// @Router /openapi.json [get]
func (a *App) openapi(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(swaggerDocs.SwaggerInfo.ReadDoc()))
}
