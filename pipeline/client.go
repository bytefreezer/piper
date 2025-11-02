package pipeline

import (
	"context"
	"github.com/bytedance/sonic"
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
	apiKey     string
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

// DatasetResponse represents dataset response from Control Service
type DatasetResponse struct {
	ID          string                 `json:"id"`
	TenantID    string                 `json:"tenant_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Active      bool                   `json:"active"`
	Status      string                 `json:"status"`
	Config      map[string]interface{} `json:"config"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
}

// NewPipelineClient creates a new pipeline client
func NewPipelineClient(cfg *config.Config) *PipelineClient {
	baseURL := cfg.ControlService.BaseURL
	apiKey := cfg.ControlService.APIKey

	return &PipelineClient{
		config:     cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		apiKey:     apiKey,
	}
}

// AccountResponse represents account response from Control Service
type AccountResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// TenantResponse represents tenant response from Control Service
type TenantResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AccountID string `json:"account_id"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// FetchTenants retrieves tenant information from control service
func (pc *PipelineClient) FetchTenants(ctx context.Context) ([]TenantInfo, error) {
	if pc.config.Dev {
		log.Debugf("Development mode enabled - returning fake tenant data")
		return pc.getFakeTenants(), nil
	}

	if pc.baseURL == "" {
		return nil, fmt.Errorf("control service base URL not configured")
	}

	// First, fetch all accounts
	accountsURL := fmt.Sprintf("%s/api/v1/accounts?limit=1000", pc.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", accountsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create accounts request: %w", err)
	}

	req.Header.Set("User-Agent", fmt.Sprintf("%s/%s", pc.config.App.Name, pc.config.App.Version))
	req.Header.Set("Accept", "application/json")

	// Add Bearer token authentication if configured
	if pc.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", pc.apiKey))
	}

	resp, err := pc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call control service for accounts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("control service returned status %d for accounts", resp.StatusCode)
	}

	var accountsResp struct {
		Items []AccountResponse `json:"items"`
		Total int               `json:"total"`
	}

	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&accountsResp); err != nil {
		return nil, fmt.Errorf("failed to decode accounts response: %w", err)
	}

	// Then, fetch tenants for each account
	allTenants := make([]TenantInfo, 0)

	for _, account := range accountsResp.Items {
		if !account.Active {
			continue
		}

		tenantsURL := fmt.Sprintf("%s/api/v1/accounts/%s/tenants?limit=1000", pc.baseURL, account.ID)
		req, err := http.NewRequestWithContext(ctx, "GET", tenantsURL, nil)
		if err != nil {
			log.Warnf("Failed to create tenants request for account %s: %v", account.ID, err)
			continue
		}

		req.Header.Set("User-Agent", fmt.Sprintf("%s/%s", pc.config.App.Name, pc.config.App.Version))
		req.Header.Set("Accept", "application/json")

		if pc.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+pc.apiKey)
		}

		resp, err := pc.httpClient.Do(req)
		if err != nil {
			log.Warnf("Failed to fetch tenants for account %s: %v", account.ID, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			log.Warnf("Control service returned status %d for tenants in account %s", resp.StatusCode, account.ID)
			continue
		}

		var tenantsResp struct {
			Items []TenantResponse `json:"items"`
			Total int              `json:"total"`
		}

		if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&tenantsResp); err != nil {
			resp.Body.Close()
			log.Warnf("Failed to decode tenants response for account %s: %v", account.ID, err)
			continue
		}
		resp.Body.Close()

		// Fetch datasets for each tenant
		for _, tenant := range tenantsResp.Items {
			if !tenant.Active {
				continue
			}

			datasetsURL := fmt.Sprintf("%s/api/v1/tenants/%s/datasets?limit=1000", pc.baseURL, tenant.ID)
			req, err := http.NewRequestWithContext(ctx, "GET", datasetsURL, nil)
			if err != nil {
				log.Warnf("Failed to create datasets request for tenant %s: %v", tenant.ID, err)
				continue
			}

			req.Header.Set("User-Agent", fmt.Sprintf("%s/%s", pc.config.App.Name, pc.config.App.Version))
			req.Header.Set("Accept", "application/json")

			if pc.apiKey != "" {
				req.Header.Set("Authorization", "Bearer "+pc.apiKey)
			}

			resp, err := pc.httpClient.Do(req)
			if err != nil {
				log.Warnf("Failed to fetch datasets for tenant %s: %v", tenant.ID, err)
				continue
			}

			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				log.Warnf("Control service returned status %d for datasets in tenant %s", resp.StatusCode, tenant.ID)
				continue
			}

			var datasetsResp struct {
				Items []DatasetResponse `json:"items"`
				Total int               `json:"total"`
			}

			if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&datasetsResp); err != nil {
				resp.Body.Close()
				log.Warnf("Failed to decode datasets response for tenant %s: %v", tenant.ID, err)
				continue
			}
			resp.Body.Close()

			// Extract dataset IDs
			datasets := make([]string, 0, len(datasetsResp.Items))
			for _, dataset := range datasetsResp.Items {
				if dataset.Active {
					datasets = append(datasets, dataset.ID)
				}
			}

			allTenants = append(allTenants, TenantInfo{
				TenantID:  tenant.ID,
				Name:      tenant.Name,
				Datasets:  datasets,
				Active:    tenant.Active,
				CreatedAt: tenant.CreatedAt,
			})
		}
	}

	return allTenants, nil
}

// FetchPipelineConfiguration retrieves pipeline configuration for a specific tenant/dataset
func (pc *PipelineClient) FetchPipelineConfiguration(ctx context.Context, tenantID, datasetID string) (*PipelineConfigResponse, error) {
	if pc.config.Dev {
		log.Debugf("Development mode enabled - returning fake pipeline config for %s/%s", tenantID, datasetID)
		return pc.getFakePipelineConfig(tenantID, datasetID), nil
	}

	if pc.baseURL == "" {
		return nil, fmt.Errorf("control service base URL not configured")
	}

	// Fetch dataset from Control Service API
	url := fmt.Sprintf("%s/api/v1/tenants/%s/datasets/%s", pc.baseURL, tenantID, datasetID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", fmt.Sprintf("%s/%s", pc.config.App.Name, pc.config.App.Version))
	req.Header.Set("Accept", "application/json")

	// Add Bearer token authentication if configured
	if pc.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", pc.apiKey))
	}

	resp, err := pc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call control service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		log.Debugf("Dataset %s/%s not found in Control Service", tenantID, datasetID)
		return nil, fmt.Errorf("dataset not found: %s/%s", tenantID, datasetID)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("control service returned status %d", resp.StatusCode)
	}

	var dataset DatasetResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&dataset); err != nil {
		return nil, fmt.Errorf("failed to decode dataset response: %w", err)
	}

	// Convert dataset config to pipeline configuration
	pipelineConfig := pc.convertDatasetToPipelineConfig(&dataset)

	return pipelineConfig, nil
}

// convertDatasetToPipelineConfig converts a dataset response to a pipeline configuration
func (pc *PipelineClient) convertDatasetToPipelineConfig(dataset *DatasetResponse) *PipelineConfigResponse {
	configKey := fmt.Sprintf("%s:%s", dataset.TenantID, dataset.ID)

	// Parse timestamps
	createdAt, _ := time.Parse(time.RFC3339, dataset.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, dataset.UpdatedAt)

	// Build filters from dataset config
	filters := []domain.FilterConfig{}

	// Extract processing/transform configuration if present
	if dataset.Config != nil {
		// Check if transform is enabled
		if transform, ok := dataset.Config["transform"].(map[string]interface{}); ok {
			if enabled, ok := transform["enabled"].(bool); ok && enabled {
				// Add transform filters based on config
				log.Debugf("Transform enabled for %s/%s", dataset.TenantID, dataset.ID)
			}
		}

		// Check if processing enrichment is enabled
		if processing, ok := dataset.Config["processing"].(map[string]interface{}); ok {
			if enrichment, ok := processing["enrichment"].(map[string]interface{}); ok {
				if enabled, ok := enrichment["enabled"].(bool); ok && enabled {
					log.Debugf("Enrichment enabled for %s/%s", dataset.TenantID, dataset.ID)
				}
			}
		}
	}

	// For now, return a basic pipeline config with empty filters
	// In the future, this can be expanded to map dataset config to specific filters
	pipelineConfig := &PipelineConfigResponse{
		TenantID:  dataset.TenantID,
		DatasetID: dataset.ID,
		Configuration: &domain.PipelineConfiguration{
			ConfigKey: configKey,
			TenantID:  dataset.TenantID,
			DatasetID: dataset.ID,
			Enabled:   dataset.Active,
			Version:   "1.0.0",
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
			UpdatedBy: "control-service",
			Checksum:  fmt.Sprintf("cs-%s-%s", dataset.TenantID, dataset.ID),
			Validated: true,
			Filters:   filters,
		},
		Version:   "1.0.0",
		Enabled:   dataset.Active,
		CreatedAt: dataset.CreatedAt,
		UpdatedAt: dataset.UpdatedAt,
	}

	return pipelineConfig
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
