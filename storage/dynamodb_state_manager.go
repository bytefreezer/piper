package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"github.com/n0needt0/go-goodies/log"

	"github.com/n0needt0/bytefreezer-piper/config"
	"github.com/n0needt0/bytefreezer-piper/domain"
)

// DynamoDBStateManager manages distributed state using DynamoDB
type DynamoDBStateManager struct {
	client            *dynamodb.Client
	lockTable         string
	jobTable          string
	configTable       string
	coordinationTable string
}

// NewDynamoDBStateManager creates a new DynamoDB state manager
func NewDynamoDBStateManager(cfg *config.DynamoDB) (*DynamoDBStateManager, error) {
	// Create AWS config
	var awsCfg aws.Config
	var err error

	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		// Use static credentials
		awsCfg, err = awsconfig.LoadDefaultConfig(context.Background(),
			awsconfig.WithRegion(cfg.Region),
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.AccessKey,
				cfg.SecretKey,
				"",
			)),
		)
	} else {
		// Use default credential chain
		awsCfg, err = awsconfig.LoadDefaultConfig(context.Background(),
			awsconfig.WithRegion(cfg.Region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create DynamoDB client
	var client *dynamodb.Client
	if cfg.Endpoint != "" {
		client = dynamodb.NewFromConfig(awsCfg, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	} else {
		client = dynamodb.NewFromConfig(awsCfg)
	}

	return &DynamoDBStateManager{
		client:            client,
		lockTable:         cfg.LockTable,
		jobTable:          cfg.JobTable,
		configTable:       cfg.ConfigTable,
		coordinationTable: cfg.CoordinationTable,
	}, nil
}

// File Lock Operations

// AcquireFileLock attempts to acquire a lock for file processing
func (dm *DynamoDBStateManager) AcquireFileLock(ctx context.Context, fileKey, processorType, processorID, jobID string) error {
	lock := domain.FileLock{
		FileKey:         fileKey,
		ProcessorType:   processorType,
		ProcessorID:     processorID,
		JobID:           jobID,
		LockTimestamp:   time.Now(),
		TTL:             time.Now().Add(1 * time.Hour).Unix(),
		LockVersion:     1,
	}

	item, err := attributevalue.MarshalMap(lock)
	if err != nil {
		return fmt.Errorf("failed to marshal lock: %w", err)
	}

	// Use conditional write to prevent race conditions
	_, err = dm.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(dm.lockTable),
		Item:      item,
		ConditionExpression: aws.String("attribute_not_exists(file_key)"),
	})

	if err != nil {
		var conditionalCheckFailedException *types.ConditionalCheckFailedException
		if errors.As(err, &conditionalCheckFailedException) {
			return fmt.Errorf("file %s is already locked by another processor", fileKey)
		}
		return fmt.Errorf("failed to acquire lock for file %s: %w", fileKey, err)
	}

	log.Debugf("Acquired lock for file %s by processor %s (job %s)", fileKey, processorID, jobID)
	return nil
}

// ReleaseFileLock releases a file processing lock
func (dm *DynamoDBStateManager) ReleaseFileLock(ctx context.Context, fileKey, processorType string) error {
	_, err := dm.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(dm.lockTable),
		Key: map[string]types.AttributeValue{
			"file_key": &types.AttributeValueMemberS{Value: fileKey},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to release lock for file %s: %w", fileKey, err)
	}

	log.Debugf("Released lock for file %s", fileKey)
	return nil
}

// IsFileLocked checks if a file is currently locked
func (dm *DynamoDBStateManager) IsFileLocked(ctx context.Context, fileKey, processorType string) (bool, error) {
	result, err := dm.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(dm.lockTable),
		Key: map[string]types.AttributeValue{
			"file_key": &types.AttributeValueMemberS{Value: fileKey},
		},
	})

	if err != nil {
		return false, fmt.Errorf("failed to check lock for file %s: %w", fileKey, err)
	}

	return len(result.Item) > 0, nil
}

// Job Management Operations

