package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/n0needt0/go-goodies/log"
)

// HealthReportingService handles health reporting to the control service
type HealthReportingService struct {
	controlURL     string
	serviceType    string
	instanceID     string
	instanceAPI    string
	reportInterval time.Duration
	timeout        time.Duration
	httpClient     *http.Client
	enabled        bool
	stopChan       chan bool
	startTime      time.Time
}

// ServiceRegistrationRequest represents a service registration request
type ServiceRegistrationRequest struct {
	ServiceType   string                 `json:"service_type"`
	InstanceID    string                 `json:"instance_id,omitempty"`
	InstanceAPI   string                 `json:"instance_api"`
	Status        string                 `json:"status"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
	Metrics       map[string]interface{} `json:"metrics,omitempty"`
}

// ServiceRegistrationResponse represents a service registration response
type ServiceRegistrationResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	InstanceID  string `json:"instance_id"`
	ServiceType string `json:"service_type"`
}

// HealthReportRequest represents a health report request
type HealthReportRequest struct {
	ServiceType   string                 `json:"service_type"`
	InstanceID    string                 `json:"instance_id"`
	InstanceAPI   string                 `json:"instance_api"`
	Status        string                 `json:"status"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
	Metrics       map[string]interface{} `json:"metrics,omitempty"`
}

// HealthReportResponse represents a health report response
type HealthReportResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// NewHealthReportingService creates a new health reporting service
func NewHealthReportingService(controlURL, serviceType, instanceAPI string, reportInterval, timeout time.Duration) *HealthReportingService {
	// Get hostname for instance ID
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	return &HealthReportingService{
		controlURL:     controlURL,
		serviceType:    serviceType,
		instanceID:     hostname,
		instanceAPI:    instanceAPI,
		reportInterval: reportInterval,
		timeout:        timeout,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		enabled:   true,
		stopChan:  make(chan bool),
		startTime: time.Now(),
	}
}

// Start begins health reporting
func (h *HealthReportingService) Start() {
	if !h.enabled {
		log.Info("Health reporting is disabled")
		return
	}

	log.Infof("Starting health reporting service - reporting to %s every %v", h.controlURL, h.reportInterval)

	// Register service on startup
	if err := h.RegisterService(); err != nil {
		log.Warnf("Failed to register service on startup: %v", err)
	}

	// Start reporting goroutine
	go h.reportingLoop()
}

// Stop stops health reporting
func (h *HealthReportingService) Stop() {
	if h.enabled {
		close(h.stopChan)
		log.Info("Health reporting service stopped")
	}
}

// RegisterService registers this service instance with the control service
func (h *HealthReportingService) RegisterService() error {
	if !h.enabled {
		return nil
	}

	// Generate comprehensive configuration data
	configuration := map[string]interface{}{
		"service_type":    h.serviceType,
		"version":         "1.0.0", // TODO: Get from config
		"instance_api":    h.instanceAPI,
		"report_interval": h.reportInterval.String(),
		"timeout":         h.timeout.String(),
		"processing": map[string]interface{}{
			"max_concurrent_jobs": 10, // TODO: Get from config
			"job_timeout":         "10m",
			"retry_attempts":      3,
			"buffer_size":         1000,
		},
		"pipeline": map[string]interface{}{
			"enable_geoip":            false,
			"config_refresh_interval": "5m",
		},
		"s3_processing": map[string]interface{}{
			"source_bucket":      "intake",
			"destination_bucket": "piper",
		},
		"capabilities": []string{
			"log_parsing",
			"geoip_enrichment",
			"format_transformation",
			"s3_processing",
			"pipeline_orchestration",
		},
	}

	registrationReq := ServiceRegistrationRequest{
		ServiceType:   h.serviceType,
		InstanceID:    h.instanceID,
		InstanceAPI:   h.instanceAPI,
		Status:        "Healthy",
		Configuration: configuration,
		Metrics:       h.generateMetrics(),
	}

	reqBody, err := json.Marshal(registrationReq)
	if err != nil {
		return fmt.Errorf("failed to marshal registration request: %w", err)
	}

	resp, err := h.httpClient.Post(
		h.controlURL+"/api/v1/services/report",
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("service registration failed with status %d", resp.StatusCode)
	}

	var registrationResp ServiceRegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&registrationResp); err != nil {
		return fmt.Errorf("failed to decode registration response: %w", err)
	}

	if !registrationResp.Success {
		return fmt.Errorf("service registration failed: %s", registrationResp.Message)
	}

	log.Infof("Successfully registered service %s with instance ID %s", h.serviceType, h.instanceID)
	return nil
}

