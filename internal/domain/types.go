package domain

import "time"

type TableStatus string

const (
	TableDraft    TableStatus = "draft"
	TableApproved TableStatus = "approved"
	TableActive   TableStatus = "active"
	TableArchived TableStatus = "archived"
)

type PricingTable struct {
	ID        string      `json:"id"`
	Version   string      `json:"version"`
	IBGEYear  int         `json:"ibge_year"`
	Status    TableStatus `json:"status"`
	Request   any         `json:"request"`
	Result    TableResult `json:"result"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type TableResult struct {
	Version     string            `json:"version"`
	IBGEYear    int               `json:"ibge_year"`
	Rows        []PricingTableRow `json:"rows"`
	Summary     TableSummary      `json:"summary"`
	Warnings    []string          `json:"warnings,omitempty"`
	GeneratedAt time.Time         `json:"generated_at"`
}

type TableSummary struct {
	UnderageQX      float64            `json:"qx_0_17"`
	MutualityMethod string             `json:"mutuality_method"`
	MutualityFactor float64            `json:"mutuality_factor"`
	ReferenceByPlan map[string]float64 `json:"reference_by_plan"`
	OperationalCost float64            `json:"operational_cost"`
	BankFee         float64            `json:"bank_fee"`
	ProfitRate      float64            `json:"profit_rate"`
	CommissionRate  float64            `json:"commission_rate"`
	Rounding        string             `json:"rounding"`
	IBGE            *IBGEMetadata      `json:"ibge,omitempty"`
}

type IBGEMetadata struct {
	CalculationYear        int    `json:"ano_calculo"`
	PopulationRevisionYear int    `json:"revisao_populacional"`
	MortalityTableYear     int    `json:"ano_tabua_mortalidade"`
	PopulationURL          string `json:"population_url"`
	MortalityURL           string `json:"mortality_url"`
	PopulationSHA256       string `json:"population_sha256"`
	MortalitySHA256        string `json:"mortality_sha256"`
	PopulationAges         int    `json:"population_ages"`
	MortalityAges          int    `json:"mortality_ages"`
}

type PricingTableRow struct {
	PlanName                string  `json:"plan_name"`
	Coverage                float64 `json:"coverage"`
	AgeBand                 string  `json:"age_band"`
	Population              float64 `json:"population"`
	QXBand                  float64 `json:"qx_band"`
	QXUnderage              float64 `json:"qx_0_17"`
	QA                      float64 `json:"qA"`
	ActuarialCost           float64 `json:"actuarial_cost"`
	ReferenceActuarialCost  float64 `json:"reference_actuarial_cost"`
	MutualizedActuarialCost float64 `json:"mutualized_actuarial_cost"`
	OperationalCost         float64 `json:"operational_cost"`
	BankFee                 float64 `json:"bank_fee"`
	TechnicalCost           float64 `json:"technical_cost"`
	BaseMonthlyPrice        float64 `json:"base_monthly_price"`
}

type QuoteRecord struct {
	ID          string    `json:"id"`
	ERPQuoteID  string    `json:"erp_quote_id"`
	RequestHash string    `json:"request_hash"`
	Request     any       `json:"request"`
	Result      any       `json:"result"`
	CreatedAt   time.Time `json:"created_at"`
}

type APIKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"nome"`
	Role       string     `json:"papel"`
	KeyHash    string     `json:"-"`
	KeyPrefix  string     `json:"prefixo"`
	Active     bool       `json:"ativa"`
	CreatedAt  time.Time  `json:"criada_em"`
	RevokedAt  *time.Time `json:"revogada_em,omitempty"`
	LastUsedAt *time.Time `json:"ultimo_uso_em,omitempty"`
}

type RequestLog struct {
	ID         string    `json:"id"`
	RequestID  string    `json:"request_id"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Status     int       `json:"status"`
	DurationMS int64     `json:"duration_ms"`
	APIKeyID   string    `json:"api_key_id,omitempty"`
	Role       string    `json:"role,omitempty"`
	RemoteAddr string    `json:"remote_addr,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