// CreateJob creates a new processing job record
func (dm *DynamoDBStateManager) CreateJob(ctx context.Context, job *domain.ProcessingJob) error {
	jobRecord := domain.JobRecord{
		JobID:         job.JobID,
		TenantID:      job.TenantID,
		DatasetID:     job.DatasetID,
		ProcessorType: "piper",
		ProcessorID:   job.ProcessorID,
		Status:        string(domain.JobStatusQueued),
		Priority:      job.Priority,
		RetryCount:    0,
		MaxRetries:    3,
		CreatedAt:     job.CreatedAt,
		UpdatedAt:     job.CreatedAt,
		TTL:           time.Now().Add(30 * 24 * time.Hour).Unix(), // 30 days retention
		SourceFiles:   []string{job.SourceFile.Key},
		FileSize:      job.SourceFile.Size,
	}

	item, err := attributevalue.MarshalMap(jobRecord)
	if err != nil {
		return fmt.Errorf("failed to marshal job record: %w", err)
	}

	_, err = dm.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(dm.jobTable),
		Item:      item,
	})

	if err != nil {
		return fmt.Errorf("failed to create job %s: %w", job.JobID, err)
	}

	log.Debugf("Created job record %s for file %s", job.JobID, job.SourceFile.Key)
	return nil
}

