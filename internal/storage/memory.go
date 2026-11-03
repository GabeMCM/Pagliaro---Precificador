package storage

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"pagliaro-precificador/internal/domain"
	"pagliaro-precificador/internal/pricing"
)

var ErrNotFound = errors.New("registro não encontrado")

type MemoryStore struct {
	mu          sync.RWMutex
	ibge        map[int]pricing.IBGEData
	tables      map[string]domain.PricingTable
	quotes      map[string]domain.QuoteRecord
	apiKeys     map[string]domain.APIKey
	requestLogs []domain.RequestLog
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		ibge:    map[int]pricing.IBGEData{},
		tables:  map[string]domain.PricingTable{},
		quotes:  map[string]domain.QuoteRecord{},
		apiKeys: map[string]domain.APIKey{},
	}
}

func (s *MemoryStore) Close() {}

func (s *MemoryStore) SaveIBGEData(_ context.Context, year int, data pricing.IBGEData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ibge[year] = data
	return nil
}

func (s *MemoryStore) GetIBGEData(_ context.Context, year int) (pricing.IBGEData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.ibge[year]
	if !ok {
		return pricing.IBGEData{}, ErrNotFound
	}
	return data, nil
}

func (s *MemoryStore) SavePricingTable(_ context.Context, table domain.PricingTable) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tables[table.ID] = table
	return nil
}

func (s *MemoryStore) ListPricingTables(_ context.Context) ([]domain.PricingTable, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tables := make([]domain.PricingTable, 0, len(s.tables))
	for _, table := range s.tables {
		tables = append(tables, table)
	}
	return tables, nil
}

func (s *MemoryStore) GetPricingTable(_ context.Context, id string) (domain.PricingTable, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	table, ok := s.tables[id]
	if !ok {
		return domain.PricingTable{}, ErrNotFound
	}
	return table, nil
}

func (s *MemoryStore) GetActivePricingTable(_ context.Context) (domain.PricingTable, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, table := range s.tables {
		if table.Status == domain.TableActive {
			return table, nil
		}
	}
	return domain.PricingTable{}, ErrNotFound
}

func (s *MemoryStore) SetPricingTableStatus(_ context.Context, id string, status domain.TableStatus) (domain.PricingTable, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	table, ok := s.tables[id]
	if !ok {
		return domain.PricingTable{}, ErrNotFound
	}
	if status == domain.TableActive {
		for otherID, otherTable := range s.tables {
			if otherTable.Status == domain.TableActive && otherID != id {
				otherTable.Status = domain.TableArchived
				otherTable.UpdatedAt = time.Now().UTC()
				s.tables[otherID] = otherTable
			}
		}
	}
	table.Status = status
	table.UpdatedAt = time.Now().UTC()
	s.tables[id] = table
	return table, nil
}

func (s *MemoryStore) SaveQuote(_ context.Context, quote domain.QuoteRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.quotes[quote.ID] = quote
	return nil
}

func (s *MemoryStore) SaveAPIKey(_ context.Context, key domain.APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apiKeys[key.ID] = key
	return nil
}

func (s *MemoryStore) ListAPIKeys(_ context.Context) ([]domain.APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]domain.APIKey, 0, len(s.apiKeys))
	for _, key := range s.apiKeys {
		keys = append(keys, key)
	}
	return keys, nil
}

func (s *MemoryStore) GetAPIKeyByHash(_ context.Context, hash string) (domain.APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, key := range s.apiKeys {
		if key.KeyHash == hash {
			return key, nil
		}
	}
	return domain.APIKey{}, ErrNotFound
}

func (s *MemoryStore) RevokeAPIKey(_ context.Context, id string) (domain.APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key, ok := s.apiKeys[id]
	if !ok {
		return domain.APIKey{}, ErrNotFound
	}
	now := time.Now().UTC()
	key.Active = false
	key.RevokedAt = &now
	s.apiKeys[id] = key
	return key, nil
}

func (s *MemoryStore) TouchAPIKey(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key, ok := s.apiKeys[id]
	if !ok {
		return ErrNotFound
	}
	now := time.Now().UTC()
	key.LastUsedAt = &now
	s.apiKeys[id] = key
	return nil
}

func (s *MemoryStore) GetQuote(_ context.Context, id string) (domain.QuoteRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	quote, ok := s.quotes[id]
	if !ok {
		return domain.QuoteRecord{}, ErrNotFound
	}
	return quote, nil
}

func (s *MemoryStore) GetQuoteByERPQuoteID(_ context.Context, erpQuoteID string) (domain.QuoteRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, quote := range s.quotes {
		if quote.ERPQuoteID == erpQuoteID {
			return quote, nil
		}
	}
	return domain.QuoteRecord{}, ErrNotFound
}

func (s *MemoryStore) SaveRequestLog(_ context.Context, requestLog domain.RequestLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requestLogs = append(s.requestLogs, requestLog)
	return nil
}

func (s *MemoryStore) ListRequestLogs(_ context.Context, limit int) ([]domain.RequestLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	logs := slices.Clone(s.requestLogs)
	slices.SortFunc(logs, func(a, b domain.RequestLog) int {
		return b.CreatedAt.Compare(a.CreatedAt)
	})
	if len(logs) > limit {
		logs = logs[:limit]
	}
	return logs, nil
}
