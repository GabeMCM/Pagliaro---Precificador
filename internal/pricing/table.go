package pricing

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"pagliaro-precificador/internal/domain"
)

const (
	MutualitySimple     = "simples"
	MutualityPopulation = "populacional"
	MutualityMortality  = "mortalidade"
)

var AgeBands = []AgeBand{
	{Name: "18-39", Start: 18, End: 39},
	{Name: "40-54", Start: 40, End: 54},
	{Name: "55-64", Start: 55, End: 64},
	{Name: "65-74", Start: 65, End: 74},
	{Name: "75-84", Start: 75, End: 84},
	{Name: "85+", Start: 85, End: 200},
}

type AgeBand struct {
	Name  string
	Start int
	End   int
}

type IBGEData struct {
	MortalityQXByAge map[int]float64
	PopulationByAge  map[int]float64
}

type GenerateTableRequest struct {
	CalculationYear        int               `json:"ano_calculo"`
	PopulationRevisionYear int               `json:"revisao_populacional,omitempty"`
	MortalityTableYear     int               `json:"ano_tabua_mortalidade,omitempty"`
	IBGEYear               int               `json:"ano_ibge,omitempty"`
	Version                string            `json:"versao"`
	Plans                  []PlanRequest     `json:"planos"`
	Costs                  CostsRequest      `json:"custos"`
	Margins                MarginsRequest    `json:"margens"`
	Mutuality              MutualityRequest  `json:"mutualidade"`
	IBGE                   IBGESourceRequest `json:"ibge,omitempty"`
}

type IBGESourceRequest struct {
	PopulationURL string `json:"population_url,omitempty"`
	MortalityURL  string `json:"mortality_url,omitempty"`
}

type IBGEMetadata struct {
	CalculationYear        int
	PopulationRevisionYear int
	MortalityTableYear     int
	PopulationURL          string
	MortalityURL           string
	PopulationSHA256       string
	MortalitySHA256        string
}

type PlanRequest struct {
	Name     string  `json:"nome"`
	Coverage float64 `json:"cobertura"`
}

type CostsRequest struct {
	FixedMonthlyExpense float64 `json:"gasto_fixo_mensal"`
	TotalClients        int     `json:"clientes_totais"`
	DelinquentClients   int     `json:"clientes_inadimplentes"`
	BankFee             float64 `json:"taxa_bancaria"`
}

type MarginsRequest struct {
	ProfitRate     float64 `json:"lucro"`
	CommissionRate float64 `json:"comissao"`
}

type MutualityRequest struct {
	Method string  `json:"metodo"`
	Factor float64 `json:"fator"`
}

type bandMetrics struct {
	Band          AgeBand
	Population    float64
	QX            float64
	Deaths        float64
	ActuarialCost float64
}

