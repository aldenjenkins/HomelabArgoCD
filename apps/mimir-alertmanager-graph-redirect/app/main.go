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

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// version is stamped at build time via -ldflags "-X main.version=...".
// Falls back to "dev" for local builds.
var version = "dev"

const serviceName = "mimir-redirect"

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

// buildExploreURL is wrapped in its own span since it's the one piece of
// real work this service does -- everything else is HTTP plumbing that
// otelhttp already covers automatically.
func buildExploreURL(ctx context.Context, cfg Config, expr string) string {
	_, span := otel.Tracer(serviceName).Start(ctx, "buildExploreURL",
		trace.WithAttributes(attribute.String("mimir.promql_expr", expr)),
	)
	defer span.End()

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

	dest := fmt.Sprintf("%s/explore?%s", cfg.GrafanaURL, q.Encode())
	span.SetAttributes(attribute.String("mimir.explore_url", dest))
	return dest
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
		ctx := r.Context()

		expr := r.URL.Query().Get("g0.expr")
		if expr == "" {
			trace.SpanFromContext(ctx).SetAttributes(attribute.Bool("mimir.missing_expr", true))
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "missing g0.expr query parameter")
			return
		}

		dest := buildExploreURL(ctx, cfg, expr)
		log.InfoContext(ctx, "redirecting", "expr", expr, "url", dest)
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
// handful of explicit log calls inside handlers ever show up, and most
// requests (healthz checks, 400s, index hits) would produce no log output
// at all. Placed inside the otelhttp handler in the middleware chain so
// r.Context() already carries the active span, letting every access-log
// line include the trace_id it belongs to.
func accessLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		sc := trace.SpanContextFromContext(r.Context())
		log.LogAttrs(r.Context(), slog.LevelInfo, "request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("query", r.URL.RawQuery),
			slog.Int("status", rec.status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("remote_addr", r.RemoteAddr),
			slog.String("user_agent", r.UserAgent()),
			slog.String("trace_id", sc.TraceID().String()),
		)
	})
}

func newLogger() *slog.Logger {
	level := slog.LevelInfo
	_ = level.UnmarshalText([]byte(os.Getenv("LOG_LEVEL")))
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}

// initTracerProvider sets up the OTel SDK to export spans via OTLP/HTTP.
// Endpoint, headers, TLS, and protocol are all configured through the
// standard OTEL_EXPORTER_OTLP_* environment variables that otlptracehttp
// reads automatically -- nothing is hardcoded here, so this points at
// whatever collector is configured purely via env vars in the deployment's
// ConfigMap.
func initTracerProvider(ctx context.Context) (func(context.Context) error, error) {
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP exporter: %w", err)
	}

	res, err := sdkresource.New(ctx,
		sdkresource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
		),
		sdkresource.WithFromEnv(), // OTEL_RESOURCE_ATTRIBUTES, OTEL_SERVICE_NAME override support
		sdkresource.WithHost(),
		sdkresource.WithProcessRuntimeVersion(),
	)
	if err != nil {
		return nil, fmt.Errorf("building OTel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

func main() {
	log := newLogger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracer, err := initTracerProvider(ctx)
	if err != nil {
		log.Error("failed to initialize tracing", "err", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracer(shutdownCtx); err != nil {
			log.Error("error shutting down tracer provider", "err", err)
		}
	}()

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

	// otelhttp is the outermost layer so it creates the span first; accessLog
	// runs inside it so r.Context() already carries that span when it logs.
	// /healthz is filtered out of tracing (kube-probe fires every couple
	// seconds -- not worth a span per hit) but is still access-logged below.
	handler := otelhttp.NewHandler(accessLog(log, mux), "http.server",
		otelhttp.WithFilter(func(r *http.Request) bool {
			return r.URL.Path != "/healthz"
		}),
	)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("starting server", "addr", srv.Addr, "version", version)
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
