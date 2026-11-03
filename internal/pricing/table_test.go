package pricing

import "testing"

func TestGenerateTableMutualityMethods(t *testing.T) {
	data := IBGEData{
		MortalityQXByAge: map[int]float64{},
		PopulationByAge:  map[int]float64{},
	}
	for age := 0; age <= 100; age++ {
		data.PopulationByAge[age] = 1000
		data.MortalityQXByAge[age] = 0.001
	}
	data.MortalityQXByAge[70] = 0.020
	data.MortalityQXByAge[80] = 0.050
	data.MortalityQXByAge[90] = 0.120

	base := GenerateTableRequest{
		IBGEYear: 2024,
		Version:  "test",
		Plans:    []PlanRequest{{Name: "Plano Livre", Coverage: 7000}},
		Costs: CostsRequest{
			FixedMonthlyExpense: 50000,
			TotalClients:        3000,
			DelinquentClients:   500,
			BankFee:             3,
		},
		Margins:   MarginsRequest{ProfitRate: 0.20, CommissionRate: 0.10},
		Mutuality: MutualityRequest{Method: MutualityPopulation, Factor: 0.30},
	}

	result, err := GenerateTable(base, data, nil)
	if err != nil {
		t.Fatalf("GenerateTable populacional: %v", err)
	}
	if len(result.Rows) != 6 {
		t.Fatalf("esperava 6 linhas, recebeu %d", len(result.Rows))
	}
	if result.Summary.MutualityMethod != MutualityPopulation {
		t.Fatalf("método incorreto: %s", result.Summary.MutualityMethod)
	}
	if result.Rows[0].BaseMonthlyPrice <= 0 {
		t.Fatalf("valor base deveria ser positivo")
	}

	base.Mutuality.Method = MutualitySimple
	if _, err := GenerateTable(base, data, nil); err != nil {
		t.Fatalf("GenerateTable simples: %v", err)
	}

	base.Mutuality.Method = MutualityMortality
	if _, err := GenerateTable(base, data, nil); err != nil {
		t.Fatalf("GenerateTable mortalidade: %v", err)
	}
}

func TestNormalizeTableRequestYearsKeepsLegacyIBGEYear(t *testing.T) {
	req := NormalizeTableRequestYears(GenerateTableRequest{IBGEYear: 2026})

	if req.CalculationYear != 2026 {
		t.Fatalf("esperava ano_calculo 2026, recebeu %d", req.CalculationYear)
	}
	if req.PopulationRevisionYear != 2024 {
		t.Fatalf("esperava revisão populacional padrão 2024, recebeu %d", req.PopulationRevisionYear)
	}
}
