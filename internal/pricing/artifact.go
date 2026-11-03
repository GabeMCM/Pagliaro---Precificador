package pricing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"pagliaro-precificador/internal/domain"
)

const (
	TableArtifactType          = "pagliaro_tabela_precos"
	TableArtifactSchemaVersion = "1.0"
)

type TableArtifact struct {
	Type             string               `json:"tipo"`
	SchemaVersion    string               `json:"schema_version"`
	Version          string               `json:"versao"`
	GeneratedAt      time.Time            `json:"gerada_em"`
	IBGE             *domain.IBGEMetadata `json:"ibge,omitempty"`
	Parameters       GenerateTableRequest `json:"parametros"`
	Result           domain.TableResult   `json:"resultado"`
	MortalityQXByAge map[int]float64      `json:"qx_mortalidade_por_idade"`
	Hash             string               `json:"hash"`
	Signature        string               `json:"assinatura,omitempty"`
}

type StatelessQuoteRequest struct {
	TableArtifact TableArtifact `json:"tabela"`
	QuoteRequest
}

func BuildTableArtifact(req GenerateTableRequest, result domain.TableResult, data IBGEData, signatureSecret string) (TableArtifact, error) {
	artifact := TableArtifact{
		Type:             TableArtifactType,
		SchemaVersion:    TableArtifactSchemaVersion,
		Version:          result.Version,
		GeneratedAt:      result.GeneratedAt,
		IBGE:             result.Summary.IBGE,
		Parameters:       NormalizeTableRequestYears(req),
		Result:           result,
		MortalityQXByAge: data.MortalityQXByAge,
	}
	hash, err := artifactHash(artifact)
	if err != nil {
		return TableArtifact{}, err
	}
	artifact.Hash = hash
	if signatureSecret != "" {
		artifact.Signature = signHash(hash, signatureSecret)
	}
	return artifact, nil
}

func GenerateQuoteFromArtifact(req StatelessQuoteRequest, signatureSecret string) (QuoteResult, error) {
	if err := ValidateTableArtifact(req.TableArtifact, signatureSecret); err != nil {
		return QuoteResult{}, err
	}
	table := domain.PricingTable{
		Version: req.TableArtifact.Version,
		Result:  req.TableArtifact.Result,
	}
	return GenerateQuote(req.QuoteRequest, table, req.TableArtifact.MortalityQXByAge)
}

func ValidateTableArtifact(artifact TableArtifact, signatureSecret string) error {
	if artifact.Type != TableArtifactType {
		return fmt.Errorf("tipo de tabela inválido: %s", artifact.Type)
	}
	if artifact.SchemaVersion != TableArtifactSchemaVersion {
		return fmt.Errorf("schema_version de tabela inválido: %s", artifact.SchemaVersion)
	}
	if artifact.Version == "" {
		return errors.New("versao da tabela deve ser informada")
	}
	if len(artifact.Result.Rows) == 0 {
		return errors.New("tabela não contém linhas de preço")
	}
	if len(artifact.MortalityQXByAge) == 0 {
		return errors.New("tabela não contém qx_mortalidade_por_idade")
	}
	expectedHash, err := artifactHash(artifact)
	if err != nil {
		return err
	}
	if artifact.Hash == "" {
		return errors.New("hash da tabela deve ser informado")
	}
	if !hmac.Equal([]byte(artifact.Hash), []byte(expectedHash)) {
		return errors.New("hash da tabela inválido; conteúdo pode ter sido alterado")
	}
	if signatureSecret != "" {
		if artifact.Signature == "" {
			return errors.New("assinatura da tabela deve ser informada")
		}
		expectedSignature := signHash(expectedHash, signatureSecret)
		if !hmac.Equal([]byte(artifact.Signature), []byte(expectedSignature)) {
			return errors.New("assinatura da tabela inválida")
		}
	}
	return nil
}

func artifactHash(artifact TableArtifact) (string, error) {
	artifact.Hash = ""
	artifact.Signature = ""
	raw, err := json.Marshal(artifact)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func signHash(hash string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(hash))
	return hex.EncodeToString(mac.Sum(nil))
}
