package alerts

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytefreezer/goodies/log"
)

// SOCConfig represents SOC alert configuration
type SOCConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Endpoint string `mapstructure:"endpoint"`
	Timeout  int    `mapstructure:"timeout"`
}

// AppConfig represents application configuration
type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
}

// AlertClientConfig represents the full config needed by SOCAlertClient
type AlertClientConfig struct {
	SOC SOCConfig `mapstructure:"soc"`
	App AppConfig `mapstructure:"app"`
	Dev bool      `mapstructure:"dev"`
}

type SOCAlertClient struct {
	config     AlertClientConfig
	httpClient *http.Client
}

type AlertPayload struct {
	Service     string    `json:"service"`
	Severity    string    `json:"severity"`
	Title       string    `json:"title"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	Environment string    `json:"environment"`
	Host        string    `json:"host,omitempty"`
	Details     string    `json:"details,omitempty"`
}

const (
	SEVERITY_CRITICAL = "critical"
	SEVERITY_HIGH     = "high"
	SEVERITY_MEDIUM   = "medium"
	SEVERITY_LOW      = "low"
)

func NewSOCAlertClient(conf AlertClientConfig) *SOCAlertClient {
	timeout := time.Duration(conf.SOC.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &SOCAlertClient{
		config: conf,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// SendAlert sends an alert to the SOC endpoint
func (sac *SOCAlertClient) SendAlert(severity, title, message, details string) error {
	if !sac.config.SOC.Enabled {
		log.Debug("SOC alerts disabled, skipping alert")
		return nil
	}

	if sac.config.SOC.Endpoint == "" {
		log.Warn("SOC alert endpoint not configured")
		return nil
	}

	// Skip alerts in development mode unless it's critical
	if sac.config.Dev && severity != SEVERITY_CRITICAL {
		log.Debugf("Development mode - skipping %s alert: %s", severity, title)
		return nil
	}

	environment := "production"
	if sac.config.Dev {
		environment = "development"
	}

	payload := AlertPayload{
		Service:     sac.config.App.Name,
		Severity:    severity,
		Title:       title,
		Message:     message,
		Timestamp:   time.Now().UTC(),
		Environment: environment,
		Details:     details,
	}

	return sac.sendAlertPayload(payload)
}

// SendCriticalAlert sends a critical alert (always sent, even in dev mode)
func (sac *SOCAlertClient) SendCriticalAlert(title, message, details string) error {
	return sac.SendAlert(SEVERITY_CRITICAL, title, message, details)
}

// SendHighAlert sends a high severity alert
func (sac *SOCAlertClient) SendHighAlert(title, message, details string) error {
	return sac.SendAlert(SEVERITY_HIGH, title, message, details)
}

// SendMediumAlert sends a medium severity alert
func (sac *SOCAlertClient) SendMediumAlert(title, message, details string) error {
	return sac.SendAlert(SEVERITY_MEDIUM, title, message, details)
}

// sendAlertPayload sends the actual HTTP request
func (sac *SOCAlertClient) sendAlertPayload(payload AlertPayload) error {
	jsonData, err := sonic.Marshal(payload)
	if err != nil {
		log.Errorf("Failed to marshal SOC alert payload: %v", err)
		return fmt.Errorf("failed to marshal alert payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), sac.httpClient.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", sac.config.SOC.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Errorf("Failed to create SOC alert request: %v", err)
		return fmt.Errorf("failed to create alert request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("%s/%s", sac.config.App.Name, sac.config.App.Version))

	log.Debugf("Sending SOC alert to %s: %s - %s", sac.config.SOC.Endpoint, payload.Severity, payload.Title)

	resp, err := sac.httpClient.Do(req)
	if err != nil {
		log.Errorf("Failed to send SOC alert: %v", err)
		return fmt.Errorf("failed to send alert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Errorf("SOC alert endpoint returned status %d", resp.StatusCode)
		return fmt.Errorf("SOC alert endpoint returned status %d", resp.StatusCode)
	}

	log.Debugf("SOC alert sent successfully (status: %d)", resp.StatusCode)
	return nil
}

// SendServiceStartAlert sends an alert when service starts
func (sac *SOCAlertClient) SendServiceStartAlert() error {
	return sac.SendMediumAlert(
		"Service Started",
		fmt.Sprintf("%s service has started successfully", sac.config.App.Name),
		fmt.Sprintf("Version: %s, Environment: %s", sac.config.App.Version, func() string {
			if sac.config.Dev {
				return "development"
			}
			return "production"
		}()),
	)
}

// Piper-specific SOC alert methods

// SendProcessingFailureAlert sends alert when processing failures exceed threshold
func (sac *SOCAlertClient) SendProcessingFailureAlert(tenantID, datasetID string, failureCount, totalProcessed int, threshold float64, details string) error {
	failureRate := float64(failureCount) / float64(totalProcessed) * 100

	severity := SEVERITY_MEDIUM
	if failureRate >= threshold*2 {
		severity = SEVERITY_CRITICAL
	} else if failureRate >= threshold*1.5 {
		severity = SEVERITY_HIGH
	}

	return sac.SendAlert(
		severity,
		fmt.Sprintf("Processing Failure Threshold Exceeded - %s/%s", tenantID, datasetID),
		fmt.Sprintf("Processing failure rate %.2f%% exceeds threshold %.2f%% for tenant %s dataset %s",
			failureRate, threshold, tenantID, datasetID),
		fmt.Sprintf("Failures: %d/%d, Details: %s", failureCount, totalProcessed, details),
	)
}

// SendFormatProcessorFailureAlert sends alert when format processor fails repeatedly
func (sac *SOCAlertClient) SendFormatProcessorFailureAlert(processorType string, failureCount int, err error) error {
	severity := SEVERITY_MEDIUM
	if failureCount >= 10 {
		severity = SEVERITY_HIGH
	}
	if failureCount >= 25 {
		severity = SEVERITY_CRITICAL
	}

	return sac.SendAlert(
		severity,
		fmt.Sprintf("Format Processor Repeated Failures - %s", processorType),
		fmt.Sprintf("Format processor %s has failed %d consecutive times", processorType, failureCount),
		fmt.Sprintf("Latest error: %v", err),
	)
}

// SendDiscoveryFailureAlert sends alert when file discovery fails repeatedly
func (sac *SOCAlertClient) SendDiscoveryFailureAlert(failureCount int, err error) error {
	severity := SEVERITY_MEDIUM
	if failureCount >= 5 {
		severity = SEVERITY_HIGH
	}
	if failureCount >= 10 {
		severity = SEVERITY_CRITICAL
	}

	return sac.SendAlert(
		severity,
		"File Discovery Repeated Failures",
		fmt.Sprintf("File discovery has failed %d consecutive times", failureCount),
		fmt.Sprintf("Latest error: %v", err),
	)
}

// SendS3OperationFailureAlert sends alert when S3 operations fail repeatedly
func (sac *SOCAlertClient) SendS3OperationFailureAlert(operation string, failureCount int, err error) error {
	severity := SEVERITY_MEDIUM
	if failureCount >= 5 {
		severity = SEVERITY_HIGH
	}
	if failureCount >= 10 {
		severity = SEVERITY_CRITICAL
	}

	return sac.SendAlert(
		severity,
		fmt.Sprintf("S3 Operation Repeated Failures - %s", operation),
		fmt.Sprintf("S3 %s operation has failed %d consecutive times", operation, failureCount),
		fmt.Sprintf("Latest error: %v", err),
	)
}

// SendDatabaseConnectionFailureAlert sends alert when database connection fails
func (sac *SOCAlertClient) SendDatabaseConnectionFailureAlert(err error) error {
	return sac.SendCriticalAlert(
		"Database Connection Failed",
		"Failed to connect to PostgreSQL database - service cannot function",
		fmt.Sprintf("Error: %v", err),
	)
}
