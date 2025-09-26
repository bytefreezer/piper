package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/n0needt0/bytefreezer-piper/domain"
	"github.com/n0needt0/bytefreezer-piper/utils"
	"github.com/n0needt0/go-goodies/log"
	"github.com/swaggest/usecase"
	"github.com/swaggest/usecase/status"
)

// DLQStatsResponse represents DLQ statistics
type DLQStatsResponse struct {
	TotalFiles      int                    `json:"total_files"`
	QueueFiles      int                    `json:"queue_files"`
	RetryFiles      int                    `json:"retry_files"`
	DLQFiles        int                    `json:"dlq_files"`
	ProcessingFiles int                    `json:"processing_files"`
	ByTenant        map[string]DLQStats    `json:"by_tenant"`
	LastUpdated     time.Time              `json:"last_updated"`
}

// DLQStats represents per-tenant DLQ statistics
type DLQStats struct {
	TotalFiles int `json:"total_files"`
	QueueFiles int `json:"queue_files"`
	RetryFiles int `json:"retry_files"`
	DLQFiles   int `json:"dlq_files"`
}

// DLQFilesRequest represents request for listing DLQ files
type DLQFilesRequest struct {
	TenantID  string `query:"tenant_id" description:"Filter by tenant ID"`
	DatasetID string `query:"dataset_id" description:"Filter by dataset ID"`
	Stage     string `query:"stage" description:"Filter by stage (queue, retry, dlq)"`
	Limit     int    `query:"limit" description:"Maximum number of files to return (default: 100)"`
	Offset    int    `query:"offset" description:"Number of files to skip (default: 0)"`
}

