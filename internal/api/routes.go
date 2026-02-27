package api

import (
	"encoding/json"
	"net/http"
	"runtime"

	"github.com/foxly-it/rootguard/internal/docker"
	"github.com/foxly-it/rootguard/internal/stack"
)

func RegisterRoutes() http.Handler {

	mux := http.NewServeMux()

	// =========================
	// BASIC
	// =========================
	mux.HandleFunc("/api/health", HealthHandler)
	mux.HandleFunc("/api/system", SystemHandler)

	// =========================
	// DOCKER
	// =========================
	mux.HandleFunc("/api/docker/status", DockerStatusHandler)
	mux.HandleFunc("/api/docker/install", DockerInstallHandler)
	mux.HandleFunc("/api/docker/install/status", DockerInstallStatusHandler)

	// =========================
	// STACK
	// =========================
	mux.HandleFunc("/api/stack/status", StackStatusHandler)
	mux.HandleFunc("/api/stack/deploy", StackDeployHandler)

	return mux
}

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

//
// =========================
// BASIC HANDLERS
// =========================
//

func HealthHandler(w http.ResponseWriter, r *http.Request) {

	enableCORS(w)

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func SystemHandler(w http.ResponseWriter, r *http.Request) {

	enableCORS(w)

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"os":   runtime.GOOS,
		"arch": runtime.GOARCH,
	})
}

//
// =========================
// DOCKER HANDLERS
// =========================
//

func DockerStatusHandler(w http.ResponseWriter, r *http.Request) {

	enableCORS(w)

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := docker.CheckDockerStatus()
	json.NewEncoder(w).Encode(status)
}

func DockerInstallHandler(w http.ResponseWriter, r *http.Request) {

	enableCORS(w)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	docker.InstallDockerAsync()
	w.WriteHeader(http.StatusAccepted)
}

func DockerInstallStatusHandler(w http.ResponseWriter, r *http.Request) {

	enableCORS(w)

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := docker.GetState()
	json.NewEncoder(w).Encode(state)
}

//
// =========================
// STACK HANDLERS
// =========================
//

func StackStatusHandler(w http.ResponseWriter, r *http.Request) {

	enableCORS(w)

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := stack.CheckStackStatus()
	json.NewEncoder(w).Encode(status)
}

func StackDeployHandler(w http.ResponseWriter, r *http.Request) {

	enableCORS(w)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := stack.DeployStack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
