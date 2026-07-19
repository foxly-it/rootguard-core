package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"runtime"
	"strings"

	"github.com/foxly-it/rootguard-core/internal/adguard"
	"github.com/foxly-it/rootguard-core/internal/docker"
	"github.com/foxly-it/rootguard-core/internal/stack"
	"github.com/foxly-it/rootguard-core/internal/unbound"
)

type Dependencies struct {
	Token   string
	Unbound *unbound.Manager
	AdGuard *adguard.Manager
}

func RegisterRoutes(deps Dependencies) http.Handler {
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/system", systemHandler)
	apiMux.HandleFunc("GET /api/docker/status", dockerStatusHandler)
	apiMux.HandleFunc("GET /api/stack/status", stackStatusHandler)
	apiMux.HandleFunc("GET /api/dashboard", dashboardHandler)
	apiMux.HandleFunc("GET /api/services", servicesHandler)
	apiMux.HandleFunc("POST /api/services/{name}/{action}", serviceActionHandler)
	apiMux.HandleFunc("GET /api/unbound/settings", getUnboundSettingsHandler(deps.Unbound))
	apiMux.HandleFunc("PUT /api/unbound/settings", putUnboundSettingsHandler(deps.Unbound))
	apiMux.HandleFunc("GET /api/adguard/status", getAdGuardStatusHandler(deps.AdGuard))
	apiMux.HandleFunc("POST /api/adguard/bootstrap", bootstrapAdGuardHandler(deps.AdGuard))

	root := http.NewServeMux()
	root.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	root.Handle("/api/", requireBearerToken(deps.Token, apiMux))
	return root
}

func getAdGuardStatusHandler(manager *adguard.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := manager.Status(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, status)
	}
}

func bootstrapAdGuardHandler(manager *adguard.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := manager.Bootstrap(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, status)
	}
}

func systemHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"os": runtime.GOOS, "arch": runtime.GOARCH,
	})
}

func dockerStatusHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, docker.CheckDockerStatus())
}

func stackStatusHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, stack.CheckStackStatus())
}

type dashboardResponse struct {
	Docker dashboardDocker `json:"docker"`
	DNS    dashboardDNS    `json:"dns"`
}

type dashboardDocker struct {
	CPU        float64 `json:"cpu"`
	Memory     float64 `json:"memory"`
	Containers int     `json:"containers"`
	Status     string  `json:"status"`
}

type dashboardDNS struct {
	Status   string `json:"status"`
	Resolver string `json:"resolver"`
	DNSSEC   bool   `json:"dnssec"`
}

func dashboardHandler(w http.ResponseWriter, _ *http.Request) {
	status := stack.CheckStackStatus()
	running := 0
	if status.AdGuard.Running {
		running++
	}
	if status.Unbound.Running {
		running++
	}

	dockerHealth := "down"
	if running == 2 {
		dockerHealth = "healthy"
	} else if running > 0 {
		dockerHealth = "degraded"
	}

	dnsHealth := "down"
	if status.Unbound.Running && status.AdGuard.Running {
		dnsHealth = "healthy"
	} else if status.Unbound.Running || status.AdGuard.Running {
		dnsHealth = "degraded"
	}

	writeJSON(w, http.StatusOK, dashboardResponse{
		Docker: dashboardDocker{Containers: running, Status: dockerHealth},
		DNS:    dashboardDNS{Status: dnsHealth, Resolver: "Unbound", DNSSEC: true},
	})
}

type serviceResponse struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

func servicesHandler(w http.ResponseWriter, _ *http.Request) {
	status := stack.CheckStackStatus()
	writeJSON(w, http.StatusOK, []serviceResponse{
		{
			Name: "adguard", DisplayName: "AdGuard Home",
			Description: "DNS filtering, blocklists and client policies",
			Status:      runningStatus(status.AdGuard.Running),
		},
		{
			Name: "unbound", DisplayName: "Unbound DNS",
			Description: "Recursive resolver with DNSSEC validation",
			Status:      runningStatus(status.Unbound.Running),
		},
	})
}

func runningStatus(running bool) string {
	if running {
		return "running"
	}
	return "stopped"
}

func serviceActionHandler(w http.ResponseWriter, r *http.Request) {
	serviceName := r.PathValue("name")
	action := r.PathValue("action")
	if err := stack.ControlService(r.Context(), serviceName, action); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, stack.ErrUnknownService) || errors.Is(err, stack.ErrUnknownAction) {
			status = http.StatusBadRequest
		}
		writeError(w, status, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"service": serviceName,
		"action":  action,
		"status":  "ok",
	})
}

func getUnboundSettingsHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		settings, err := manager.Load()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, settings)
	}
}

func putUnboundSettingsHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()

		var settings unbound.Settings
		if err := decoder.Decode(&settings); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := manager.Apply(r.Context(), settings); err != nil {
			if errors.Is(err, unbound.ErrInvalidSettings) {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, settings)
	}
}

func requireBearerToken(expected string, next http.Handler) http.Handler {
	expectedHash := sha256.Sum256([]byte(expected))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		providedHash := sha256.Sum256([]byte(provided))
		if provided == "" || subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) != 1 {
			writeError(w, http.StatusUnauthorized, errors.New("unauthorized"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
