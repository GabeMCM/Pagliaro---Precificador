package ibge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"pagliaro-precificador/internal/pricing"
)

const (
	defaultPopulationURLTemplate = "https://ftp.ibge.gov.br/Projecao_da_Populacao/Projecao_da_Populacao_%d/projecoes_%d_tab1_idade_simples.xlsx"
	defaultMortalityIndexURL     = "https://ftp.ibge.gov.br/Tabuas_Completas_de_Mortalidade/"
	defaultMortalityURLTemplate  = "https://ftp.ibge.gov.br/Tabuas_Completas_de_Mortalidade/Tabuas_Completas_de_Mortalidade_%d/xlsx/ambos_os_sexos.xlsx"
)

type Provider struct {
	client *http.Client
}

func NewProvider() *Provider {
	return &Provider{
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *Provider) Fetch(ctx context.Context, calculationYear int, populationRevisionYear int, mortalityTableYear int, populationURL string, mortalityURL string) (pricing.IBGEData, *pricing.IBGEMetadata, error) {
	if calculationYear <= 0 {
		return pricing.IBGEData{}, nil, fmt.Errorf("ano_calculo deve ser informado")
	}
	if populationRevisionYear <= 0 {
		populationRevisionYear = 2024
	}
	if populationURL == "" {
		populationURL = fmt.Sprintf(defaultPopulationURLTemplate, populationRevisionYear, populationRevisionYear)
	}
	if mortalityURL == "" {
		if mortalityTableYear <= 0 {
			resolved, err := p.latestMortalityTableYear(ctx, calculationYear)
			if err != nil {
				return pricing.IBGEData{}, nil, err
			}
			mortalityTableYear = resolved
		}
		mortalityURL = fmt.Sprintf(defaultMortalityURLTemplate, mortalityTableYear)
	}

	populationBytes, err := p.download(ctx, populationURL)
	if err != nil {
		return pricing.IBGEData{}, nil, fmt.Errorf("baixando população IBGE: %w", err)
	}
	mortalityBytes, err := p.download(ctx, mortalityURL)
	if err != nil {
		return pricing.IBGEData{}, nil, fmt.Errorf("baixando mortalidade IBGE: %w", err)
	}

	population, err := ParsePopulationXLSX(populationBytes, calculationYear)
	if err != nil {
		return pricing.IBGEData{}, nil, fmt.Errorf("lendo população IBGE: %w", err)
	}
	mortality, err := ParseMortalityXLSX(mortalityBytes)
	if err != nil {
		return pricing.IBGEData{}, nil, fmt.Errorf("lendo mortalidade IBGE: %w", err)
	}

	return pricing.IBGEData{
			MortalityQXByAge: mortality,
			PopulationByAge:  population,
		}, &pricing.IBGEMetadata{
			PopulationURL:          populationURL,
			MortalityURL:           mortalityURL,
			CalculationYear:        calculationYear,
			PopulationRevisionYear: populationRevisionYear,
			MortalityTableYear:     mortalityTableYear,
			PopulationSHA256:       checksum(populationBytes),
			MortalitySHA256:        checksum(mortalityBytes),
		}, nil
}

func (p *Provider) latestMortalityTableYear(ctx context.Context, maxYear int) (int, error) {
	body, err := p.download(ctx, defaultMortalityIndexURL)
	if err != nil {
		return 0, fmt.Errorf("consultando índice de mortalidade IBGE: %w", err)
	}
	matches := regexp.MustCompile(`Tabuas_Completas_de_Mortalidade_(\d{4})/`).FindAllStringSubmatch(string(body), -1)
	latest := 0
	for _, match := range matches {
		year, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		if year <= maxYear && year > latest {
			latest = year
		}
	}
	if latest == 0 {
		return 0, fmt.Errorf("nenhuma tábua de mortalidade disponível até %d encontrada no IBGE", maxYear)
	}
	return latest, nil
}

func (p *Provider) download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d em %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func ParsePopulationXLSX(data []byte, year int) (map[int]float64, error) {
	sheet, err := readFirstSheetXLSX(data)
	if err != nil {
		return nil, err
	}

	yearColumn := -1
	headerRow := -1
	for rowIndex, row := range sheet.Rows {
		for colIndex, value := range row {
			if strings.TrimSpace(value) == strconv.Itoa(year) {
				yearColumn = colIndex
			}
		}
		if len(row) >= 5 && strings.EqualFold(row[0], "IDADE") && strings.EqualFold(row[1], "SEXO") {
			headerRow = rowIndex
		}
		if yearColumn >= 0 && headerRow >= 0 {
			break
		}
	}
	if yearColumn < 0 {
		return nil, fmt.Errorf("coluna do ano %d não encontrada", year)
	}
	if headerRow < 0 {
		return nil, fmt.Errorf("cabeçalho de população não encontrado")
	}

	population := map[int]float64{}
	for _, row := range sheet.Rows[headerRow+1:] {
		if len(row) <= yearColumn || len(row) < 5 {
			continue
		}
		if !strings.EqualFold(row[1], "Ambos") || row[3] != "BR" || !strings.EqualFold(row[4], "Brasil") {
			continue
		}
		age, err := parseInt(row[0])
		if err != nil {
			continue
		}
		value, err := parseFloat(row[yearColumn])
		if err != nil {
			return nil, fmt.Errorf("população inválida para idade %d: %w", age, err)
		}
		population[age] = value
	}
	if len(population) == 0 {
		return nil, fmt.Errorf("nenhuma população importada")
	}
	return population, nil
}

func ParseMortalityXLSX(data []byte) (map[int]float64, error) {
	sheet, err := readFirstSheetXLSX(data)
	if err != nil {
		return nil, err
	}

	qxByAge := map[int]float64{}
	for _, row := range sheet.Rows {
		if len(row) < 2 {
			continue
		}
		age, err := parseInt(row[0])
		if err != nil {
			continue
		}
		qxPerThousand, err := parseFloat(row[1])
		if err != nil {
			return nil, fmt.Errorf("qx inválido para idade %d: %w", age, err)
		}
		qxByAge[age] = qxPerThousand / 1000
	}
	if len(qxByAge) == 0 {
		return nil, fmt.Errorf("nenhum qx importado")
	}
	return qxByAge, nil
}
