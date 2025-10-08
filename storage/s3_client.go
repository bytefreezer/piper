package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/n0needt0/go-goodies/log"

	"github.com/n0needt0/bytefreezer-piper/config"
	"github.com/n0needt0/bytefreezer-piper/domain"
)

// S3Client provides S3 operations for both source and destination
type S3Client struct {
	sourceClient *s3.Client
	destClient   *s3.Client
	sourceBucket string
	destBucket   string
	sourcePrefix string
	destPrefix   string
}

// NewS3Client creates a new S3 client for source and destination operations
func NewS3Client(sourceConfig *config.S3Source, destConfig *config.S3Dest) (*S3Client, error) {
	// Create source client
	sourceClient, err := createS3Client(
		sourceConfig.Region,
		sourceConfig.AccessKey,
		sourceConfig.SecretKey,
		sourceConfig.Endpoint,
		sourceConfig.SSL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create source S3 client: %w", err)
	}

	// Create destination client
	destClient, err := createS3Client(
		destConfig.Region,
		destConfig.AccessKey,
		destConfig.SecretKey,
		destConfig.Endpoint,
		destConfig.SSL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination S3 client: %w", err)
	}

	return &S3Client{
		sourceClient: sourceClient,
		destClient:   destClient,
		sourceBucket: sourceConfig.BucketName,
		destBucket:   destConfig.BucketName,
		sourcePrefix: "",
		destPrefix:   "",
	}, nil
}

// createS3Client creates an S3 client with the given configuration
func createS3Client(region, accessKey, secretKey, endpoint string, useSSL bool) (*s3.Client, error) {
	var awsCfg aws.Config
	var err error

	if accessKey != "" && secretKey != "" {
		// Use static credentials
		awsCfg, err = awsconfig.LoadDefaultConfig(context.Background(),
			awsconfig.WithRegion(region),
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				accessKey,
				secretKey,
				"",
			)),
		)
	} else {
		// Use default credential chain
		awsCfg, err = awsconfig.LoadDefaultConfig(context.Background(),
			awsconfig.WithRegion(region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	var client *s3.Client
	if endpoint != "" {
		// Custom endpoint (MinIO, LocalStack, etc.)
		endpointURL := endpoint
		if !strings.HasPrefix(endpointURL, "http") {
			if useSSL {
				endpointURL = "https://" + endpointURL
			} else {
				endpointURL = "http://" + endpointURL
			}
		}

		client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpointURL)
			o.UsePathStyle = true
			// Disable checksum validation for MinIO compatibility
			o.DisableS3ExpressSessionAuth = aws.Bool(true)
		})
	} else {
		// Standard AWS S3
		client = s3.NewFromConfig(awsCfg)
	}

	return client, nil
}

// ListRawFiles lists raw data files in the source bucket that need processing
func (sc *S3Client) ListRawFiles(ctx context.Context, prefix string, maxKeys int32) ([]domain.S3Object, error) {
	if maxKeys <= 0 {
		maxKeys = 1000
	}

	fullPrefix := sc.sourcePrefix
	if prefix != "" {
		fullPrefix = sc.sourcePrefix + prefix
	}

	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(sc.sourceBucket),
		Prefix:  aws.String(fullPrefix),
		MaxKeys: &maxKeys,
	}

	result, err := sc.sourceClient.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects in source bucket %s: %w", sc.sourceBucket, err)
	}

	objects := make([]domain.S3Object, 0, len(result.Contents))
	for _, obj := range result.Contents {
		if obj.Key == nil {
			continue
		}

		// Filter for .ndjson.gz files only
		if !strings.HasSuffix(strings.ToLower(*obj.Key), ".ndjson.gz") {
			continue
		}

		s3Obj := domain.S3Object{
			Key: *obj.Key,
		}

		if obj.Size != nil {
			s3Obj.Size = *obj.Size
		}

		if obj.LastModified != nil {
			s3Obj.LastModified = *obj.LastModified
		}

		if obj.ETag != nil {
			s3Obj.ETag = *obj.ETag
		}

		objects = append(objects, s3Obj)
	}

	log.Debugf("Listed %d raw files from S3 source bucket %s with prefix '%s'", len(objects), sc.sourceBucket, fullPrefix)
	return objects, nil
}