// SendHealthReport sends a health report to the control service
func (h *HealthReportingService) SendHealthReport(healthy bool, metrics map[string]interface{}) error {
	if !h.enabled {
		return nil
	}

	status := "Healthy"
	if !healthy {
		status = "Unhealthy"
	}

	// Generate comprehensive configuration data for each report
	configuration := map[string]interface{}{
		"service_type":    h.serviceType,
		"version":         "1.0.0", // TODO: Get from config
		"instance_api":    h.instanceAPI,
		"report_interval": h.reportInterval.String(),
		"timeout":         h.timeout.String(),
		"processing": map[string]interface{}{
			"max_concurrent_jobs": 10, // TODO: Get from config
			"job_timeout":         "10m",
			"retry_attempts":      3,
			"buffer_size":         1000,
		},
		"pipeline": map[string]interface{}{
			"enable_geoip":            false,
			"config_refresh_interval": "5m",
		},
		"s3_processing": map[string]interface{}{
			"source_bucket":      "intake",
			"destination_bucket": "piper",
		},
		"capabilities": []string{
			"log_parsing",
			"geoip_enrichment",
			"format_transformation",
			"s3_processing",
			"pipeline_orchestration",
		},
		"last_report": time.Now().UTC().Format(time.RFC3339),
	}

	// Merge provided metrics with generated metrics
	allMetrics := h.generateMetrics()
	if metrics != nil {
		for k, v := range metrics {
			allMetrics[k] = v
		}
	}

	healthReq := HealthReportRequest{
		ServiceType:   h.serviceType,
		InstanceID:    h.instanceID,
		InstanceAPI:   h.instanceAPI,
		Status:        status,
		Configuration: configuration,
		Metrics:       allMetrics,
	}

	reqBody, err := json.Marshal(healthReq)
	if err != nil {
		return fmt.Errorf("failed to marshal health report: %w", err)
	}

	resp, err := h.httpClient.Post(
		h.controlURL+"/api/v1/services/report",
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		return fmt.Errorf("failed to send health report: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health report failed with status %d", resp.StatusCode)
	}

	log.Debugf("Successfully sent health report for %s (status: %s)", h.serviceType, status)
	return nil
}

// reportingLoop runs the periodic health reporting
func (h *HealthReportingService) reportingLoop() {
	ticker := time.NewTicker(h.reportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Generate health report with basic metrics
			healthy := h.isServiceHealthy()
			metrics := h.generateMetrics()

			if err := h.SendHealthReport(healthy, metrics); err != nil {
				log.Warnf("Failed to send health report: %v", err)
			}

		case <-h.stopChan:
			return
		}
	}
}

// isServiceHealthy performs basic health checks
func (h *HealthReportingService) isServiceHealthy() bool {
	// Basic health check - service is healthy if we can execute this function
	// In a real implementation, this would check:
	// - S3 connectivity (source and destination buckets)
	// - PostgreSQL connectivity
	// - Pipeline processing status
	// - Memory usage
	// - Disk space
	return true
}

// generateMetrics generates basic service metrics
func (h *HealthReportingService) generateMetrics() map[string]interface{} {
	uptime := time.Since(h.startTime)
	return map[string]interface{}{
		"timestamp":         time.Now().Unix(),
		"service_type":      h.serviceType,
		"instance_id":       h.instanceID,
		"uptime_seconds":    uptime.Seconds(),
		"uptime_formatted":  uptime.String(),
		"version":           "1.0.0", // TODO: Get from config
		"last_health_check": time.Now().UTC().Format(time.RFC3339),
		"report_interval":   h.reportInterval.String(),
		"processing_stats": map[string]interface{}{
			"jobs_processed": 0, // TODO: Get from actual stats
			"files_processed": 0, // TODO: Get from actual stats
			"bytes_processed": 0, // TODO: Get from actual stats
		},
	}
}