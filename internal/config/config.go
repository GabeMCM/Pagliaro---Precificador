package config

import (
	"bufio"
	"os"
	"strings"
)

type Config struct {
	HTTPAddr             string
	DatabaseURL          string
	AdminAPIKey          string
	ERPAPIKey            string
	TableSignatureSecret string
	UseMemoryDB          bool
}

func Load() Config {
	httpAddr := os.Getenv("HTTP_ADDR")
	if httpAddr == "" {
		if port := os.Getenv("PORT"); port != "" {
			httpAddr = "0.0.0.0:" + port
		} else {
			httpAddr = "127.0.0.1:8080"
		}
	}
	cfg := Config{
		HTTPAddr:             httpAddr,
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		AdminAPIKey:          env("ADMIN_API_KEY", "dev-admin-key"),
		ERPAPIKey:            env("ERP_API_KEY", "dev-erp-key"),
		TableSignatureSecret: os.Getenv("TABLE_SIGNATURE_SECRET"),
	}
	cfg.UseMemoryDB = cfg.DatabaseURL == ""
	return cfg
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func LoadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
	return scanner.Err()
}