// GetRawFile downloads a raw file from the source bucket
func (sc *S3Client) GetRawFile(ctx context.Context, key string) (io.ReadCloser, error) {
	if key == "" {
		return nil, fmt.Errorf("object key cannot be empty")
	}

	log.Debugf("Getting raw file %s from source bucket %s", key, sc.sourceBucket)

	result, err := sc.sourceClient.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(sc.sourceBucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get raw file %s from bucket %s: %w", key, sc.sourceBucket, err)
	}

	return result.Body, nil
}

// UploadProcessedFile uploads a processed file to the destination bucket
func (sc *S3Client) UploadProcessedFile(ctx context.Context, key string, body io.Reader, metadata map[string]string) error {
	if key == "" {
		return fmt.Errorf("object key cannot be empty")
	}

	// Ensure the key has the correct prefix
	fullKey := key
	if !strings.HasPrefix(key, sc.destPrefix) {
		fullKey = sc.destPrefix + key
	}

	log.Debugf("Uploading processed file %s to destination bucket %s", fullKey, sc.destBucket)

	// Add default metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["processed-at"] = time.Now().Format(time.RFC3339)
	metadata["processor-type"] = "bytefreezer-piper"

	_, err := sc.destClient.PutObject(ctx, &s3.PutObjectInput{
		Bucket:   aws.String(sc.destBucket),
		Key:      aws.String(fullKey),
		Body:     body,
		Metadata: metadata,
	})

	if err != nil {
		return fmt.Errorf("failed to upload processed file %s to bucket %s: %w", fullKey, sc.destBucket, err)
	}

	log.Debugf("Successfully uploaded processed file %s to destination bucket %s with %d metadata fields", fullKey, sc.destBucket, len(metadata))
	return nil
}

// TestSourceConnection tests the source S3 connection
func (sc *S3Client) TestSourceConnection(ctx context.Context) error {
	log.Debugf("Testing source S3 connection to bucket: %s", sc.sourceBucket)

	_, err := sc.sourceClient.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(sc.sourceBucket),
	})

	if err != nil {
		return fmt.Errorf("failed to access source S3 bucket %s: %w", sc.sourceBucket, err)
	}

	log.Infof("Source S3 connection successful to bucket: %s", sc.sourceBucket)
	return nil
}

// TestDestinationConnection tests the destination S3 connection
func (sc *S3Client) TestDestinationConnection(ctx context.Context) error {
	log.Debugf("Testing destination S3 connection to bucket: %s", sc.destBucket)

	_, err := sc.destClient.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(sc.destBucket),
	})

	if err != nil {
		return fmt.Errorf("failed to access destination S3 bucket %s: %w", sc.destBucket, err)
	}

	log.Infof("Destination S3 connection successful to bucket: %s", sc.destBucket)
	return nil
}

// GetSourceBucketInfo returns information about the source bucket
func (sc *S3Client) GetSourceBucketInfo() (bucket, prefix string) {
	return sc.sourceBucket, sc.sourcePrefix
}

// GetDestinationBucketInfo returns information about the destination bucket
func (sc *S3Client) GetDestinationBucketInfo() (bucket, prefix string) {
	return sc.destBucket, sc.destPrefix
}

// ParseS3Path parses an S3 object key to extract tenant and dataset information
func ParseS3Path(key string) (tenantID, datasetID string, err error) {
	// Expected format: raw/tenant=TENANT_ID/dataset=DATASET_ID/year=YYYY/month=MM/day=DD/hour=HH/filename.ndjson.gz
	// or processed/tenant=TENANT_ID/dataset=DATASET_ID/year=YYYY/month=MM/day=DD/hour=HH/filename-processed.ndjson.gz

	parts := strings.Split(key, "/")
	if len(parts) < 4 {
		return "", "", fmt.Errorf("invalid S3 path format: %s", key)
	}

	// Find tenant and dataset parts
	for _, part := range parts {
		if strings.HasPrefix(part, "tenant=") {
			tenantID = strings.TrimPrefix(part, "tenant=")
		} else if strings.HasPrefix(part, "dataset=") {
			datasetID = strings.TrimPrefix(part, "dataset=")
		}
	}

	if tenantID == "" {
		return "", "", fmt.Errorf("tenant ID not found in S3 path: %s", key)
	}

	if datasetID == "" {
		return "", "", fmt.Errorf("dataset ID not found in S3 path: %s", key)
	}

	return tenantID, datasetID, nil
}

