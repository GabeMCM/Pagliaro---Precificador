package pricing

import (
	"encoding/json"
	"testing"

	"pagliaro-precificador/internal/domain"
)

func TestTableArtifactValidationDetectsTampering(t *testing.T) {
	data := IBGEData{
		MortalityQXByAge: map[int]float64{},
		PopulationByAge:  map[int]float64{},
	}
	for age := 0; age <= 100; age++ {
		data.PopulationByAge[age] = 1000
		data.MortalityQXByAge[age] = 0.001
	}

	req := GenerateTableRequest{
		CalculationYear: 2026,
		Version:         "test-json",
		Plans:           []PlanRequest{{Name: "Plano Livre", Coverage: 7000}},
		Costs: CostsRequest{
			FixedMonthlyExpense: 50000,
			TotalClients:        3000,
			DelinquentClients:   500,
			BankFee:             3,
		},
		Margins:   MarginsRequest{ProfitRate: 0.20, CommissionRate: 0.10},
		Mutuality: MutualityRequest{Method: MutualityPopulation, Factor: 0.30},
	}
	result, err := GenerateTable(req, data, nil)
	if err != nil {
		t.Fatalf("GenerateTable: %v", err)
	}
	artifact, err := BuildTableArtifact(req, result, data, "secret")
	if err != nil {
		t.Fatalf("BuildTableArtifact: %v", err)
	}
	if artifact.Hash == "" {
		t.Fatalf("esperava hash")
	}
	if artifact.Signature == "" {
		t.Fatalf("esperava assinatura")
	}
	if err := ValidateTableArtifact(artifact, "secret"); err != nil {
		t.Fatalf("ValidateTableArtifact: %v", err)
	}

	artifact.Result.Rows[0].BaseMonthlyPrice = artifact.Result.Rows[0].BaseMonthlyPrice + 1
	if err := ValidateTableArtifact(artifact, "secret"); err == nil {
		t.Fatalf("esperava erro ao validar tabela alterada")
	}
}

func TestGenerateQuoteFromArtifact(t *testing.T) {
	data := IBGEData{
		MortalityQXByAge: map[int]float64{},
		PopulationByAge:  map[int]float64{},
	}
	for age := 0; age <= 100; age++ {
		data.PopulationByAge[age] = 1000
		data.MortalityQXByAge[age] = 0.001
	}

	req := GenerateTableRequest{
		CalculationYear: 2026,
		Version:         "test-json",
		Plans:           []PlanRequest{{Name: "Plano Livre", Coverage: 7000}},
		Costs: CostsRequest{
			FixedMonthlyExpense: 50000,
			TotalClients:        3000,
			DelinquentClients:   500,
			BankFee:             3,
		},
		Margins:   MarginsRequest{ProfitRate: 0.20, CommissionRate: 0.10},
		Mutuality: MutualityRequest{Method: MutualityPopulation, Factor: 0.30},
	}
	result, err := GenerateTable(req, data, nil)
	if err != nil {
		t.Fatalf("GenerateTable: %v", err)
	}
	artifact, err := BuildTableArtifact(req, result, data, "")
	if err != nil {
		t.Fatalf("BuildTableArtifact: %v", err)
	}

	resultQuote, err := GenerateQuoteFromArtifact(StatelessQuoteRequest{
		TableArtifact: artifact,
		QuoteRequest: QuoteRequest{
			ERPQuoteID: "ERP-1",
			PlanName:   "Plano Livre",
			Coverage:   7000,
			Formula:    "C",
			Members: []MemberRequest{
				{ERPPersonID: "1", Age: ptr(35), Relationship: "titular"},
			},
		},
	}, "")
	if err != nil {
		t.Fatalf("GenerateQuoteFromArtifact: %v", err)
	}
	if resultQuote.TableVersion != "test-json" {
		t.Fatalf("versão de tabela incorreta: %s", resultQuote.TableVersion)
	}
}

func TestTableArtifactJSONRoundTrip(t *testing.T) {
	artifact := TableArtifact{
		Type:          TableArtifactType,
		SchemaVersion: TableArtifactSchemaVersion,
		Version:       "test-json",
		Result: domain.TableResult{
			Rows: []domain.PricingTableRow{
				{PlanName: "Plano Livre", Coverage: 7000, AgeBand: "18-39", BaseMonthlyPrice: 100},
			},
		},
		MortalityQXByAge: map[int]float64{35: 0.001},
	}
	hash, err := artifactHash(artifact)
	if err != nil {
		t.Fatalf("artifactHash: %v", err)
	}
	artifact.Hash = hash

	raw, err := json.Marshal(StatelessQuoteRequest{
		TableArtifact: artifact,
		QuoteRequest: QuoteRequest{
			ERPQuoteID: "ERP-1",
			PlanName:   "Plano Livre",
			Coverage:   7000,
			Formula:    "C",
			Members: []MemberRequest{
				{ERPPersonID: "1", Age: ptr(35), Relationship: "titular"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded StatelessQuoteRequest
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := decoded.TableArtifact.MortalityQXByAge[35]; !ok {
		t.Fatalf("esperava qx da idade 35 após roundtrip JSON")
	}
	if _, err := GenerateQuoteFromArtifact(decoded, ""); err != nil {
		t.Fatalf("GenerateQuoteFromArtifact decoded: %v", err)
	}
}
