package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Config holds the environment-derived settings, resolved once at startup
// so a missing required var fails fast instead of on the first request.
type Config struct {
	GrafanaURL   string
	MimirDSUID   string
	GrafanaOrgID string
	LookbackMs   int64
}

func loadConfig() (Config, error) {
	grafanaURL, ok := os.LookupEnv("GRAFANA_URL")
	if !ok {
		return Config{}, fmt.Errorf("required env var GRAFANA_URL is not set")
	}
	mimirDSUID, ok := os.LookupEnv("MIMIR_DS_UID")
	if !ok {
		return Config{}, fmt.Errorf("required env var MIMIR_DS_UID is not set")
	}

	orgID := os.Getenv("GRAFANA_ORG_ID")
	if orgID == "" {
		orgID = "1"
	}

	lookbackMin := int64(60)
	if v := os.Getenv("LOOKBACK_MINUTES"); v != "" {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("invalid LOOKBACK_MINUTES %q: %w", v, err)
		}
		lookbackMin = parsed
	}

	return Config{
		GrafanaURL:   strings.TrimRight(grafanaURL, "/"),
		MimirDSUID:   mimirDSUID,
		GrafanaOrgID: orgID,
		LookbackMs:   lookbackMin * 60 * 1000,
	}, nil
}

// Grafana Explore pane structures -- mirrors the JSON shape the frontend expects.
type exploreQuery struct {
	RefID      string            `json:"refId"`
	Expr       string            `json:"expr"`
	Datasource exploreDatasource `json:"datasource"`
}

type exploreDatasource struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

type exploreRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type explorePane struct {
	Datasource string         `json:"datasource"`
	Queries    []exploreQuery `json:"queries"`
	Range      exploreRange   `json:"range"`
}

func buildExploreURL(cfg Config, expr string) string {
	nowMs := time.Now().UnixMilli()

	panes := map[string]explorePane{
		"a": {
			Datasource: cfg.MimirDSUID,
			Queries: []exploreQuery{
				{
					RefID:      "A",
					Expr:       expr,
					Datasource: exploreDatasource{Type: "prometheus", UID: cfg.MimirDSUID},
				},
			},
			Range: exploreRange{
				From: strconv.FormatInt(nowMs-cfg.LookbackMs, 10),
				To:   strconv.FormatInt(nowMs, 10),
			},
		},
	}

	panesJSON, err := json.Marshal(panes)
	if err != nil {
		// panes is a fixed, known-serializable shape -- this can't fail in
		// practice, but fail loudly rather than emit a broken URL.
		panic(fmt.Sprintf("failed to marshal explore panes: %v", err))
	}

	q := url.Values{}
	q.Set("schemaVersion", "1")
	q.Set("panes", string(panesJSON))
	q.Set("orgId", cfg.GrafanaOrgID)

	return fmt.Sprintf("%s/explore?%s", cfg.GrafanaURL, q.Encode())
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, "mimir-redirect: converts Prometheus classic-UI graph links "+
		"(/graph?g0.expr=...) into Grafana Explore links.\n")
}

func graphHandler(cfg Config, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expr := r.URL.Query().Get("g0.expr")
		if expr == "" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "missing g0.expr query parameter")
			return
		}

		dest := buildExploreURL(cfg, expr)
		log.Info("redirecting", "expr", expr, "url", dest)
		http.Redirect(w, r, dest, http.StatusFound)
	}
}

// statusRecorder wraps a ResponseWriter so the access-log middleware can
// see the status code a handler actually wrote -- http.ResponseWriter
// doesn't expose that after the fact otherwise.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// accessLog is the Go equivalent of gunicorn's --access-logfile: net/http
// doesn't log requests on its own, so without this middleware only the
// handful of explicit log.Info calls inside handlers ever show up, and
// most requests (successful healthz checks, 400s, index hits) produce no
// log output at all.
func accessLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
	})
}

func newLogger() *slog.Logger {
	level := slog.LevelInfo
	_ = level.UnmarshalText([]byte(os.Getenv("LOG_LEVEL")))
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}

func main() {
	log := newLogger()

	cfg, err := loadConfig()
	if err != nil {
		log.Error("config error", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthzHandler)
	mux.HandleFunc("GET /graph", graphHandler(cfg, log))
	mux.HandleFunc("GET /alertmanager/graph", graphHandler(cfg, log))
	mux.HandleFunc("GET /", indexHandler)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      accessLog(log, mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("starting server", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutdown signal received, draining connections")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("error during server shutdown", "err", err)
	}
}
