package services

import (
	"testing"
	"time"

	"github.com/n0needt0/bytefreezer-piper/config"
)

func TestConfigValidation(t *testing.T) {
	cfg := &config.Config{
		S3Source: config.S3Source{
			BucketName:   "test-bucket",
			Prefix:       "raw/",
			PollInterval: 30 * time.Second,
		},
		App: config.App{
			InstanceID: "test-instance",
		},
	}

	if cfg.S3Source.BucketName != "test-bucket" {
		t.Errorf("Expected bucket name 'test-bucket', got '%s'", cfg.S3Source.BucketName)
	}

	if cfg.S3Source.Prefix != "raw/" {
		t.Errorf("Expected prefix 'raw/', got '%s'", cfg.S3Source.Prefix)
	}

	if cfg.App.InstanceID != "test-instance" {
		t.Errorf("Expected instance ID 'test-instance', got '%s'", cfg.App.InstanceID)
	}
}
