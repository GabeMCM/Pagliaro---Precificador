package pricing

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"pagliaro-precificador/internal/domain"
)

type QuoteRequest struct {
	ERPQuoteID        string          `json:"erp_orcamento_id"`
	PlanName          string          `json:"nome_plano"`
	Coverage          float64         `json:"cobertura"`
	Formula           string          `json:"formula"`
	ReturnComparative bool            `json:"retornar_comparativo"`
	CalculationDate   string          `json:"data_calculo,omitempty"`
	Members           []MemberRequest `json:"integrantes"`
}

type MemberRequest struct {
	ERPPersonID  string `json:"erp_pessoa_id"`
	Name         string `json:"nome,omitempty"`
	BirthDate    string `json:"data_nascimento,omitempty"`
	Age          *int   `json:"idade,omitempty"`
	Relationship string `json:"parentesco"`
	IsPrimary    bool   `json:"titular,omitempty"`
}

type QuoteResult struct {
	ERPQuoteID         string         `json:"erp_orcamento_id"`
	TableVersion       string         `json:"versao_tabela"`
	PlanName           string         `json:"nome_plano"`
	Coverage           float64        `json:"cobertura"`
	BaseAgeBand        string         `json:"faixa_base"`
	WeightedAverageAge float64        `json:"idade_media_ponderada"`
	SummedQX           float64        `json:"soma_qx"`
	BaseMonthlyPrice   float64        `json:"valor_base_mensal"`
	Result             QuotePrices    `json:"resultado"`
	Members            []PricedMember `json:"integrantes"`
	GeneratedAt        time.Time      `json:"gerado_em"`
}

type QuotePrices struct {
	RecommendedFormula  string   `json:"formula_recomendada"`
	FormulaA            *float64 `json:"formula_a,omitempty"`
	FormulaC            *float64 `json:"formula_c,omitempty"`
	FinalMonthlyPrice   float64  `json:"valor_mensal_final"`
	ExpectedAnnualClaim float64  `json:"sinistro_anual_esperado"`
}

type PricedMember struct {
	ERPPersonID  string  `json:"erp_pessoa_id"`
	Age          int     `json:"idade"`
	Relationship string  `json:"parentesco"`
	IsPrimary    bool    `json:"titular"`
	QX           float64 `json:"qx"`
}

func GenerateQuote(req QuoteRequest, table domain.PricingTable, mortalityQXByAge map[int]float64) (QuoteResult, error) {
	if req.ERPQuoteID == "" {
		return QuoteResult{}, errors.New("erp_orcamento_id deve ser informado")
	}
	if req.PlanName == "" {
		return QuoteResult{}, errors.New("nome_plano deve ser informado")
	}
	if req.Coverage <= 0 {
		return QuoteResult{}, errors.New("cobertura deve ser maior que zero")
	}
	if req.Formula == "" {
		req.Formula = "C"
	}
	if req.Formula != "A" && req.Formula != "C" && req.Formula != "comparativo" {
		return QuoteResult{}, errors.New("formula deve ser A, C ou comparativo")
	}
	if len(req.Members) == 0 {
		return QuoteResult{}, errors.New("ao menos um integrante deve ser informado")
	}

	calculationDate := time.Now().UTC()
	if req.CalculationDate != "" {
		parsed, err := time.Parse("2006-01-02", req.CalculationDate)
		if err != nil {
			return QuoteResult{}, errors.New("data_calculo deve estar no formato YYYY-MM-DD")
		}
		calculationDate = parsed
	}

	members, err := priceMembers(req.Members, calculationDate, mortalityQXByAge)
	if err != nil {
		return QuoteResult{}, err
	}
	sumQX := 0.0
	weightedAgeNumerator := 0.0
	for _, member := range members {
		sumQX += member.QX
		weightedAgeNumerator += float64(member.Age) * member.QX
	}
	if sumQX <= 0 {
		return QuoteResult{}, errors.New("soma de qx deve ser maior que zero")
	}
	weightedAverageAge := weightedAgeNumerator / sumQX
	baseAgeBand := BandForAge(weightedAverageAge)

	baseRow, err := findTableRow(table.Result.Rows, req.PlanName, req.Coverage, baseAgeBand)
	if err != nil {
		return QuoteResult{}, err
	}

	formulaADecimal := decimal.NewFromFloat(baseRow.BaseMonthlyPrice).
		Mul(decimal.NewFromFloat(1 + sumQX*float64(len(members)-1)))
	formulaAValue, _ := formulaADecimal.Float64()
	formulaA := CeilMoney(formulaAValue)
	additionalQX := 0.0
	for _, member := range members {
		if !member.IsPrimary {
			additionalQX += member.QX
		}
	}
	formulaCDecimal := decimal.NewFromFloat(baseRow.BaseMonthlyPrice).
		Mul(decimal.NewFromFloat(1 + additionalQX + 0.01*float64(len(members)-1)))
	formulaCValue, _ := formulaCDecimal.Float64()
	formulaC := CeilMoney(formulaCValue)
	expectedClaimValue, _ := decimal.NewFromFloat(req.Coverage).Mul(decimal.NewFromFloat(sumQX)).Float64()
	expectedClaim := CeilMoney(expectedClaimValue)

	prices := QuotePrices{
		RecommendedFormula:  "C",
		ExpectedAnnualClaim: expectedClaim,
	}
	switch req.Formula {
	case "A":
		prices.FormulaA = &formulaA
		prices.FinalMonthlyPrice = formulaA
	case "C":
		prices.FormulaC = &formulaC
		prices.FinalMonthlyPrice = formulaC
	case "comparativo":
		prices.FormulaA = &formulaA
		prices.FormulaC = &formulaC
		prices.FinalMonthlyPrice = formulaC
	}
	if req.ReturnComparative && req.Formula != "comparativo" {
		prices.FormulaA = &formulaA
		prices.FormulaC = &formulaC
	}

	return QuoteResult{
		ERPQuoteID:         req.ERPQuoteID,
		TableVersion:       table.Version,
		PlanName:           req.PlanName,
		Coverage:           CeilMoney(req.Coverage),
		BaseAgeBand:        baseAgeBand,
		WeightedAverageAge: CeilMoney(weightedAverageAge),
		SummedQX:           sumQX,
		BaseMonthlyPrice:   baseRow.BaseMonthlyPrice,
		Result:             prices,
		Members:            members,
		GeneratedAt:        time.Now().UTC(),
	}, nil
}