func GenerateTable(req GenerateTableRequest, data IBGEData, metadata *IBGEMetadata) (domain.TableResult, error) {
	req = NormalizeTableRequestYears(req)
	if req.CalculationYear <= 0 {
		return domain.TableResult{}, errors.New("ano_calculo deve ser informado")
	}
	if req.Version == "" {
		return domain.TableResult{}, errors.New("versao deve ser informada")
	}
	if len(req.Plans) == 0 {
		return domain.TableResult{}, errors.New("ao menos um plano deve ser informado")
	}
	if req.Mutuality.Method == "" {
		req.Mutuality.Method = MutualityPopulation
	}
	if req.Mutuality.Factor < 0 || req.Mutuality.Factor > 1 {
		return domain.TableResult{}, errors.New("fator de mutualidade deve estar entre 0 e 1")
	}

	payingClients := req.Costs.TotalClients - req.Costs.DelinquentClients
	if payingClients <= 0 {
		return domain.TableResult{}, errors.New("clientes adimplentes deve ser maior que zero")
	}
	operationalCost, _ := decimal.NewFromFloat(req.Costs.FixedMonthlyExpense).Div(decimal.NewFromInt(int64(payingClients))).Float64()
	qxUnderage, err := weightedQX(0, 17, data)
	if err != nil {
		return domain.TableResult{}, fmt.Errorf("calculando qx 0-17: %w", err)
	}
	baseBandMetrics, err := calculateBandMetrics(data)
	if err != nil {
		return domain.TableResult{}, err
	}

	rows := make([]domain.PricingTableRow, 0, len(req.Plans)*len(AgeBands))
	referenceByPlan := map[string]float64{}
	for _, plan := range req.Plans {
		if plan.Name == "" {
			return domain.TableResult{}, errors.New("nome do plano deve ser informado")
		}
		if plan.Coverage <= 0 {
			return domain.TableResult{}, fmt.Errorf("cobertura do plano %s deve ser maior que zero", plan.Name)
		}

		metrics := cloneMetrics(baseBandMetrics)
		for i := range metrics {
			qA := metrics[i].QX + qxUnderage
			metrics[i].ActuarialCost, _ = decimal.NewFromFloat(plan.Coverage).
				Mul(decimal.NewFromFloat(qA)).
				Div(decimal.NewFromInt(12)).
				Float64()
		}

		reference, err := referenceActuarialCost(metrics, req.Mutuality.Method)
		if err != nil {
			return domain.TableResult{}, err
		}
		referenceByPlan[plan.Name] = CeilMoney(reference)

		for _, metric := range metrics {
			qA := metric.QX + qxUnderage
			mutualizedDecimal := decimal.NewFromFloat(metric.ActuarialCost).
				Mul(decimal.NewFromFloat(1 - req.Mutuality.Factor)).
				Add(decimal.NewFromFloat(reference).Mul(decimal.NewFromFloat(req.Mutuality.Factor)))
			technicalDecimal := mutualizedDecimal.
				Add(decimal.NewFromFloat(operationalCost)).
				Add(decimal.NewFromFloat(req.Costs.BankFee))
			basePriceDecimal := technicalDecimal.Mul(decimal.NewFromFloat(1 + req.Margins.ProfitRate + req.Margins.CommissionRate))
			mutualizedActuarialCost, _ := mutualizedDecimal.Float64()
			technicalCost, _ := technicalDecimal.Float64()
			basePrice, _ := basePriceDecimal.Float64()
			rows = append(rows, domain.PricingTableRow{
				PlanName:                plan.Name,
				Coverage:                CeilMoney(plan.Coverage),
				AgeBand:                 metric.Band.Name,
				Population:              metric.Population,
				QXBand:                  metric.QX,
				QXUnderage:              qxUnderage,
				QA:                      qA,
				ActuarialCost:           CeilMoney(metric.ActuarialCost),
				ReferenceActuarialCost:  CeilMoney(reference),
				MutualizedActuarialCost: CeilMoney(mutualizedActuarialCost),
				OperationalCost:         CeilMoney(operationalCost),
				BankFee:                 CeilMoney(req.Costs.BankFee),
				TechnicalCost:           CeilMoney(technicalCost),
				BaseMonthlyPrice:        CeilMoney(basePrice),
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].PlanName == rows[j].PlanName {
			return bandIndex(rows[i].AgeBand) < bandIndex(rows[j].AgeBand)
		}
		return rows[i].PlanName < rows[j].PlanName
	})

	var ibgeMetadata *domain.IBGEMetadata
	if metadata != nil {
		ibgeMetadata = &domain.IBGEMetadata{
			PopulationURL:          metadata.PopulationURL,
			MortalityURL:           metadata.MortalityURL,
			PopulationSHA256:       metadata.PopulationSHA256,
			MortalitySHA256:        metadata.MortalitySHA256,
			CalculationYear:        metadata.CalculationYear,
			PopulationRevisionYear: metadata.PopulationRevisionYear,
			MortalityTableYear:     metadata.MortalityTableYear,
			PopulationAges:         len(data.PopulationByAge),
			MortalityAges:          len(data.MortalityQXByAge),
		}
	}

	return domain.TableResult{
		Version:  req.Version,
		IBGEYear: req.CalculationYear,
		Rows:     rows,
		Summary: domain.TableSummary{
			UnderageQX:      qxUnderage,
			MutualityMethod: req.Mutuality.Method,
			MutualityFactor: req.Mutuality.Factor,
			ReferenceByPlan: referenceByPlan,
			OperationalCost: CeilMoney(operationalCost),
			BankFee:         CeilMoney(req.Costs.BankFee),
			ProfitRate:      req.Margins.ProfitRate,
			CommissionRate:  req.Margins.CommissionRate,
			Rounding:        "ceil_2_decimals",
			IBGE:            ibgeMetadata,
		},
		Warnings: []string{
			"População importada da Projeção da População do IBGE; o ano_calculo seleciona uma coluna projetada/estimada da revisão informada.",
			"Tábua de mortalidade importada da última publicação anual disponível quando ano_tabua_mortalidade não é informado.",
		},
		GeneratedAt: time.Now().UTC(),
	}, nil
}

func NormalizeTableRequestYears(req GenerateTableRequest) GenerateTableRequest {
	if req.CalculationYear == 0 {
		req.CalculationYear = req.IBGEYear
	}
	if req.PopulationRevisionYear == 0 {
		req.PopulationRevisionYear = 2024
	}
	return req
}

func calculateBandMetrics(data IBGEData) ([]bandMetrics, error) {
	metrics := make([]bandMetrics, 0, len(AgeBands))
	for _, band := range AgeBands {
		qx, pop, err := weightedQXAndPopulation(band.Start, band.End, data)
		if err != nil {
			return nil, fmt.Errorf("calculando faixa %s: %w", band.Name, err)
		}
		metrics = append(metrics, bandMetrics{
			Band:       band,
			Population: pop,
			QX:         qx,
			Deaths:     pop * qx,
		})
	}
	return metrics, nil
}

func weightedQX(start, end int, data IBGEData) (float64, error) {
	qx, _, err := weightedQXAndPopulation(start, end, data)
	return qx, err
}

func weightedQXAndPopulation(start, end int, data IBGEData) (float64, float64, error) {
	var numerator, denominator float64
	for age := start; age <= end; age++ {
		pop, ok := data.PopulationByAge[age]
		if !ok {
			if start >= 85 && age > maxAvailableAge(data) {
				break
			}
			return 0, 0, fmt.Errorf("população ausente para idade %d", age)
		}
		qx, ok := data.MortalityQXByAge[age]
		if !ok {
			if start >= 85 && age > maxAvailableAge(data) {
				break
			}
			return 0, 0, fmt.Errorf("qx ausente para idade %d", age)
		}
		numerator += pop * qx
		denominator += pop
	}
	if denominator <= 0 {
		return 0, 0, errors.New("população ponderadora deve ser maior que zero")
	}
	return numerator / denominator, denominator, nil
}

func maxAvailableAge(data IBGEData) int {
	max := 0
	for age := range data.MortalityQXByAge {
		if _, ok := data.PopulationByAge[age]; ok && age > max {
			max = age
		}
	}
	return max
}

func referenceActuarialCost(metrics []bandMetrics, method string) (float64, error) {
	switch method {
	case MutualitySimple:
		var sum float64
		for _, metric := range metrics {
			sum += metric.ActuarialCost
		}
		return sum / float64(len(metrics)), nil
	case MutualityPopulation:
		return weightedReference(metrics, func(metric bandMetrics) float64 { return metric.Population })
	case MutualityMortality:
		return weightedReference(metrics, func(metric bandMetrics) float64 { return metric.Deaths })
	default:
		return 0, fmt.Errorf("método de mutualidade inválido: %s", method)
	}
}

func weightedReference(metrics []bandMetrics, weight func(bandMetrics) float64) (float64, error) {
	var numerator, denominator float64
	for _, metric := range metrics {
		w := weight(metric)
		numerator += metric.ActuarialCost * w
		denominator += w
	}
	if denominator <= 0 {
		return 0, errors.New("peso de referência deve ser maior que zero")
	}
	return numerator / denominator, nil
}

func cloneMetrics(metrics []bandMetrics) []bandMetrics {
	cloned := make([]bandMetrics, len(metrics))
	copy(cloned, metrics)
	return cloned
}

func bandIndex(name string) int {
	for i, band := range AgeBands {
		if band.Name == name {
			return i
		}
	}
	return len(AgeBands)
}

func BandForAge(age float64) string {
	if age < 40 {
		return "18-39"
	}
	if age < 55 {
		return "40-54"
	}
	if age < 65 {
		return "55-64"
	}
	if age < 75 {
		return "65-74"
	}
	if age < 85 {
		return "75-84"
	}
	return "85+"
}