// GenerateProcessedFileName generates the processed file name from the raw file name
func GenerateProcessedFileName(rawFileName string) string {
	// Remove path prefix and add -processed suffix before .ndjson.gz
	fileName := rawFileName
	if lastSlash := strings.LastIndex(fileName, "/"); lastSlash != -1 {
		fileName = fileName[lastSlash+1:]
	}

	if strings.HasSuffix(fileName, ".ndjson.gz") {
		return strings.TrimSuffix(fileName, ".ndjson.gz") + "-processed.ndjson.gz"
	}

	return fileName + "-processed"
}

// GetSourceObject downloads an object from the source bucket
func (sc *S3Client) GetSourceObject(ctx context.Context, key string) ([]byte, error) {
	log.Debugf("Downloading object from source bucket: %s", key)

	output, err := sc.sourceClient.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(sc.sourceBucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object %s from source bucket %s: %w", key, sc.sourceBucket, err)
	}
	defer output.Body.Close()

	data, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object body: %w", err)
	}

	log.Debugf("Successfully downloaded %d bytes from %s", len(data), key)
	return data, nil
}

// GetSourceObjectStream returns a streaming reader for the source object (memory efficient)
func (sc *S3Client) GetSourceObjectStream(ctx context.Context, key string) (io.ReadCloser, error) {
	log.Debugf("Streaming object from source bucket: %s", key)

	output, err := sc.sourceClient.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(sc.sourceBucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object %s from source bucket %s: %w", key, sc.sourceBucket, err)
	}

	log.Debugf("Successfully opened stream for %s", key)
	return output.Body, nil
}

// PutDestinationObject uploads data to the destination bucket
func (sc *S3Client) PutDestinationObject(ctx context.Context, key string, data []byte) error {
	log.Debugf("Uploading object to destination bucket: %s", key)

	_, err := sc.destClient.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(sc.destBucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(string(data)),
		Metadata: map[string]string{
			"processed-at":   time.Now().Format(time.RFC3339),
			"processor-type": "bytefreezer-piper",
		},
	})

	if err != nil {
		return fmt.Errorf("failed to put object %s to destination bucket %s: %w", key, sc.destBucket, err)
	}

	log.Debugf("Successfully uploaded %d bytes to %s", len(data), key)
	return nil
}

// GetSourceObjectMetadata retrieves metadata from a source object
func (sc *S3Client) GetSourceObjectMetadata(ctx context.Context, key string) (map[string]string, error) {
	log.Debugf("Getting metadata for source object: %s", key)

	result, err := sc.sourceClient.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(sc.sourceBucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata for object %s from source bucket %s: %w", key, sc.sourceBucket, err)
	}

	// Convert AWS metadata to map[string]string
	metadata := make(map[string]string)
	for key, value := range result.Metadata {
		metadata[key] = value
	}

	// Add standard object attributes as metadata
	if result.ContentLength != nil {
		metadata["source-content-length"] = fmt.Sprintf("%d", *result.ContentLength)
	}
	if result.ContentType != nil {
		metadata["source-content-type"] = *result.ContentType
	}
	if result.ETag != nil {
		metadata["source-etag"] = *result.ETag
	}
	if result.LastModified != nil {
		metadata["source-last-modified"] = result.LastModified.Format(time.RFC3339)
	}

	log.Debugf("Retrieved %d metadata fields for source object: %s", len(metadata), key)
	return metadata, nil
}

// DeleteSourceObject deletes an object from the source bucket
func (sc *S3Client) DeleteSourceObject(ctx context.Context, key string) error {
	log.Debugf("Deleting object from source bucket: %s", key)

	_, err := sc.sourceClient.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(sc.sourceBucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete object %s from source bucket %s: %w", key, sc.sourceBucket, err)
	}

	log.Debugf("Successfully deleted object: %s", key)
	return nil
}

// ListSourceObjects lists objects in the source bucket with the given prefix
func (sc *S3Client) ListSourceObjects(ctx context.Context, prefix string) ([]string, error) {
	log.Debugf("Listing objects in source bucket with prefix: %s", prefix)

	var objects []string
	paginator := s3.NewListObjectsV2Paginator(sc.sourceClient, &s3.ListObjectsV2Input{
		Bucket: aws.String(sc.sourceBucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects in source bucket %s: %w", sc.sourceBucket, err)
		}

		for _, obj := range output.Contents {
			if obj.Key != nil {
				objects = append(objects, *obj.Key)
			}
		}
	}

	log.Debugf("Found %d objects in source bucket with prefix %s", len(objects), prefix)
	return objects, nil
}
