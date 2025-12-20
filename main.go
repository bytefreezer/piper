// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytefreezer/goodies/log"

	"github.com/bytefreezer/piper/api"
	"github.com/bytefreezer/piper/config"
	"github.com/bytefreezer/piper/geoip"
	"github.com/bytefreezer/piper/services"
	"github.com/bytefreezer/piper/storage"
)

var (
	configPath  = flag.String("config", "config.yaml", "Path to configuration file")
	showVersion = flag.Bool("version", false, "Show version and exit")
	showHelp    = flag.Bool("help", false, "Show help and exit")
	version     = "1.0.0" // Will be set during build
	buildTime   = "unknown"
	gitCommit   = "unknown"
)

// Execute executes the root command.
func Run() error {
	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("bytefreezer-piper version %s (built %s, commit %s)\n", version, buildTime, gitCommit)
		os.Exit(0)
	}

	// Handle help flag
	if *showHelp {
		fmt.Printf("ByteFreezer Piper - Data processing and pipeline service\n\n")
		fmt.Printf("Usage: %s [options]\n\n", os.Args[0])
		fmt.Printf("Options:\n")
		flag.PrintDefaults()
		os.Exit(0)
	}

	// Suppress AWS SDK warnings about checksums (MinIO compatibility)
	stdlog.SetOutput(io.Discard)

	// Initialize logger
	log.Infof("Starting bytefreezer-piper version %s", version)

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Infof("Configuration loaded successfully")
	log.Infof("Instance ID: %s", cfg.App.InstanceID)

	log.Infof("Source bucket: %s (prefix: %s)", cfg.S3Source.BucketName, "(none)")
	log.Infof("Destination bucket: %s (prefix: %s)", cfg.S3Dest.BucketName, "(none)")

	// Cleanup abandoned locks from previous runs of this instance
	log.Infof("Cleaning up abandoned locks from previous runs of this instance (%s)...", cfg.App.InstanceID)
	if stateManager, err := storage.NewControlAPIStateManager(&cfg.ControlService, cfg.App.InstanceID); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := stateManager.CleanupInstanceLocks(ctx, cfg.App.InstanceID); err != nil {
			log.Warnf("Failed to cleanup instance locks on startup: %v", err)
		} else {
			log.Infof("Instance lock cleanup completed successfully for %s", cfg.App.InstanceID)
		}
		stateManager.Close()
	} else {
		log.Warnf("Failed to create state manager for lock cleanup: %v", err)
	}

	// Cleanup stale operations from previous runs (self-healing)
	log.Infof("Cleaning up stale operations from previous runs...")
	cleanupStaleOperations(cfg)

	// Initialize GeoIP updater
	geoipUpdater, err := geoip.NewUpdater(cfg)
	if err != nil {
		log.Errorf("Failed to initialize GeoIP updater: %v", err)
		// Continue without GeoIP updates rather than failing completely
		geoipUpdater = nil
	}

	// Business Logic - Initialize services
	servicesInstance := services.NewServices(cfg)

	// Create server
	server := NewServer(servicesInstance, cfg)

	// Create API server
	server.HttpApi = api.NewAPI(servicesInstance, cfg)

	// Define housekeeping function for pipeline database updates and GeoIP updates
	housekeepingFn := func() {
		log.Debug("Starting housekeeping cycle...")

		// Update pipeline database during housekeeping
		log.Debug("Updating pipeline database during housekeeping...")
		if err := servicesInstance.PipelineDatabase.UpdateDatabase(server.ctx); err != nil {
			log.Errorf("Failed to update pipeline database during housekeeping: %v", err)
			return
		}
		log.Debug("Pipeline database updated successfully")

		// Update GeoIP databases during housekeeping
		if geoipUpdater != nil {
			log.Debug("Checking for GeoIP database updates during housekeeping...")
			if err := geoipUpdater.CheckAndUpdate(server.ctx); err != nil {
				log.Errorf("Failed to update GeoIP databases during housekeeping: %v", err)
			} else {
				log.Debug("GeoIP database update check completed successfully")
			}
		}

		log.Debug("Housekeeping cycle completed")
	}

	// Start background services
	go server.Start(housekeepingFn, nil)

	// Start API server on configured port
	apiPort := strconv.Itoa(cfg.Server.ApiPort)
	server.HttpApi.Serve(":"+apiPort, server.HttpApi.NewRouter())

	return nil
}