// UpdateJobStatus updates the status of a processing job
func (dm *DynamoDBStateManager) UpdateJobStatus(ctx context.Context, jobID string, status domain.JobStatus, metadata map[string]interface{}) error {
	updateExpr := "SET #status = :status, updated_at = :updated_at"
	exprAttrNames := map[string]string{
		"#status": "status",
	}
	exprAttrValues := map[string]types.AttributeValue{
		":status":     &types.AttributeValueMemberS{Value: string(status)},
		":updated_at": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
	}

	// Add optional metadata
	if startedAt, ok := metadata["started_at"].(time.Time); ok {
		updateExpr += ", started_at = :started_at"
		exprAttrValues[":started_at"] = &types.AttributeValueMemberS{Value: startedAt.Format(time.RFC3339)}
	}

	if completedAt, ok := metadata["completed_at"].(time.Time); ok {
		updateExpr += ", completed_at = :completed_at"
		exprAttrValues[":completed_at"] = &types.AttributeValueMemberS{Value: completedAt.Format(time.RFC3339)}
	}

	if errorMsg, ok := metadata["error_message"].(string); ok {
		updateExpr += ", error_message = :error_message"
		exprAttrValues[":error_message"] = &types.AttributeValueMemberS{Value: errorMsg}
	}

	if outputFile, ok := metadata["output_file"].(string); ok {
		updateExpr += ", output_files = :output_files"
		exprAttrValues[":output_files"] = &types.AttributeValueMemberSS{Value: []string{outputFile}}
	}

	if recordCount, ok := metadata["record_count"].(int64); ok {
		updateExpr += ", record_count = :record_count"
		exprAttrValues[":record_count"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", recordCount)}
	}

	if processingTime, ok := metadata["processing_time_ms"].(int64); ok {
		updateExpr += ", processing_time_ms = :processing_time_ms"
		exprAttrValues[":processing_time_ms"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", processingTime)}
	}

	_, err := dm.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(dm.jobTable),
		Key: map[string]types.AttributeValue{
			"job_id": &types.AttributeValueMemberS{Value: jobID},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeNames:  exprAttrNames,
		ExpressionAttributeValues: exprAttrValues,
	})

	if err != nil {
		return fmt.Errorf("failed to update job %s status to %s: %w", jobID, status, err)
	}

	log.Debugf("Updated job %s status to %s", jobID, status)
	return nil
}

// GetJobStatus retrieves the current status of a job
func (dm *DynamoDBStateManager) GetJobStatus(ctx context.Context, jobID string) (*domain.JobRecord, error) {
	result, err := dm.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(dm.jobTable),
		Key: map[string]types.AttributeValue{
			"job_id": &types.AttributeValueMemberS{Value: jobID},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get job %s: %w", jobID, err)
	}

	if len(result.Item) == 0 {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	var jobRecord domain.JobRecord
	err = attributevalue.UnmarshalMap(result.Item, &jobRecord)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal job record: %w", err)
	}

	return &jobRecord, nil
}

// GetJobsByStatus retrieves jobs with a specific status
func (dm *DynamoDBStateManager) GetJobsByStatus(ctx context.Context, status domain.JobStatus, limit int) ([]domain.JobRecord, error) {
	if limit <= 0 {
		limit = 100
	}

	result, err := dm.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(dm.jobTable),
		IndexName:              aws.String("status-created-index"),
		KeyConditionExpression: aws.String("#status = :status"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: string(status)},
		},
		Limit:            aws.Int32(int32(limit)),
		ScanIndexForward: aws.Bool(true), // Oldest first
	})

	if err != nil {
		return nil, fmt.Errorf("failed to query jobs by status %s: %w", status, err)
	}

	var jobs []domain.JobRecord
	for _, item := range result.Items {
		var job domain.JobRecord
		err = attributevalue.UnmarshalMap(item, &job)
		if err != nil {
			log.Errorf("Failed to unmarshal job record: %v", err)
			continue
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// Pipeline Configuration Operations

// GetPipelineConfig retrieves pipeline configuration for a tenant/dataset
func (dm *DynamoDBStateManager) GetPipelineConfig(ctx context.Context, tenantID, datasetID string) (*domain.PipelineConfiguration, error) {
	configKey := fmt.Sprintf("tenant#%s#dataset#%s", tenantID, datasetID)

	result, err := dm.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(dm.configTable),
		Key: map[string]types.AttributeValue{
			"config_key": &types.AttributeValueMemberS{Value: configKey},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get pipeline config for %s/%s: %w", tenantID, datasetID, err)
	}

	if len(result.Item) == 0 {
		// Return default empty configuration if not found
		return &domain.PipelineConfiguration{
			ConfigKey: configKey,
			TenantID:  tenantID,
			DatasetID: datasetID,
			Enabled:   false,
			Version:   "1.0.0",
			Filters:   []domain.FilterConfig{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Validated: true,
		}, nil
	}

	var config domain.PipelineConfiguration
	err = attributevalue.UnmarshalMap(result.Item, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal pipeline config: %w", err)
	}

	return &config, nil
}

// Service Coordination Operations

// RegisterServiceInstance registers this service instance
func (dm *DynamoDBStateManager) RegisterServiceInstance(ctx context.Context, instance *domain.ServiceInstance) error {
	instance.CoordinationKey = fmt.Sprintf("instance#%s#%s", instance.ServiceType, instance.InstanceID)
	instance.TTL = time.Now().Add(5 * time.Minute).Unix() // 5 minute TTL

	item, err := attributevalue.MarshalMap(instance)
	if err != nil {
		return fmt.Errorf("failed to marshal service instance: %w", err)
	}

	_, err = dm.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(dm.coordinationTable),
		Item:      item,
	})

	if err != nil {
		return fmt.Errorf("failed to register service instance: %w", err)
	}

	log.Infof("Registered service instance %s", instance.InstanceID)
	return nil
}

// UpdateHeartbeat updates the heartbeat for this service instance
func (dm *DynamoDBStateManager) UpdateHeartbeat(ctx context.Context, serviceType, instanceID string) error {
	coordinationKey := fmt.Sprintf("instance#%s#%s", serviceType, instanceID)

	_, err := dm.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(dm.coordinationTable),
		Key: map[string]types.AttributeValue{
			"coordination_key": &types.AttributeValueMemberS{Value: coordinationKey},
		},
		UpdateExpression: aws.String("SET last_heartbeat = :heartbeat, #ttl = :ttl"),
		ExpressionAttributeNames: map[string]string{
			"#ttl": "ttl",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":heartbeat": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
			":ttl":       &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Add(5*time.Minute).Unix())},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to update heartbeat: %w", err)
	}

	return nil
}

// GenerateJobID generates a unique job ID
func GenerateJobID(processorType string) string {
	timestamp := time.Now().Format("20060102-150405")
	shortUUID := uuid.New().String()[:8]
	return fmt.Sprintf("job-%s-%s-%s", processorType, timestamp, shortUUID)
}