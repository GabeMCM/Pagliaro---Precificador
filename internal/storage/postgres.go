package storage

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pagliaro-precificador/internal/domain"
	"pagliaro-precificador/internal/pricing"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) Close() {
	s.pool.Close()
}

func (s *PostgresStore) SaveIBGEData(ctx context.Context, year int, data pricing.IBGEData) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		insert into ibge_data (year, data, updated_at)
		values ($1, $2, now())
		on conflict (year) do update set data = excluded.data, updated_at = now()
	`, year, raw)
	return err
}

func (s *PostgresStore) GetIBGEData(ctx context.Context, year int) (pricing.IBGEData, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `select data from ibge_data where year = $1`, year).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return pricing.IBGEData{}, ErrNotFound
	}
	if err != nil {
		return pricing.IBGEData{}, err
	}
	var data pricing.IBGEData
	if err := json.Unmarshal(raw, &data); err != nil {
		return pricing.IBGEData{}, err
	}
	return data, nil
}

func (s *PostgresStore) SavePricingTable(ctx context.Context, table domain.PricingTable) error {
	request, err := json.Marshal(table.Request)
	if err != nil {
		return err
	}
	result, err := json.Marshal(table.Result)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		insert into pricing_table_versions
			(id, version, ibge_year, status, request, result, created_at, updated_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
	`, table.ID, table.Version, table.IBGEYear, table.Status, request, result, table.CreatedAt, table.UpdatedAt)
	return err
}

func (s *PostgresStore) ListPricingTables(ctx context.Context) ([]domain.PricingTable, error) {
	rows, err := s.pool.Query(ctx, `
		select id, version, ibge_year, status, request, result, created_at, updated_at
		from pricing_table_versions
		order by created_at desc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []domain.PricingTable
	for rows.Next() {
		table, err := scanPricingTable(rows)
		if err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, rows.Err()
}

func (s *PostgresStore) GetPricingTable(ctx context.Context, id string) (domain.PricingTable, error) {
	row := s.pool.QueryRow(ctx, `
		select id, version, ibge_year, status, request, result, created_at, updated_at
		from pricing_table_versions
		where id = $1
	`, id)
	return scanPricingTable(row)
}

func (s *PostgresStore) GetActivePricingTable(ctx context.Context) (domain.PricingTable, error) {
	row := s.pool.QueryRow(ctx, `
		select id, version, ibge_year, status, request, result, created_at, updated_at
		from pricing_table_versions
		where status = 'active'
		order by updated_at desc
		limit 1
	`)
	return scanPricingTable(row)
}

func (s *PostgresStore) SetPricingTableStatus(ctx context.Context, id string, status domain.TableStatus) (domain.PricingTable, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.PricingTable{}, err
	}
	defer tx.Rollback(ctx)

	if status == domain.TableActive {
		if _, err := tx.Exec(ctx, `update pricing_table_versions set status = 'archived', updated_at = now() where status = 'active' and id <> $1`, id); err != nil {
			return domain.PricingTable{}, err
		}
	}
	_, err = tx.Exec(ctx, `update pricing_table_versions set status = $2, updated_at = now() where id = $1`, id, status)
	if err != nil {
		return domain.PricingTable{}, err
	}
	row := tx.QueryRow(ctx, `
		select id, version, ibge_year, status, request, result, created_at, updated_at
		from pricing_table_versions
		where id = $1
	`, id)
	table, err := scanPricingTable(row)
	if err != nil {
		return domain.PricingTable{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PricingTable{}, err
	}
	return table, nil
}

func (s *PostgresStore) SaveQuote(ctx context.Context, quote domain.QuoteRecord) error {
	request, err := json.Marshal(quote.Request)
	if err != nil {
		return err
	}
	result, err := json.Marshal(quote.Result)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		insert into quote_results (id, erp_quote_id, request_hash, request, result, created_at)
		values ($1, $2, $3, $4, $5, $6)
	`, quote.ID, quote.ERPQuoteID, quote.RequestHash, request, result, quote.CreatedAt)
	return err
}

func (s *PostgresStore) GetQuote(ctx context.Context, id string) (domain.QuoteRecord, error) {
	return s.getQuoteBy(ctx, "id = $1", id)
}

func (s *PostgresStore) GetQuoteByERPQuoteID(ctx context.Context, erpQuoteID string) (domain.QuoteRecord, error) {
	return s.getQuoteBy(ctx, "erp_quote_id = $1", erpQuoteID)
}

func (s *PostgresStore) getQuoteBy(ctx context.Context, where string, value string) (domain.QuoteRecord, error) {
	var quote domain.QuoteRecord
	var request, result []byte
	err := s.pool.QueryRow(ctx, `
		select id, erp_quote_id, request_hash, request, result, created_at
		from quote_results
		where `+where+`
		order by created_at desc
		limit 1
	`, value).Scan(&quote.ID, &quote.ERPQuoteID, &quote.RequestHash, &request, &result, &quote.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.QuoteRecord{}, ErrNotFound
	}
	if err != nil {
		return domain.QuoteRecord{}, err
	}
	_ = json.Unmarshal(request, &quote.Request)
	_ = json.Unmarshal(result, &quote.Result)
	return quote, nil
}