// Server provides basic service functions and state common to all service types
type Server struct {
	Config   *config.Config
	Name     string
	quitterC chan time.Duration
	HttpApi  *api.API
	Services *services.Services
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewServer(services *services.Services, conf *config.Config) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		Config:   conf,
		Name:     conf.App.Name,
		quitterC: make(chan time.Duration),
		Services: services,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (svc *Server) Start(housekeepingFn func(), quitterFn func(time.Duration)) {
	// Capture geoipUpdater from Run() function scope
	geoipUpdater, _ := geoip.NewUpdater(svc.Config)
	// exit cleanly on signal
	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGTERM)
	go func() {
		sig := <-signalC
		log.Debugf("Received signal %v", sig)

		if err := svc.Stop(2 * time.Second); err != nil {
			log.Fatalf("error stopping service: %v", err)
		}
	}()

	// Start the main piper service
	if err := svc.Services.PiperService.Start(svc.ctx); err != nil {
		log.Fatalf("Failed to start piper service: %v", err)
	}

	// Start health reporting if enabled
	if svc.Services.HealthReporter != nil {
		svc.Services.HealthReporter.Start()
		log.Info("Health reporter started successfully")
	}

	// Start transformation metrics reporter if enabled
	if svc.Services.MetricsReporter != nil {
		svc.Services.MetricsReporter.Start(svc.ctx)
		log.Info("Transformation metrics reporter started successfully")
	}

	// Start transformation job service if enabled
	if svc.Services.TransformationJobService != nil {
		go svc.Services.TransformationJobService.Start(svc.ctx)
		log.Info("Transformation job service started successfully")
	}

	baseInterval := time.Duration(svc.Config.Housekeeping.IntervalSeconds) * time.Second
	if baseInterval <= 0 {
		baseInterval = 10 * time.Minute
	}

	log.Infof("Starting housekeeping with interval %v", baseInterval)

	// Run initial housekeeping immediately on startup
	log.Info("Running initial housekeeping on startup...")
	if housekeepingFn != nil && svc.Config.Housekeeping.Enabled {
		housekeepingFn()
	}

	// Run initial GeoIP update on startup (separate from housekeeping)
	if geoipUpdater != nil {
		log.Info("Running initial GeoIP database update on startup...")
		if err := geoipUpdater.CheckAndUpdate(svc.ctx); err != nil {
			log.Errorf("Failed to update GeoIP databases on startup: %v", err)
		} else {
			log.Info("Initial GeoIP database update completed successfully")
		}
	}

	// Continue with regular intervals
	for {
		timer := time.NewTimer(baseInterval)

		select {
		case <-timer.C:
			log.Debug("housekeeping")
			if housekeepingFn != nil && svc.Config.Housekeeping.Enabled {
				housekeepingFn()
			}

		case timeout := <-svc.quitterC:
			log.Debug("exiting service")
			timer.Stop()

			// Cleanup operations immediately - don't wait for completion
			log.Info("Cleaning up in-progress operations before shutdown...")
			cleanupStaleOperations(svc.Config)

			if quitterFn != nil {
				quitterFn(timeout)
			}

			svc.HttpApi.Stop()

			// Stop transformation job service
			if svc.Services.TransformationJobService != nil {
				svc.Services.TransformationJobService.Stop()
				log.Info("Transformation job service stopped")
			}

			// Stop metrics reporter
			if svc.Services.MetricsReporter != nil {
				svc.Services.MetricsReporter.Stop()
				log.Info("Transformation metrics reporter stopped")
			}

			// Stop health reporting
			if svc.Services.HealthReporter != nil {
				svc.Services.HealthReporter.Stop()
				log.Info("Health reporter stopped")
			}

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), timeout)
			defer shutdownCancel()
			svc.Services.PiperService.Stop(shutdownCtx)
			svc.cancel()
			return
		}
	}
}

func (svc *Server) Stop(timeout time.Duration) error {
	log.Debugf("sending timeout %s to quitterC:", timeout)

	// Try to send timeout to main loop, wait up to the timeout duration
	select {
	case svc.quitterC <- timeout:
		log.Debug("sent timeout to quitterC - graceful shutdown initiated")
	case <-time.After(timeout):
		log.Warn("timed out sending shutdown signal - forcing shutdown")
		close(svc.quitterC)
		log.Debug("forced close of quitterC channel")
	}
	return nil
}

func main() {
	err := Run()
	if err != nil {
		log.Fatalf("failed to start: %s\n", err.Error())
		os.Exit(11)
	}
}

// cleanupStaleOperations marks all in-progress operations for this instance as interrupted
// This allows the system to self-heal on restart - files will be picked up on next processing cycle
func cleanupStaleOperations(cfg *config.Config) {
	if cfg.ControlService.ControlURL == "" {
		log.Debug("Control service URL not configured, skipping operation cleanup")
		return
	}

	payload := map[string]string{
		"service_type": "piper",
		"instance_id":  cfg.App.InstanceID,
	}

	body, err := sonic.Marshal(payload)
	if err != nil {
		log.Warnf("Failed to marshal cleanup request: %v", err)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", cfg.ControlService.ControlURL+"/api/v1/activity/operations/cleanup", bytes.NewBuffer(body))
	if err != nil {
		log.Warnf("Failed to create cleanup request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if cfg.ControlService.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.ControlService.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Warnf("Failed to cleanup stale operations: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var result struct {
			OperationsCleaned int `json:"operations_cleaned"`
		}
		if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&result); err == nil && result.OperationsCleaned > 0 {
			log.Infof("Cleaned up %d stale operations from previous run", result.OperationsCleaned)
		}
	} else {
		log.Warnf("Failed to cleanup stale operations: HTTP %d", resp.StatusCode)
	}
}
