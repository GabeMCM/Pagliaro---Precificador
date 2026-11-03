package pricing

import (
	"testing"
	"time"

	"pagliaro-precificador/internal/domain"
)

func TestGenerateQuoteComparative(t *testing.T) {
	table := domain.PricingTable{
		Version: "IBGE-2024-v1",
		Result: domain.TableResult{
			Rows: []domain.PricingTableRow{
				{
					PlanName:         "Plano Livre",
					Coverage:         7000,
					AgeBand:          "55-64",
					BaseMonthlyPrice: 100,
				},
			},
		},
	}
	mortality := map[int]float64{
		22: 0.0008,
		35: 0.0012,
		60: 0.0120,
		70: 0.019705,
	}

	age22 := 22
	result, err := GenerateQuote(QuoteRequest{
		ERPQuoteID:      "ERP-1",
		PlanName:        "Plano Livre",
		Coverage:        7000,
		Formula:         "comparativo",
		CalculationDate: time.Now().Format("2006-01-02"),
		Members: []MemberRequest{
			{ERPPersonID: "1", Age: &age22, Relationship: "titular"},
			{ERPPersonID: "2", Age: ptr(35), Relationship: "conjuge"},
			{ERPPersonID: "3", Age: ptr(60), Relationship: "pai"},
			{ERPPersonID: "4", Age: ptr(70), Relationship: "mae"},
		},
	}, table, mortality)
	if err != nil {
		t.Fatalf("GenerateQuote: %v", err)
	}

	if result.BaseAgeBand != "55-64" {
		t.Fatalf("faixa esperada 55-64, recebeu %s", result.BaseAgeBand)
	}
	if result.Result.FormulaA == nil || result.Result.FormulaC == nil {
		t.Fatalf("comparativo deveria retornar fórmula A e C")
	}
	if result.Result.FinalMonthlyPrice != *result.Result.FormulaC {
		t.Fatalf("fórmula recomendada C deve definir valor final")
	}
}

func ptr(value int) *int {
	return &value
}
