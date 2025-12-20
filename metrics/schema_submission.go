// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package metrics

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytefreezer/goodies/log"
)

// SchemaSubmissionClient handles sending dataset schemas and samples to the control service
type SchemaSubmissionClient struct {
	controlURL string
	apiKey     string
	httpClient *http.Client
	enabled    bool
}

// SampleData represents a data sample to submit
type SampleData struct {
	LineNumber int                    `json:"line_number"`
	SampleData map[string]interface{} `json:"sample_data"`
	BatchID    string                 `json:"batch_id"`
}

// SchemaSubmissionRequest represents a schema submission request to control service
type SchemaSubmissionRequest struct {
	SchemaType string       `json:"schema_type"` // "input" or "output"
	Schema     interface{}  `json:"schema"`      // The inferred schema
	Samples    []SampleData `json:"samples"`     // Sample records (max 10)
}

// SchemaSubmissionResponse represents the response from submitting schema
type SchemaSubmissionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// NewSchemaSubmissionClient creates a new schema submission client
func NewSchemaSubmissionClient(controlURL string, apiKey string, timeoutSeconds int, enabled bool) *SchemaSubmissionClient {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 10 // Schema submission may take longer
	}

	return &SchemaSubmissionClient{
		controlURL: controlURL,
		apiKey:     apiKey,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
		enabled: enabled,
	}
}

// SubmitSchema sends dataset schema and samples to the control service
// This is called after successful data processing
func (c *SchemaSubmissionClient) SubmitSchema(ctx context.Context, tenantID, datasetID, schemaType string,
	schema interface{}, samples []SampleData) error {

	if !c.enabled {
		log.Debug("Schema submission is disabled")
		return nil
	}

	// Limit to 10 samples
	if len(samples) > 10 {
		samples = samples[:10]
	}

	// Build the submission request
	schemaReq := SchemaSubmissionRequest{
		SchemaType: schemaType,
		Schema:     schema,
		Samples:    samples,
	}

	reqBody, err := sonic.Marshal(schemaReq)
	if err != nil {
		return fmt.Errorf("failed to marshal schema request: %w", err)
	}

	// Build the URL - matches control service API
	url := fmt.Sprintf("%s/api/v1/tenants/%s/datasets/%s/schema",
		c.controlURL, tenantID, datasetID)

	log.Debugf("Submitting %s schema and %d samples to %s", schemaType, len(samples), url)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create schema request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Don't fail the processing if schema submission fails - best effort
		log.Warnf("Failed to submit schema to control service: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Warnf("Control service returned non-200 status for schema submission: %d", resp.StatusCode)
		return fmt.Errorf("control service returned status %d", resp.StatusCode)
	}

	log.Infof("Successfully submitted %s schema and %d samples for %s/%s", schemaType, len(samples), tenantID, datasetID)
	return nil
}

// GetSamplesResponse represents the response from retrieving samples
type GetSamplesResponse struct {
	Samples    []SampleData `json:"samples"`
	TotalCount int          `json:"total_count"`
	TenantID   string       `json:"tenant_id"`
	DatasetID  string       `json:"dataset_id"`
	SampleType string       `json:"sample_type"`
}

// GetStoredSchemaResponse represents the response from retrieving stored schema
type GetStoredSchemaResponse struct {
	Schema     interface{} `json:"schema"`
	TenantID   string      `json:"tenant_id"`
	DatasetID  string      `json:"dataset_id"`
	SchemaType string      `json:"schema_type"`
	UpdatedAt  string      `json:"updated_at"`
}

// GetSamples retrieves stored samples from the control service
func (c *SchemaSubmissionClient) GetSamples(ctx context.Context, tenantID, datasetID, sampleType string, limit int) ([]SampleData, error) {
	if !c.enabled {
		return nil, fmt.Errorf("sample retrieval is disabled")
	}

	if limit <= 0 {
		limit = 10
	}

	// Build the URL - matches control service API
	url := fmt.Sprintf("%s/api/v1/tenants/%s/datasets/%s/samples?type=%s&limit=%d",
		c.controlURL, tenantID, datasetID, sampleType, limit)

	log.Debugf("Retrieving %s samples from %s", sampleType, url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create samples request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve samples from control service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("control service returned status %d", resp.StatusCode)
	}

	var response GetSamplesResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode samples response: %w", err)
	}

	log.Debugf("Retrieved %d %s samples for %s/%s", len(response.Samples), sampleType, tenantID, datasetID)
	return response.Samples, nil
}

// GetStoredSchema retrieves stored schema from the control service
func (c *SchemaSubmissionClient) GetStoredSchema(ctx context.Context, tenantID, datasetID, schemaType string) (interface{}, error) {
	if !c.enabled {
		return nil, fmt.Errorf("schema retrieval is disabled")
	}

	// Build the URL - matches control service API
	url := fmt.Sprintf("%s/api/v1/tenants/%s/datasets/%s/schema/stored?type=%s",
		c.controlURL, tenantID, datasetID, schemaType)

	log.Debugf("Retrieving %s schema from %s", schemaType, url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create schema request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve schema from control service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("control service returned status %d", resp.StatusCode)
	}

	var response GetStoredSchemaResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode schema response: %w", err)
	}

	log.Debugf("Retrieved %s schema for %s/%s", schemaType, tenantID, datasetID)
	return response.Schema, nil
}
