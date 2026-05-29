// Package httpapi exposes the service over a stdlib net/http JSON API with CORS.
package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/souzavinny/reviews-api/internal/service"
)

const (
	maxRequestBody = 4096
	maxWindowHours = 90 * 24
)

// Config configures the HTTP API.
type Config struct {
	DefaultWindow time.Duration
	AllowedOrigin string
	// OnAppAdded, if set, is called with the app id after a successful add so
	// the composition root can trigger a non-blocking initial poll.
	OnAppAdded func(appID string)
}

type handler struct {
	svc           *service.Service
	defaultWindow time.Duration
	allowedOrigin string
	onAppAdded    func(appID string)
}

// New builds the routed, CORS-wrapped HTTP handler.
func New(svc *service.Service, cfg Config) http.Handler {
	h := &handler{
		svc:           svc,
		defaultWindow: cfg.DefaultWindow,
		allowedOrigin: cfg.AllowedOrigin,
		onAppAdded:    cfg.OnAppAdded,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /apps", h.listApps)
	mux.HandleFunc("POST /apps", h.addApp)
	mux.HandleFunc("DELETE /apps/{appID}", h.removeApp)
	mux.HandleFunc("GET /apps/{appID}/reviews", h.getReviews)
	mux.HandleFunc("GET /apps/{appID}/summary", h.getSummary)
	mountDocs(mux)
	return h.withCORS(mux)
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handler) getReviews(w http.ResponseWriter, r *http.Request) {
	appID, ok := parseAppID(w, r)
	if !ok {
		return
	}
	reviews, err := h.svc.GetRecentReviews(r.Context(), appID, h.window(r))
	if err != nil {
		log.Printf("httpapi: get reviews: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load reviews")
		return
	}
	writeJSON(w, http.StatusOK, reviews)
}

func (h *handler) getSummary(w http.ResponseWriter, r *http.Request) {
	appID, ok := parseAppID(w, r)
	if !ok {
		return
	}
	summary, err := h.svc.GetSummary(r.Context(), appID, h.window(r))
	if err != nil {
		log.Printf("httpapi: get summary: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load summary")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *handler) listApps(w http.ResponseWriter, r *http.Request) {
	apps, err := h.svc.ListApps(r.Context())
	if err != nil {
		log.Printf("httpapi: list apps: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list apps")
		return
	}
	writeJSON(w, http.StatusOK, apps)
}

func (h *handler) addApp(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if _, err := strconv.ParseUint(req.ID, 10, 64); err != nil {
		writeError(w, http.StatusBadRequest, "id must be numeric")
		return
	}

	app, err := h.svc.AddApp(r.Context(), req.ID)
	if err != nil {
		if errors.Is(err, service.ErrAppNotFound) {
			writeError(w, http.StatusNotFound, "app not found in the App Store")
			return
		}
		log.Printf("httpapi: add app %s: %v", req.ID, err)
		writeError(w, http.StatusInternalServerError, "failed to add app")
		return
	}
	if h.onAppAdded != nil {
		h.onAppAdded(app.ID)
	}
	writeJSON(w, http.StatusCreated, app)
}

func (h *handler) removeApp(w http.ResponseWriter, r *http.Request) {
	appID, ok := parseAppID(w, r)
	if !ok {
		return
	}
	if err := h.svc.RemoveApp(r.Context(), appID); err != nil {
		log.Printf("httpapi: remove app: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to remove app")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// window reads the ?hours= query param, defaulting when absent or invalid and
// clamping to maxWindowHours so a crafted value can't widen the window without
// bound.
func (h *handler) window(r *http.Request) time.Duration {
	hours := r.URL.Query().Get("hours")
	if hours == "" {
		return h.defaultWindow
	}
	n, err := strconv.Atoi(hours)
	if err != nil || n <= 0 {
		return h.defaultWindow
	}
	if n > maxWindowHours {
		n = maxWindowHours
	}
	return time.Duration(n) * time.Hour
}

// parseAppID returns the numeric {appID} path value, writing a 400 and
// returning false when it isn't numeric.
func parseAppID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("appID")
	if _, err := strconv.ParseUint(id, 10, 64); err != nil {
		writeError(w, http.StatusBadRequest, "app id must be numeric")
		return "", false
	}
	return id, true
}

func (h *handler) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", h.allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type errorBody struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("httpapi: encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorBody{Error: msg})
}
