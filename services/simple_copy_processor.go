package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/n0needt0/go-goodies/log"

	"github.com/n0needt0/bytefreezer-piper/config"
	"github.com/n0needt0/bytefreezer-piper/domain"
	"github.com/n0needt0/bytefreezer-piper/storage"
)

// SimpleCopyProcessor handles simple file copying from source to destination
type SimpleCopyProcessor struct {
	config       *config.Config
	s3Client     *storage.S3Client
	stateManager *storage.PostgreSQLStateManager
	processorID  string
}

// NewSimpleCopyProcessor creates a new simple copy processor
func NewSimpleCopyProcessor(cfg *config.Config, s3Client *storage.S3Client, stateManager *storage.PostgreSQLStateManager) *SimpleCopyProcessor {
	return &SimpleCopyProcessor{
		config:       cfg,
		s3Client:     s3Client,
		stateManager: stateManager,
		processorID:  cfg.App.InstanceID,
	}
}

// ProcessFile processes a single file by copying it from source to destination
func (scp *SimpleCopyProcessor) ProcessFile(ctx context.Context, job *domain.JobRecord) error {
	log.Infof("Processing file: %s (job: %s)", job.SourceFiles[0], job.JobID)

	startTime := time.Now()
	sourceKey := job.SourceFiles[0]

	// Update job status to processing
	if err := scp.stateManager.UpdateJobStatus(ctx, job.JobID, domain.JobStatusProcessing); err != nil {
		log.Errorf("Failed to update job status to processing: %v", err)
	}

	// Parse source file metadata
	metadata, err := parseS3Key(sourceKey)
	if err != nil {
		return fmt.Errorf("failed to parse S3 key %s: %w", sourceKey, err)
	}

	// Generate destination key
	destKey := scp.generateDestinationKey(sourceKey, metadata)

	// Copy file from source to destination
	if err := scp.copyFile(ctx, sourceKey, destKey); err != nil {
		// Update job status to failed
		if updateErr := scp.stateManager.UpdateJobStatus(ctx, job.JobID, domain.JobStatusFailed); updateErr != nil {
			log.Errorf("Failed to update job status to failed: %v", updateErr)
		}
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Delete source file after successful copy
	if err := scp.deleteSourceFile(ctx, sourceKey); err != nil {
		log.Errorf("Failed to delete source file %s: %v", sourceKey, err)
		// Don't fail the job for deletion errors, just log them
	}

	// Update job record with completion info
	job.OutputFiles = []string{destKey}
	job.ProcessingTime = time.Since(startTime).Milliseconds()
	job.CompletedAt = &startTime

	// Update job status to completed
	if err := scp.stateManager.UpdateJobStatus(ctx, job.JobID, domain.JobStatusCompleted); err != nil {
		log.Errorf("Failed to update job status to completed: %v", err)
	}

	log.Infof("Successfully processed file %s -> %s in %v", sourceKey, destKey, time.Since(startTime))
	return nil
}

// copyFile copies a file from source bucket to destination bucket
func (scp *SimpleCopyProcessor) copyFile(ctx context.Context, sourceKey, destKey string) error {
	log.Debugf("Copying file from %s to %s", sourceKey, destKey)

	// Download from source
	sourceData, err := scp.s3Client.GetSourceObject(ctx, sourceKey)
	if err != nil {
		return fmt.Errorf("failed to download source file: %w", err)
	}

	// Upload to destination
	if err := scp.s3Client.PutDestinationObject(ctx, destKey, sourceData); err != nil {
		return fmt.Errorf("failed to upload to destination: %w", err)
	}

	log.Debugf("Successfully copied %d bytes from %s to %s", len(sourceData), sourceKey, destKey)
	return nil
}

// deleteSourceFile deletes the source file after successful processing
func (scp *SimpleCopyProcessor) deleteSourceFile(ctx context.Context, sourceKey string) error {
	log.Debugf("Deleting source file: %s", sourceKey)

	if err := scp.s3Client.DeleteSourceObject(ctx, sourceKey); err != nil {
		return fmt.Errorf("failed to delete source file: %w", err)
	}

	log.Debugf("Successfully deleted source file: %s", sourceKey)
	return nil
}

// generateDestinationKey generates the destination key based on source key
func (scp *SimpleCopyProcessor) generateDestinationKey(sourceKey string, metadata *domain.S3FileMetadata) string {
	// Convert from raw/ to processed/
	// Example: raw/tenant=acme/dataset=logs/2024/01/15/file.ndjson.gz
	//       -> processed/tenant=acme/dataset=logs/2024/01/15/file.ndjson.gz

	destKey := sourceKey

	log.Debugf("Generated destination key: %s -> %s", sourceKey, destKey)
	return destKey
}

// parseS3Key parses an S3 key to extract metadata (supports multiple formats)
func parseS3Key(s3Key string) (*domain.S3FileMetadata, error) {
	metadata := &domain.S3FileMetadata{}

	// Check for new format: customer-1--ebpf-data--timestamp--ndjson.gz
	if strings.Contains(s3Key, "--") {
		// Handle filename format: customer-1--ebpf-data--timestamp--ndjson.gz
		lastSlash := strings.LastIndex(s3Key, "/")
		filename := s3Key
		if lastSlash != -1 {
			filename = s3Key[lastSlash+1:]
		}

		parts := strings.Split(filename, "--")
		if len(parts) >= 2 {
			metadata.TenantID = parts[0]
			metadata.DatasetID = parts[1]
			metadata.Filename = filename
		}

		// Extract path components for tenant/dataset structure
		pathParts := strings.Split(s3Key, "/")
		if len(pathParts) >= 2 {
			metadata.TenantID = pathParts[0]
			metadata.DatasetID = pathParts[1]
		}
	} else {
		// Handle traditional format: tenant=customer1/dataset=logs/file.ndjson.gz or customer-1/ebpf-data/file.ndjson.gz
		parts := strings.Split(s3Key, "/")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid S3 key format: %s", s3Key)
		}

		// Check for key=value format
		hasKeyValueFormat := false
		for _, part := range parts {
			if strings.HasPrefix(part, "tenant=") {
				metadata.TenantID = strings.TrimPrefix(part, "tenant=")
				hasKeyValueFormat = true
			} else if strings.HasPrefix(part, "dataset=") {
				metadata.DatasetID = strings.TrimPrefix(part, "dataset=")
				hasKeyValueFormat = true
			} else if strings.HasPrefix(part, "year=") {
				metadata.Year = strings.TrimPrefix(part, "year=")
			} else if strings.HasPrefix(part, "month=") {
				metadata.Month = strings.TrimPrefix(part, "month=")
			} else if strings.HasPrefix(part, "day=") {
				metadata.Day = strings.TrimPrefix(part, "day=")
			} else if strings.HasPrefix(part, "hour=") {
				metadata.Hour = strings.TrimPrefix(part, "hour=")
			}
		}

		// If no key=value format, assume simple path format: tenant/dataset/file
		if !hasKeyValueFormat && len(parts) >= 2 {
			metadata.TenantID = parts[0]
			metadata.DatasetID = parts[1]
		}

		// Extract filename from the last part
		if len(parts) > 0 {
			metadata.Filename = parts[len(parts)-1]
		}
	}

	// Validate required fields
	if metadata.TenantID == "" || metadata.DatasetID == "" {
		return nil, fmt.Errorf("missing required tenant_id or dataset_id in S3 key: %s", s3Key)
	}

	return metadata, nil
}

// isNumeric checks if a string contains only digits
func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// S3FileMetadata represents metadata extracted from S3 file paths
// (moved to domain package if not already there)
