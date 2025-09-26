package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/n0needt0/go-goodies/log"

	"github.com/n0needt0/bytefreezer-piper/config"
	"github.com/n0needt0/bytefreezer-piper/domain"
)

// PipelineClient handles communication with the control service for pipeline configurations
type PipelineClient struct {
	config     *config.Config
	httpClient *http.Client
	baseURL    string
}

// TenantInfo represents tenant information from control service
type TenantInfo struct {
	TenantID  string   `json:"tenant_id"`
	Name      string   `json:"name"`
	Datasets  []string `json:"datasets"`
	Active    bool     `json:"active"`
	CreatedAt string   `json:"created_at"`
}

// PipelineConfigResponse represents pipeline configuration response from control service
type PipelineConfigResponse struct {
	TenantID      string                        `json:"tenant_id"`
	DatasetID     string                        `json:"dataset_id"`
	Configuration *domain.PipelineConfiguration `json:"configuration"`
	Version       string                        `json:"version"`
	Enabled       bool                          `json:"enabled"`
	CreatedAt     string                        `json:"created_at"`
	UpdatedAt     string                        `json:"updated_at"`
}

// NewPipelineClient creates a new pipeline client
func NewPipelineClient(cfg *config.Config) *PipelineClient {
	return &PipelineClient{
		config:     cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    cfg.Pipeline.ControllerEndpoint,
	}
}

// FetchTenants retrieves tenant information from control service
func (pc *PipelineClient) FetchTenants(ctx context.Context) ([]TenantInfo, error) {
	if pc.config.Dev {
		log.Debugf("Development mode enabled - returning fake tenant data")
		return pc.getFakeTenants(), nil
	}

	if pc.baseURL == "" {
		return nil, fmt.Errorf("controller endpoint not configured")
	}

	url := fmt.Sprintf("%s/api/v2/tenants", pc.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", fmt.Sprintf("%s/%s", pc.config.App.Name, pc.config.App.Version))
	req.Header.Set("Accept", "application/json")

	resp, err := pc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call control service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("control service returned status %d", resp.StatusCode)
	}

	var tenants []TenantInfo
	if err := json.NewDecoder(resp.Body).Decode(&tenants); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return tenants, nil
}

// FetchPipelineConfiguration retrieves pipeline configuration for a specific tenant/dataset
func (pc *PipelineClient) FetchPipelineConfiguration(ctx context.Context, tenantID, datasetID string) (*PipelineConfigResponse, error) {
	if pc.config.Dev {
		log.Debugf("Development mode enabled - returning fake pipeline config for %s/%s", tenantID, datasetID)
		return pc.getFakePipelineConfig(tenantID, datasetID), nil
	}

	if pc.baseURL == "" {
		return nil, fmt.Errorf("controller endpoint not configured")
	}

	url := fmt.Sprintf("%s/api/v2/pipelines/%s/%s", pc.baseURL, tenantID, datasetID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", fmt.Sprintf("%s/%s", pc.config.App.Name, pc.config.App.Version))
	req.Header.Set("Accept", "application/json")

	resp, err := pc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call control service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("control service returned status %d", resp.StatusCode)
	}

	var pipelineConfig PipelineConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&pipelineConfig); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &pipelineConfig, nil
}

// getFakeTenants returns fake tenant data for development mode
func (pc *PipelineClient) getFakeTenants() []TenantInfo {
	return []TenantInfo{
		{
			TenantID:  "customer-1",
			Name:      "Customer One Corp",
			Datasets:  []string{"ebpf-data", "sflow-data"},
			Active:    true,
			CreatedAt: time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		},
	}
}

