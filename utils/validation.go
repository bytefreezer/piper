package utils

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateFilePath validates that a file path is safe and within allowed directory
// Following the receiver pattern for security consistency
func ValidateFilePath(filePath, allowedBasePath string) error {
	cleanPath := filepath.Clean(filePath)
	cleanBasePath := filepath.Clean(allowedBasePath)

	// Prevent directory traversal
	if !strings.HasPrefix(cleanPath, cleanBasePath) {
		return fmt.Errorf("file path %s is outside allowed directory %s", filePath, allowedBasePath)
	}

	// Check for malicious characters
	if strings.ContainsAny(filePath, "\x00\r\n") {
		return fmt.Errorf("invalid characters in file path: %s", filePath)
	}

	return nil
}

// IsMalformedFilename checks if a filename contains malicious patterns
// Following the receiver pattern for security consistency
func IsMalformedFilename(filename string) bool {
	// Check for double dots (directory traversal)
	if strings.Contains(filename, "..") {
		return true
	}

	// Check for null bytes and control characters
	if strings.ContainsAny(filename, "\x00\r\n") {
		return true
	}

	// Check for absolute paths
	if strings.HasPrefix(filename, "/") || strings.HasPrefix(filename, "\\") {
		return true
	}

	// Check for Windows drive paths
	if len(filename) >= 2 && filename[1] == ':' {
		return true
	}

	return false
}

// ParseTenantDatasetFromKey extracts tenant and dataset from S3 key
// Expected format: tenant/dataset/filename
func ParseTenantDatasetFromKey(key string) (tenant, dataset, filename string, err error) {
	parts := strings.SplitN(key, "/", 3)
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid S3 key format: %s (expected: tenant/dataset/filename)", key)
	}

	tenant = parts[0]
	dataset = parts[1]
	filename = parts[2]

	// Validate components are not empty
	if tenant == "" || dataset == "" || filename == "" {
		return "", "", "", fmt.Errorf("empty tenant, dataset, or filename in key: %s", key)
	}

	// Check for malformed filename
	if IsMalformedFilename(filename) {
		return "", "", "", fmt.Errorf("malformed filename in key: %s", key)
	}

	return tenant, dataset, filename, nil
}

// BuildSpoolPath builds a spool directory path for a tenant/dataset
func BuildSpoolPath(basePath, tenant, dataset string, stage string) string {
	return filepath.Join(basePath, tenant, dataset, stage)
}

// BuildMalformedPath builds a path for quarantining malformed files
func BuildMalformedPath(basePath string) string {
	return filepath.Join(basePath, "malformed_local")
}