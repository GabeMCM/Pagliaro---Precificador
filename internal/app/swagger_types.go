package app

import (
	"pagliaro-precificador/internal/domain"
	"pagliaro-precificador/internal/pricing"
)

type healthResponse struct {
	Status string `json:"status" example:"ok"`
}

type errorBody struct {
	Code    string   `json:"code" example:"QUOTE_VALIDATION_ERROR"`
	Message string   `json:"message" example:"mensagem do erro"`
	Details []string `json:"details"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

type pricingTableListResponse struct {
	Items []domain.PricingTable `json:"items"`
}

type apiKeyListResponse struct {
	Items []domain.APIKey `json:"items"`
}

type requestLogListResponse struct {
	Items []domain.RequestLog `json:"items"`
}

type simulateTableItemDoc struct {
	Mutuality pricing.MutualityRequest `json:"mutualidade"`
	Result    domain.TableResult       `json:"result"`
}

type simulateTableResponseDoc struct {
	Items []simulateTableItemDoc `json:"items"`
}
