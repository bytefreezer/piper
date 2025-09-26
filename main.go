package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/n0needt0/go-goodies/log"

	"github.com/n0needt0/bytefreezer-piper/api"
	"github.com/n0needt0/bytefreezer-piper/config"
	"github.com/n0needt0/bytefreezer-piper/services"
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

	// Initialize logger
	log.Infof("Starting bytefreezer-piper version %s", version)

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Infof("Configuration loaded successfully")
	log.Infof("Instance ID: %s", cfg.App.InstanceID)

	sourcePrefix := cfg.S3Source.Prefix
	if sourcePrefix == "" {
		sourcePrefix = "(none)"
	}
	destPrefix := cfg.S3Dest.Prefix
	if destPrefix == "" {
		destPrefix = "(none)"
	}

	log.Infof("Source bucket: %s (prefix: %s)", cfg.S3Source.BucketName, sourcePrefix)
	log.Infof("Destination bucket: %s (prefix: %s)", cfg.S3Dest.BucketName, destPrefix)

	// Business Logic - Initialize services
	servicesInstance := services.NewServices(cfg)

	// Create server
	server := NewServer(servicesInstance, cfg)

	// Create API server
	server.HttpApi = api.NewAPI(servicesInstance, cfg)

	// Define housekeeping function for pipeline database updates
	housekeepingFn := func() {
		log.Debug("Starting housekeeping cycle...")

		// Update pipeline database during housekeeping
		log.Debug("Updating pipeline database during housekeeping...")
		if err := servicesInstance.PipelineDatabase.UpdateDatabase(server.ctx); err != nil {
			log.Errorf("Failed to update pipeline database during housekeeping: %v", err)
			return
		}

		log.Debug("Pipeline database updated successfully")
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

			if quitterFn != nil {
				quitterFn(timeout)
			}

			svc.HttpApi.Stop()
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