func (s *PostgresStore) SaveAPIKey(ctx context.Context, key domain.APIKey) error {
	_, err := s.pool.Exec(ctx, `
		insert into api_keys (id, name, role, key_hash, key_prefix, active, created_at, revoked_at, last_used_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, key.ID, key.Name, key.Role, key.KeyHash, key.KeyPrefix, key.Active, key.CreatedAt, key.RevokedAt, key.LastUsedAt)
	return err
}

func (s *PostgresStore) ListAPIKeys(ctx context.Context) ([]domain.APIKey, error) {
	rows, err := s.pool.Query(ctx, `
		select id, name, role, key_hash, key_prefix, active, created_at, revoked_at, last_used_at
		from api_keys
		order by created_at desc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []domain.APIKey
	for rows.Next() {
		var key domain.APIKey
		if err := rows.Scan(&key.ID, &key.Name, &key.Role, &key.KeyHash, &key.KeyPrefix, &key.Active, &key.CreatedAt, &key.RevokedAt, &key.LastUsedAt); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *PostgresStore) GetAPIKeyByHash(ctx context.Context, hash string) (domain.APIKey, error) {
	var key domain.APIKey
	err := s.pool.QueryRow(ctx, `
		select id, name, role, key_hash, key_prefix, active, created_at, revoked_at, last_used_at
		from api_keys
		where key_hash = $1
	`, hash).Scan(&key.ID, &key.Name, &key.Role, &key.KeyHash, &key.KeyPrefix, &key.Active, &key.CreatedAt, &key.RevokedAt, &key.LastUsedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.APIKey{}, ErrNotFound
	}
	return key, err
}

func (s *PostgresStore) RevokeAPIKey(ctx context.Context, id string) (domain.APIKey, error) {
	row := s.pool.QueryRow(ctx, `
		update api_keys
		set active = false, revoked_at = now()
		where id = $1
		returning id, name, role, key_hash, key_prefix, active, created_at, revoked_at, last_used_at
	`, id)
	var key domain.APIKey
	err := row.Scan(&key.ID, &key.Name, &key.Role, &key.KeyHash, &key.KeyPrefix, &key.Active, &key.CreatedAt, &key.RevokedAt, &key.LastUsedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.APIKey{}, ErrNotFound
	}
	return key, err
}

func (s *PostgresStore) TouchAPIKey(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `update api_keys set last_used_at = now() where id = $1`, id)
	return err
}

func (s *PostgresStore) SaveRequestLog(ctx context.Context, requestLog domain.RequestLog) error {
	_, err := s.pool.Exec(ctx, `
		insert into request_logs
			(id, request_id, method, path, status, duration_ms, api_key_id, role, remote_addr, created_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, requestLog.ID, requestLog.RequestID, requestLog.Method, requestLog.Path, requestLog.Status, requestLog.DurationMS, nullableString(requestLog.APIKeyID), nullableString(requestLog.Role), requestLog.RemoteAddr, requestLog.CreatedAt)
	return err
}

func (s *PostgresStore) ListRequestLogs(ctx context.Context, limit int) ([]domain.RequestLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		select id, request_id, method, path, status, duration_ms, coalesce(api_key_id, ''), coalesce(role, ''), coalesce(remote_addr, ''), created_at
		from request_logs
		order by created_at desc
		limit $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := []domain.RequestLog{}
	for rows.Next() {
		var requestLog domain.RequestLog
		if err := rows.Scan(
			&requestLog.ID,
			&requestLog.RequestID,
			&requestLog.Method,
			&requestLog.Path,
			&requestLog.Status,
			&requestLog.DurationMS,
			&requestLog.APIKeyID,
			&requestLog.Role,
			&requestLog.RemoteAddr,
			&requestLog.CreatedAt,
		); err != nil {
			return nil, err
		}
		logs = append(logs, requestLog)
	}
	return logs, rows.Err()
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

type pricingTableScanner interface {
	Scan(dest ...any) error
}

func scanPricingTable(row pricingTableScanner) (domain.PricingTable, error) {
	var table domain.PricingTable
	var status string
	var request, result []byte
	err := row.Scan(&table.ID, &table.Version, &table.IBGEYear, &status, &request, &result, &table.CreatedAt, &table.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PricingTable{}, ErrNotFound
	}
	if err != nil {
		return domain.PricingTable{}, err
	}
	table.Status = domain.TableStatus(status)
	var requestPayload any
	_ = json.Unmarshal(request, &requestPayload)
	table.Request = requestPayload
	if err := json.Unmarshal(result, &table.Result); err != nil {
		return domain.PricingTable{}, err
	}
	if table.CreatedAt.IsZero() {
		table.CreatedAt = time.Now().UTC()
	}
	return table, nil
}
