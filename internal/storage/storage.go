package storage

import (
	"context"

	"pagliaro-precificador/internal/domain"
	"pagliaro-precificador/internal/pricing"
)

type Store interface {
	Close()
	SaveIBGEData(ctx context.Context, year int, data pricing.IBGEData) error
	GetIBGEData(ctx context.Context, year int) (pricing.IBGEData, error)
	SavePricingTable(ctx context.Context, table domain.PricingTable) error
	ListPricingTables(ctx context.Context) ([]domain.PricingTable, error)
	GetPricingTable(ctx context.Context, id string) (domain.PricingTable, error)
	GetActivePricingTable(ctx context.Context) (domain.PricingTable, error)
	SetPricingTableStatus(ctx context.Context, id string, status domain.TableStatus) (domain.PricingTable, error)
	SaveAPIKey(ctx context.Context, key domain.APIKey) error
	ListAPIKeys(ctx context.Context) ([]domain.APIKey, error)
	GetAPIKeyByHash(ctx context.Context, hash string) (domain.APIKey, error)
	RevokeAPIKey(ctx context.Context, id string) (domain.APIKey, error)
	TouchAPIKey(ctx context.Context, id string) error
	SaveQuote(ctx context.Context, quote domain.QuoteRecord) error
	GetQuote(ctx context.Context, id string) (domain.QuoteRecord, error)
	GetQuoteByERPQuoteID(ctx context.Context, erpQuoteID string) (domain.QuoteRecord, error)
	SaveRequestLog(ctx context.Context, log domain.RequestLog) error
	ListRequestLogs(ctx context.Context, limit int) ([]domain.RequestLog, error)
}