// getFakePipelineConfig returns fake pipeline configuration for development mode
func (pc *PipelineClient) getFakePipelineConfig(tenantID, datasetID string) *PipelineConfigResponse {
	if tenantID == "customer-1" && datasetID == "ebpf-data" {
		createdAt := time.Now().Add(-24 * time.Hour)
		updatedAt := time.Now().Add(-1 * time.Hour)

		return &PipelineConfigResponse{
			TenantID:  tenantID,
			DatasetID: datasetID,
			Configuration: &domain.PipelineConfiguration{
				ConfigKey: fmt.Sprintf("%s:%s", tenantID, datasetID),
				TenantID:  tenantID,
				DatasetID: datasetID,
				Enabled:   true,
				Version:   "dev-1.0.0",
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
				UpdatedBy: "dev-system",
				Checksum:  "fake-checksum-ebpf",
				Validated: true,
				Filters: []domain.FilterConfig{
					{
						Type: "json_validate",
						Config: map[string]interface{}{
							"fail_on_invalid": true,
						},
						Enabled: false,
					},
					{
						Type: "json_flatten",
						Config: map[string]interface{}{
							"separator": ".",
						},
						Enabled: false,
					},
					{
						Type: "uppercase_keys",
						Config: map[string]interface{}{
							"recursive": true,
						},
						Enabled: false,
					},
					{
						Type: "add_field",
						Config: map[string]interface{}{
							"field": "TESTKEY",
							"value": fmt.Sprintf("%d", time.Now().Unix()),
						},
						Enabled: false,
					},
					{
						Type: "add_field",
						Config: map[string]interface{}{
							"field": "PIPELINE_VERSION",
							"value": "dev-1.0.0",
						},
						Enabled: false,
					},
				},
			},
			Version:   "dev-1.0.0",
			Enabled:   true,
			CreatedAt: createdAt.Format(time.RFC3339),
			UpdatedAt: updatedAt.Format(time.RFC3339),
		}
	}

	if tenantID == "customer-1" && datasetID == "sflow-data" {
		createdAt := time.Now().Add(-24 * time.Hour)
		updatedAt := time.Now().Add(-1 * time.Hour)

		return &PipelineConfigResponse{
			TenantID:  tenantID,
			DatasetID: datasetID,
			Configuration: &domain.PipelineConfiguration{
				ConfigKey: fmt.Sprintf("%s:%s", tenantID, datasetID),
				TenantID:  tenantID,
				DatasetID: datasetID,
				Enabled:   true,
				Version:   "dev-1.0.0",
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
				UpdatedBy: "dev-system",
				Checksum:  "fake-checksum-sflow",
				Validated: true,
				Filters: []domain.FilterConfig{
					{
						Type: "parse",
						Config: map[string]interface{}{
							"parser": "sflow",
						},
						Enabled: false,
					},
					{
						Type: "add_field",
						Config: map[string]interface{}{
							"field": "data_type",
							"value": "sflow",
						},
						Enabled: false,
					},
					{
						Type: "add_field",
						Config: map[string]interface{}{
							"field": "PIPELINE_VERSION",
							"value": "dev-1.0.0",
						},
						Enabled: false,
					},
				},
			},
			Version:   "dev-1.0.0",
			Enabled:   true,
			CreatedAt: createdAt.Format(time.RFC3339),
			UpdatedAt: updatedAt.Format(time.RFC3339),
		}
	}

	// Default empty configuration for unknown tenant/dataset combinations
	createdAt := time.Now().Add(-1 * time.Hour)
	updatedAt := time.Now()

	return &PipelineConfigResponse{
		TenantID:  tenantID,
		DatasetID: datasetID,
		Configuration: &domain.PipelineConfiguration{
			ConfigKey: fmt.Sprintf("%s:%s", tenantID, datasetID),
			TenantID:  tenantID,
			DatasetID: datasetID,
			Enabled:   true,
			Version:   "default-1.0.0",
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
			UpdatedBy: "dev-system",
			Checksum:  "fake-checksum-default",
			Validated: true,
			Filters:   []domain.FilterConfig{},
		},
		Version:   "default-1.0.0",
		Enabled:   true,
		CreatedAt: createdAt.Format(time.RFC3339),
		UpdatedAt: updatedAt.Format(time.RFC3339),
	}
}
