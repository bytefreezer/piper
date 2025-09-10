package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/n0needt0/go-goodies/log"

	"github.com/n0needt0/bytefreezer-piper/config"
	"github.com/n0needt0/bytefreezer-piper/services"
)

var (
	configPath = flag.String("config", "config.yaml", "Path to configuration file")
	version    = "1.0.0" // Will be set during build
)

func main() {
	flag.Parse()

	// Initialize logger
	log.Infof("Starting bytefreezer-piper version %s", version)

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Infof("Configuration loaded successfully")
	log.Infof("Instance ID: %s", cfg.App.InstanceID)
	log.Infof("Source bucket: %s (prefix: %s)", cfg.S3Source.BucketName, cfg.S3Source.Prefix)
	log.Infof("Destination bucket: %s (prefix: %s)", cfg.S3Dest.BucketName, cfg.S3Dest.Prefix)

	// Create main service
	piperService, err := services.NewPiperService(cfg)
	if err != nil {
		log.Fatalf("Failed to create piper service: %v", err)
	}

	// Create context that cancels on interrupt
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start the service
	log.Infof("Starting piper service...")
	if err := piperService.Start(ctx); err != nil {
		log.Fatalf("Failed to start piper service: %v", err)
	}

	// Wait for interrupt signal
	<-ctx.Done()
	log.Infof("Received shutdown signal, stopping service...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop the service gracefully
	if err := piperService.Stop(shutdownCtx); err != nil {
		log.Errorf("Error during service shutdown: %v", err)
		os.Exit(1)
	}

	log.Infof("Service stopped gracefully")
}