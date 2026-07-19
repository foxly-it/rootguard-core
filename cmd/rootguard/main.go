package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/foxly-it/rootguard-core/internal/api"
	"github.com/foxly-it/rootguard-core/internal/unbound"
)

func main() {
	token := os.Getenv("ROOTGUARD_API_TOKEN")
	if token == "" {
		log.Fatal("ROOTGUARD_API_TOKEN must be set")
	}

	port := envOrDefault("PORT", "8081")
	manager := unbound.NewManager(
		envOrDefault("UNBOUND_CONFIG_DIR", "/var/lib/rootguard/unbound"),
		envOrDefault("UNBOUND_CONTAINER_CONFIG_DIR", "/etc/unbound/unbound.d"),
		envOrDefault("UNBOUND_CONTAINER_NAME", "rootguard-unbound"),
	)

	handler := api.RegisterRoutes(api.Dependencies{
		Token:   token,
		Unbound: manager,
	})

	server := &http.Server{
		Addr:              "0.0.0.0:" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("RootGuard Core API listening on :%s", port)
	log.Fatal(server.ListenAndServe())
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