// DLQFilesResponse represents response for listing DLQ files
type DLQFilesResponse struct {
	Files  []DLQFileInfo `json:"files"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

// DLQFileInfo represents information about a file in DLQ system
type DLQFileInfo struct {
	TenantID         string    `json:"tenant_id"`
	DatasetID        string    `json:"dataset_id"`
	Filename         string    `json:"filename"`
	Stage            string    `json:"stage"`
	FilePath         string    `json:"file_path"`
	CompressedSize   int64     `json:"compressed_size"`
	UncompressedSize int64     `json:"uncompressed_size"`
	LineCount        int64     `json:"line_count"`
	CreatedAt        time.Time `json:"created_at"`
	Status           string    `json:"status"`
	RetryCount       int       `json:"retry_count"`
	LastRetry        time.Time `json:"last_retry"`
	FailureReason    string    `json:"failure_reason,omitempty"`
	ProcessedAt      time.Time `json:"processed_at,omitempty"`
}

// RetryFileRequest represents request for retrying a specific file
type RetryFileRequest struct {
	TenantID  string `path:"tenantId" description:"Tenant ID"`
	DatasetID string `path:"datasetId" description:"Dataset ID"`
	Filename  string `path:"filename" description:"Filename"`
}

// RetryFileResponse represents response for retrying a file
type RetryFileResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// GetDLQStats returns a handler for getting DLQ statistics
func (api *API) GetDLQStats() usecase.Interactor {
	u := usecase.NewInteractor(func(ctx context.Context, input struct{}, output *DLQStatsResponse) error {
		if api.SpoolingService == nil {
			return status.Wrap(fmt.Errorf("spooling service not available"), status.Internal)
		}

		stats, err := api.collectDLQStats()
		if err != nil {
			log.Errorf("Failed to collect DLQ stats: %v", err)
			return status.Wrap(err, status.Internal)
		}

		*output = *stats
		return nil
	})

	u.SetTitle("Get DLQ Statistics")
	u.SetDescription("Retrieve statistics about files in the DLQ system (queue, retry, dlq stages)")
	u.SetTags("DLQ")

	return u
}

// GetDLQFiles returns a handler for listing DLQ files
func (api *API) GetDLQFiles() usecase.Interactor {
	u := usecase.NewInteractor(func(ctx context.Context, input DLQFilesRequest, output *DLQFilesResponse) error {
		if api.SpoolingService == nil {
			return status.Wrap(fmt.Errorf("spooling service not available"), status.Internal)
		}

		// Set defaults
		if input.Limit <= 0 || input.Limit > 1000 {
			input.Limit = 100
		}
		if input.Offset < 0 {
			input.Offset = 0
		}

		files, total, err := api.listDLQFiles(input)
		if err != nil {
			log.Errorf("Failed to list DLQ files: %v", err)
			return status.Wrap(err, status.Internal)
		}

		output.Files = files
		output.Total = total
		output.Limit = input.Limit
		output.Offset = input.Offset

		return nil
	})

	u.SetTitle("List DLQ Files")
	u.SetDescription("List files in the DLQ system with optional filtering")
	u.SetTags("DLQ")

	return u
}

// RetryDLQFile returns a handler for retrying a specific file
func (api *API) RetryDLQFile() usecase.Interactor {
	u := usecase.NewInteractor(func(ctx context.Context, input RetryFileRequest, output *RetryFileResponse) error {
		if api.SpoolingService == nil {
			return status.Wrap(fmt.Errorf("spooling service not available"), status.Internal)
		}

		err := api.retryFile(input.TenantID, input.DatasetID, input.Filename)
		if err != nil {
			log.Errorf("Failed to retry file %s/%s/%s: %v", input.TenantID, input.DatasetID, input.Filename, err)
			output.Success = false
			output.Message = fmt.Sprintf("Failed to retry file: %v", err)
			return nil
		}

		output.Success = true
		output.Message = "File successfully moved to retry queue"

		log.Infof("File retry initiated: %s/%s/%s", input.TenantID, input.DatasetID, input.Filename)
		return nil
	})

	u.SetTitle("Retry DLQ File")
	u.SetDescription("Move a file from DLQ back to retry queue for reprocessing")
	u.SetTags("DLQ")

	return u
}

// validateSpoolPath ensures the path is safe and within expected boundaries
func (api *API) validateSpoolPath(filePath, baseDir string) error {
	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(filePath)

	// Ensure the path is absolute
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Ensure base directory is absolute
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute base directory: %w", err)
	}

	// Check if the file path is within the base directory
	relPath, err := filepath.Rel(absBaseDir, absPath)
	if err != nil {
		return fmt.Errorf("invalid path relationship: %w", err)
	}

	// Ensure the relative path doesn't start with .. (escaping base directory)
	if strings.HasPrefix(relPath, "..") || strings.Contains(relPath, string(filepath.Separator)+"..") {
		return fmt.Errorf("path traversal attempt detected: %s", filePath)
	}

	return nil
}

// collectDLQStats collects statistics from the spool directories
func (api *API) collectDLQStats() (*DLQStatsResponse, error) {
	spoolPath := api.Config.Spooling.SpoolPath
	if spoolPath == "" {
		spoolPath = "/var/spool/bytefreezer-piper"
	}

	stats := &DLQStatsResponse{
		ByTenant:    make(map[string]DLQStats),
		LastUpdated: time.Now().UTC(),
	}

	err := filepath.Walk(spoolPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and hidden files
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Skip metadata files and malformed files
		if strings.HasSuffix(path, ".meta") || strings.Contains(path, "malformed_local") {
			return nil
		}

		// Extract tenant/dataset/stage from path
		relPath, err := filepath.Rel(spoolPath, path)
		if err != nil {
			return err
		}

		pathParts := strings.Split(relPath, string(filepath.Separator))
		if len(pathParts) < 3 {
			return nil
		}

		tenant := pathParts[0]
		_ = pathParts[1] // dataset - not used in stats collection
		stage := pathParts[2]

		// Update overall stats
		stats.TotalFiles++
		switch stage {
		case "queue":
			stats.QueueFiles++
		case "retry":
			stats.RetryFiles++
		case "dlq":
			stats.DLQFiles++
		}

		// Update per-tenant stats
		if _, exists := stats.ByTenant[tenant]; !exists {
			stats.ByTenant[tenant] = DLQStats{}
		}

		tenantStats := stats.ByTenant[tenant]
		tenantStats.TotalFiles++
		switch stage {
		case "queue":
			tenantStats.QueueFiles++
		case "retry":
			tenantStats.RetryFiles++
		case "dlq":
			tenantStats.DLQFiles++
		}
		stats.ByTenant[tenant] = tenantStats

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk spool directories: %w", err)
	}

	return stats, nil
}

// listDLQFiles lists files in the DLQ system with filtering
func (api *API) listDLQFiles(req DLQFilesRequest) ([]DLQFileInfo, int, error) {
	spoolPath := api.Config.Spooling.SpoolPath
	if spoolPath == "" {
		spoolPath = "/var/spool/bytefreezer-piper"
	}

	var allFiles []DLQFileInfo

	err := filepath.Walk(spoolPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and hidden files
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Skip metadata files and malformed files
		if strings.HasSuffix(path, ".meta") || strings.Contains(path, "malformed_local") {
			return nil
		}

		// Extract tenant/dataset/stage from path
		relPath, err := filepath.Rel(spoolPath, path)
		if err != nil {
			return err
		}

		pathParts := strings.Split(relPath, string(filepath.Separator))
		if len(pathParts) < 3 {
			return nil
		}

		tenant := pathParts[0]
		dataset := pathParts[1]
		stage := pathParts[2]
		filename := info.Name()

		// Apply filters
		if req.TenantID != "" && tenant != req.TenantID {
			return nil
		}
		if req.DatasetID != "" && dataset != req.DatasetID {
			return nil
		}
		if req.Stage != "" && stage != req.Stage {
			return nil
		}

		// Create file info
		fileInfo := DLQFileInfo{
			TenantID:       tenant,
			DatasetID:      dataset,
			Filename:       filename,
			Stage:          stage,
			FilePath:       path,
			CompressedSize: info.Size(),
			CreatedAt:      info.ModTime(),
			Status:         stage,
		}

		// Try to read metadata if it exists
		metaPath := path + ".meta"

		// Validate the metadata path for security
		if err := api.validateSpoolPath(metaPath, spoolPath); err != nil {
			log.Warnf("Invalid metadata path detected: %s", metaPath)
			return nil
		}

		// #nosec G304 - Path is validated by validateSpoolPath function above
		if metaData, err := os.ReadFile(metaPath); err == nil {
			var retryFile domain.RetryFile
			if err := json.Unmarshal(metaData, &retryFile); err == nil {
				fileInfo.UncompressedSize = retryFile.UncompressedSize
				fileInfo.LineCount = retryFile.LineCount
				fileInfo.RetryCount = retryFile.RetryCount
				fileInfo.LastRetry = retryFile.LastRetry
				fileInfo.FailureReason = retryFile.FailureReason
				fileInfo.ProcessedAt = retryFile.ProcessedAt
				if !retryFile.CreatedAt.IsZero() {
					fileInfo.CreatedAt = retryFile.CreatedAt
				}
			}
		}

		allFiles = append(allFiles, fileInfo)
		return nil
	})

	if err != nil {
		return nil, 0, fmt.Errorf("failed to walk spool directories: %w", err)
	}

	// Apply pagination
	total := len(allFiles)
	start := req.Offset
	end := start + req.Limit

	if start > total {
		return []DLQFileInfo{}, total, nil
	}
	if end > total {
		end = total
	}

	return allFiles[start:end], total, nil
}

// retryFile moves a file from DLQ back to retry queue
func (api *API) retryFile(tenantID, datasetID, filename string) error {
	spoolPath := api.Config.Spooling.SpoolPath
	if spoolPath == "" {
		spoolPath = "/var/spool/bytefreezer-piper"
	}

	// Validate inputs
	if utils.IsMalformedFilename(filename) {
		return fmt.Errorf("malformed filename: %s", filename)
	}

	// Build paths
	dlqPath := utils.BuildSpoolPath(spoolPath, tenantID, datasetID, domain.StageDLQ.GetStageDirectory())
	retryPath := utils.BuildSpoolPath(spoolPath, tenantID, datasetID, domain.StageRetry.GetStageDirectory())

	dlqFilePath := filepath.Join(dlqPath, filename)
	dlqMetaPath := dlqFilePath + ".meta"
	retryFilePath := filepath.Join(retryPath, filename)
	retryMetaPath := retryFilePath + ".meta"

	// Check if file exists in DLQ
	if _, err := os.Stat(dlqFilePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found in DLQ: %s", filename)
	}

	// Create retry directory
	if err := os.MkdirAll(retryPath, 0750); err != nil {
		return fmt.Errorf("failed to create retry directory: %w", err)
	}

	// Check if file already exists in retry
	if _, err := os.Stat(retryFilePath); err == nil {
		return fmt.Errorf("file already exists in retry queue: %s", filename)
	}

	// Read and update metadata
	var retryFile domain.RetryFile

	// Validate the metadata path for security
	if err := api.validateSpoolPath(dlqMetaPath, spoolPath); err != nil {
		return fmt.Errorf("invalid metadata path: %w", err)
	}

	// #nosec G304 - Path is validated by validateSpoolPath function above
	if metaData, err := os.ReadFile(dlqMetaPath); err == nil {
		if err := json.Unmarshal(metaData, &retryFile); err != nil {
			return fmt.Errorf("failed to parse metadata: %w", err)
		}
	} else {
		// Create new metadata if it doesn't exist
		fileInfo, err := os.Stat(dlqFilePath)
		if err != nil {
			return fmt.Errorf("failed to get file info: %w", err)
		}

		retryFile = domain.RetryFile{
			TenantID:       tenantID,
			DatasetID:      datasetID,
			Filename:       filename,
			CompressedSize: fileInfo.Size(),
			CreatedAt:      time.Now().UTC(),
		}
	}

	// Reset retry metadata
	retryFile.Status = "retry"
	retryFile.RetryCount = 0
	retryFile.LastRetry = time.Time{}
	retryFile.FailureReason = ""
	retryFile.FilePath = retryFilePath

	// Move file atomically
	if err := os.Rename(dlqFilePath, retryFilePath); err != nil {
		return fmt.Errorf("failed to move file to retry: %w", err)
	}

	// Write updated metadata
	updatedMeta, err := json.Marshal(retryFile)
	if err != nil {
		// Rollback file move
		os.Rename(retryFilePath, dlqFilePath)
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(retryMetaPath, updatedMeta, 0600); err != nil {
		// Rollback file move
		os.Rename(retryFilePath, dlqFilePath)
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Remove old DLQ metadata
	os.Remove(dlqMetaPath)

	log.Infof("File moved from DLQ to retry: %s/%s/%s", tenantID, datasetID, filename)
	return nil
}