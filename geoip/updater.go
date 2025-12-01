package geoip

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/bytefreezer/goodies/log"
	"github.com/bytefreezer/piper/config"
)

// Updater handles automatic updates of GeoIP database files from S3
type Updater struct {
	config    *config.Config
	s3Client  *s3.S3
	localPath string
}

// NewUpdater creates a new GeoIP database updater
func NewUpdater(cfg *config.Config) (*Updater, error) {
	// Create S3 session for GeoIP bucket
	s3Config := &aws.Config{
		Region: aws.String(cfg.S3GeoIP.Region),
	}

	if cfg.S3GeoIP.Endpoint != "" {
		s3Config.Endpoint = aws.String(fmt.Sprintf("http%s://%s",
			map[bool]string{true: "s", false: ""}[cfg.S3GeoIP.SSL],
			cfg.S3GeoIP.Endpoint))
		s3Config.S3ForcePathStyle = aws.Bool(true)
	}

	if !cfg.S3GeoIP.UseIamRole {
		// Use static credentials (access key + secret key)
		s3Config.Credentials = credentials.NewStaticCredentials(
			cfg.S3GeoIP.AccessKey,
			cfg.S3GeoIP.SecretKey,
			"")
	}
	// If UseIamRole is true, credentials will use default credential chain (IAM role)

	sess, err := session.NewSession(s3Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 session for GeoIP: %w", err)
	}

	return &Updater{
		config:    cfg,
		s3Client:  s3.New(sess),
		localPath: cfg.Pipeline.GeoIPDatabasePath,
	}, nil
}

// CheckAndUpdate checks for new versions of GeoIP databases and updates them if needed
func (u *Updater) CheckAndUpdate(ctx context.Context) error {
	log.Infof("Checking for GeoIP database updates...")

	// Ensure local directory exists
	if err := os.MkdirAll(u.localPath, 0750); err != nil {
		return fmt.Errorf("failed to create GeoIP directory %s: %w", u.localPath, err)
	}

	// Update both city and country databases
	databases := []string{
		u.config.Pipeline.GeoIPCityDatabase,
		u.config.Pipeline.GeoIPCountryDatabase,
	}

	for _, dbFile := range databases {
		if err := u.updateDatabase(ctx, dbFile); err != nil {
			log.Errorf("Failed to update %s: %v", dbFile, err)
			continue
		}
	}

	log.Infof("GeoIP database update check completed")
	return nil
}

// updateDatabase updates a specific database file if a newer version is available
func (u *Updater) updateDatabase(ctx context.Context, dbFile string) error {
	// Validate dbFile path to prevent directory traversal
	if strings.Contains(dbFile, "..") || strings.Contains(dbFile, "/") || strings.Contains(dbFile, "\\") {
		return fmt.Errorf("invalid database file path: %s", dbFile)
	}

	localFile := filepath.Join(u.localPath, dbFile)
	tempFile := localFile + ".tmp"

	// Get remote object metadata
	headInput := &s3.HeadObjectInput{
		Bucket: aws.String(u.config.S3GeoIP.BucketName),
		Key:    aws.String(dbFile),
	}

	remoteInfo, err := u.s3Client.HeadObjectWithContext(ctx, headInput)
	if err != nil {
		return fmt.Errorf("failed to get remote file info for %s: %w", dbFile, err)
	}

	// Check if local file exists and compare modification times
	if localInfo, err := os.Stat(localFile); err == nil {
		localModTime := localInfo.ModTime()
		remoteModTime := *remoteInfo.LastModified

		if !remoteModTime.After(localModTime) {
			log.Debugf("Local %s is up to date", dbFile)
			return nil
		}
		log.Infof("Remote %s is newer, updating...", dbFile)
	} else {
		log.Infof("Local %s not found, downloading...", dbFile)
	}

	// Download to temporary file first (atomic update)
	getInput := &s3.GetObjectInput{
		Bucket: aws.String(u.config.S3GeoIP.BucketName),
		Key:    aws.String(dbFile),
	}

	result, err := u.s3Client.GetObjectWithContext(ctx, getInput)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", dbFile, err)
	}
	defer result.Body.Close()

	// Create temporary file with secure permissions
	// #nosec G304 - tempFile path is validated above to prevent directory traversal
	tempFileHandle, err := os.OpenFile(tempFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create temp file %s: %w", tempFile, err)
	}

	// Copy data and calculate checksum
	hash := sha256.New()
	writer := io.MultiWriter(tempFileHandle, hash)

	bytesWritten, err := io.Copy(writer, result.Body)
	tempFileHandle.Close()

	if err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to write %s: %w", tempFile, err)
	}

	// Verify size matches
	if result.ContentLength != nil && bytesWritten != *result.ContentLength {
		os.Remove(tempFile)
		return fmt.Errorf("size mismatch for %s: expected %d, got %d",
			dbFile, *result.ContentLength, bytesWritten)
	}

	// Set modification time to match remote
	if err := os.Chtimes(tempFile, time.Now(), *remoteInfo.LastModified); err != nil {
		log.Warnf("Failed to set modification time for %s: %v", tempFile, err)
	}

	// Atomic move (rename) to final location
	if err := os.Rename(tempFile, localFile); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to move temp file to final location %s: %w", localFile, err)
	}

	checksum := fmt.Sprintf("%x", hash.Sum(nil))
	log.Infof("Successfully updated %s (size: %d bytes, checksum: %s)",
		dbFile, bytesWritten, checksum[:8])

	return nil
}

// VerifyDatabases checks that the required GeoIP database files exist and are readable
func (u *Updater) VerifyDatabases() error {
	databases := []string{
		u.config.Pipeline.GeoIPCityDatabase,
		u.config.Pipeline.GeoIPCountryDatabase,
	}

	var missing []string
	for _, dbFile := range databases {
		localFile := filepath.Join(u.localPath, dbFile)
		if _, err := os.Stat(localFile); os.IsNotExist(err) {
			missing = append(missing, dbFile)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing GeoIP database files: %s", strings.Join(missing, ", "))
	}

	log.Debugf("All required GeoIP database files are present")
	return nil
}
