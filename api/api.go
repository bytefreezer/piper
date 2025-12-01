package api

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/bytefreezer/goodies/log"
	"github.com/bytefreezer/piper/config"
	"github.com/swaggest/openapi-go/openapi3"
	"github.com/swaggest/rest/web"
	swgui "github.com/swaggest/swgui/v5emb"
	"go.opentelemetry.io/otel/metric"
)

// Services interface to match receiver pattern
type Services interface {
	GetPipelineConfigAsInterface(ctx context.Context, tenantID, datasetID string) (interface{}, error)
	GetCacheStats() map[string]interface{}
	GetCachedPipelineList(ctx context.Context) ([]map[string]interface{}, error)

	// Transformation testing methods
	GetSchemaAndSamples(ctx context.Context, tenantID, datasetID string, count int) ([]SchemaField, []TransformationSample, int, error)
	TestTransformation(ctx context.Context, tenantID, datasetID string, filters []FilterConfig, samples []TransformationSample) ([]TransformationResult, error)
	ValidateFreshData(ctx context.Context, tenantID, datasetID string, filters []FilterConfig, count int) ([]TransformationResult, string, int, error)
	ActivateTransformation(ctx context.Context, tenantID, datasetID string, filters []FilterConfig, enabled bool) (string, error)
	GetTransformationStats(ctx context.Context, tenantID, datasetID string) (TransformationStats, error)
	PreviewTransformation(ctx context.Context, tenantID, datasetID string, count int) ([]TransformationResult, bool, int, string, error)
}

type API struct {
	Services   Services
	ApiMetrics map[string]metric.Int64Counter
	HttpServer *http.Server
	Config     *config.Config
	sync.RWMutex
}

// NewAPI creates a new API instance following receiver pattern
func NewAPI(services Services, conf *config.Config) *API {
	return &API{
		Services:   services,
		ApiMetrics: make(map[string]metric.Int64Counter),
		Config:     conf,
	}
}

// NewRouter returns a new router serving API endpoints
func (api *API) NewRouter() *web.Service {
	service := web.NewService(openapi3.NewReflector())

	// Configure OpenAPI schema
	service.OpenAPISchema().SetTitle("ByteFreezer Piper API")
	service.OpenAPISchema().SetDescription("ByteFreezer Piper API for pipeline management and monitoring")
	service.OpenAPISchema().SetVersion("v1.0.0")

	// Apply defaults for decoder factory
	service.DecoderFactory.ApplyDefaults = true

	// Wrap to finalize middleware setup
	service.Wrap()

	// Health check endpoint (test DB connection)
	service.Get("/api/v1/health", api.HealthCheck())

	// Configuration endpoint
	service.Get("/api/v1/config", api.GetConfig())

	// Pipeline endpoints
	service.Get("/api/v1/pipelines", api.GetPipelineList())
	service.Get("/api/v1/pipelines/{tenantId}/{datasetId}", api.GetPipelineDetails())

	// Transformation testing endpoints
	service.Get("/api/v1/transformations/{tenantId}/{datasetId}/schema", api.GetSchema())
	service.Post("/api/v1/transformations/test", api.TestTransformation())
	service.Post("/api/v1/transformations/validate", api.ValidateFreshData())
	service.Post("/api/v1/transformations/activate", api.ActivateTransformation())
	service.Get("/api/v1/transformations/{tenantId}/{datasetId}/stats", api.GetTransformationStats())
	service.Get("/api/v1/transformations/{tenantId}/{datasetId}/preview", api.PreviewTransformation())

	// API documentation
	service.Docs("/docs", swgui.New)

	// Root redirect to documentation
	service.Router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs", http.StatusFound)
	})

	return service
}

// Serve serves http endpoints
func (api *API) Serve(address string, router http.Handler) {
	log.Infof("API server started on %s", address)

	api.HttpServer = &http.Server{
		Addr:           address,
		Handler:        router,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}
	err := api.HttpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		log.Info("API server closed")
	} else {
		log.Errorf("API server failed and closed: %v", err)
	}
}

// Stop stops the server
func (api *API) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer func() {
		api.HttpServer = nil
		cancel()
	}()

	if api.HttpServer != nil {
		if err := api.HttpServer.Shutdown(ctx); err != nil {
			log.Errorf("Error shutting down API server: %v", err)
		}
	}

	log.Info("API server shut down gracefully")
}