func priceMembers(raw []MemberRequest, calculationDate time.Time, mortalityQXByAge map[int]float64) ([]PricedMember, error) {
	members := make([]PricedMember, 0, len(raw))
	primaryIndex := -1
	for i, member := range raw {
		age, err := resolveAge(member, calculationDate)
		if err != nil {
			return nil, fmt.Errorf("integrantes[%d]: %w", i, err)
		}
		qx, ok := mortalityQXByAge[age]
		if !ok {
			return nil, fmt.Errorf("qx não encontrado para idade %d", age)
		}
		isPrimary := member.IsPrimary || member.Relationship == "titular"
		if isPrimary {
			if primaryIndex >= 0 {
				return nil, errors.New("apenas um integrante pode ser titular")
			}
			primaryIndex = i
		}
		members = append(members, PricedMember{
			ERPPersonID:  member.ERPPersonID,
			Age:          age,
			Relationship: member.Relationship,
			IsPrimary:    isPrimary,
			QX:           qx,
		})
	}
	if primaryIndex < 0 {
		members[0].IsPrimary = true
	}
	return members, nil
}

func resolveAge(member MemberRequest, calculationDate time.Time) (int, error) {
	if member.BirthDate != "" {
		birthDate, err := time.Parse("2006-01-02", member.BirthDate)
		if err != nil {
			return 0, errors.New("data_nascimento deve estar no formato YYYY-MM-DD")
		}
		age := calculationDate.Year() - birthDate.Year()
		birthdayThisYear := time.Date(calculationDate.Year(), birthDate.Month(), birthDate.Day(), 0, 0, 0, 0, time.UTC)
		if calculationDate.Before(birthdayThisYear) {
			age--
		}
		if age < 0 {
			return 0, errors.New("data_nascimento não pode ser futura")
		}
		return age, nil
	}
	if member.Age == nil {
		return 0, errors.New("data_nascimento ou idade deve ser informada")
	}
	if *member.Age < 0 {
		return 0, errors.New("idade não pode ser negativa")
	}
	return *member.Age, nil
}

func findTableRow(rows []domain.PricingTableRow, planName string, coverage float64, ageBand string) (domain.PricingTableRow, error) {
	for _, row := range rows {
		if row.PlanName == planName && row.Coverage == CeilMoney(coverage) && row.AgeBand == ageBand {
			return row, nil
		}
	}
	return domain.PricingTableRow{}, fmt.Errorf("valor-base não encontrado para plano=%s cobertura=%.2f faixa=%s", planName, coverage, ageBand)
}
