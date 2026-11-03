package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pagliaro-precificador/internal/app"
	"pagliaro-precificador/internal/config"
)

// @title Pagliaro Precificador API
// @version 0.3.0
// @description API para geração de tabela anual de preços e cálculo de orçamento de planos funerários.
// @description O fluxo stateless usa /v1/admin/tabelas/gerar-json e /v1/erp/orcamentos/calcular sem banco obrigatório.
// @BasePath /
// @schemes http https
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
func main() {
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Printf("carregando .env: %v", err)
	}

	cfg := config.Load()
	ctx := context.Background()

	application, err := app.New(ctx, cfg)
	if err != nil {
		log.Fatalf("iniciando aplicação: %v", err)
	}
	defer application.Close()

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           application.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("API disponível em http://%s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("servidor HTTP: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("encerrando servidor: %v", err)
	}
}
