package main

import (
	"log"
	"net/http"

	"github.com/foxly-it/rootguard/internal/api"
)

func main() {

	handler := api.RegisterRoutes()

	log.Println("🦊 RootGuard API running on :8080")

	err := http.ListenAndServe("0.0.0.0:8080", handler)
	if err != nil {
		log.Fatal(err)
	}
}
