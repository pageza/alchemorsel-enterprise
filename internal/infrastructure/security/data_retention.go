// Package security provides data retention and deletion policy enforcement
package security

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// DataRetentionService manages data lifecycle and retention policies
type DataRetentionService struct {
	logger      *zap.Logger
	redisClient *redis.Client
	auditLogger *AuditLogger
	policies    map[string]*RetentionPolicy
}

// NewDataRetentionService creates a new data retention service
func NewDataRetentionService(logger *zap.Logger, redisClient *redis.Client, auditLogger *AuditLogger) *DataRetentionService {
	service := &DataRetentionService{
		logger:      logger,
		redisClient: redisClient,
		auditLogger: auditLogger,
		policies:    make(map[string]*RetentionPolicy),
	}
	
	// Initialize default retention policies
	service.initializeDefaultPolicies()
	
	return service
}

// RetentionPolicy defines data retention rules
type RetentionPolicy struct {
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	Description       string              `json:"description"`
	DataTypes         []DataType          `json:"data_types"`
	RetentionPeriod   time.Duration       `json:"retention_period"`
	GracePeriod       time.Duration       `json:"grace_period"`
	DeletionMethod    DeletionMethod      `json:"deletion_method"`
	LegalBasis        LegalBasis          `json:"legal_basis"`
	Exceptions        []RetentionException `json:"exceptions"`
	AutomatedDeletion bool                `json:"automated_deletion"`
	RequiresApproval  bool                `json:"requires_approval"`
	ComplianceReqs    []ComplianceFramework `json:"compliance_requirements"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
	Active            bool                `json:"active"`
}

// DataType represents different types of data
type DataType string

const (
	DataTypeUserProfile      DataType = "user_profile"
	DataTypeAuthData         DataType = "auth_data"
	DataTypeRecipeData       DataType = "recipe_data"
	DataTypeSocialData       DataType = "social_data"
	DataTypeAnalyticsData    DataType = "analytics_data"
	DataTypeAuditLogs        DataType = "audit_logs"
	DataTypeSessionData      DataType = "session_data"
	DataTypePaymentData      DataType = "payment_data"
	DataTypeCommunications   DataType = "communications"
	DataTypeSupport          DataType = "support_data"
	DataTypeMarketingData    DataType = "marketing_data"
	DataTypeDeviceData       DataType = "device_data"
	DataTypeLocationData     DataType = "location_data"
	DataTypeBiometricData    DataType = "biometric_data"
	DataTypeHealthData       DataType = "health_data"
)

// DeletionMethod defines how data should be deleted
type DeletionMethod string

const (
	DeletionSoft        DeletionMethod = "soft_delete"     // Mark as deleted, keep for recovery
	DeletionHard        DeletionMethod = "hard_delete"     // Permanently delete
	DeletionArchive     DeletionMethod = "archive"         // Move to long-term storage
	DeletionAnonymize   DeletionMethod = "anonymize"       // Remove PII, keep aggregated data
	DeletionPseudonymize DeletionMethod = "pseudonymize"   // Replace identifiers with pseudonyms
)

// RetentionException defines exceptions to retention policies
type RetentionException struct {
	Condition   string        `json:"condition"`
	ExtendedPeriod time.Duration `json:"extended_period"`
	Reason      string        `json:"reason"`
	LegalBasis  LegalBasis    `json:"legal_basis"`
}

// DataRecord represents a data record with retention metadata
type DataRecord struct {
	ID              string      `json:"id"`
	UserID          string      `json:"user_id"`
	DataType        DataType    `json:"data_type"`
	CreatedAt       time.Time   `json:"created_at"`
	LastAccessedAt  time.Time   `json:"last_accessed_at"`
	RetentionDate   time.Time   `json:"retention_date"`
	DeletionDate    *time.Time  `json:"deletion_date,omitempty"`
	DeletionMethod  DeletionMethod `json:"deletion_method"`
	LegalHold       bool        `json:"legal_hold"`
	Archived        bool        `json:"archived"`
	PolicyID        string      `json:"policy_id"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// DeletionJob represents a scheduled deletion job
type DeletionJob struct {
	ID             string      `json:"id"`
	DataRecords    []string    `json:"data_records"`
	ScheduledFor   time.Time   `json:"scheduled_for"`
	Status         JobStatus   `json:"status"`
	Method         DeletionMethod `json:"method"`
	CreatedAt      time.Time   `json:"created_at"`
	StartedAt      *time.Time  `json:"started_at,omitempty"`
	CompletedAt    *time.Time  `json:"completed_at,omitempty"`
	Error          string      `json:"error,omitempty"`
	ApprovedBy     string      `json:"approved_by,omitempty"`
	ApprovedAt     *time.Time  `json:"approved_at,omitempty"`
}

// JobStatus represents deletion job status
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusApproved  JobStatus = "approved"
	JobStatusScheduled JobStatus = "scheduled"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// initializeDefaultPolicies sets up default retention policies
func (d *DataRetentionService) initializeDefaultPolicies() {
	policies := []*RetentionPolicy{
		{
			ID:                "user_profile_policy",
			Name:              "User Profile Data",
			Description:       "User account and profile information",
			DataTypes:         []DataType{DataTypeUserProfile, DataTypeAuthData},
			RetentionPeriod:   7 * 365 * 24 * time.Hour, // 7 years
			GracePeriod:       30 * 24 * time.Hour,       // 30 days
			DeletionMethod:    DeletionHard,
			LegalBasis:        BasisContract,
			AutomatedDeletion: true,
			RequiresApproval:  false,
			ComplianceReqs:    []ComplianceFramework{ComplianceGDPR, ComplianceCCPA},
			Active:            true,
		},
		{
			ID:                "recipe_data_policy",
			Name:              "Recipe and Content Data",
			Description:       "User-generated recipes and content",
			DataTypes:         []DataType{DataTypeRecipeData, DataTypeSocialData},
			RetentionPeriod:   3 * 365 * 24 * time.Hour, // 3 years
			GracePeriod:       90 * 24 * time.Hour,       // 90 days
			DeletionMethod:    DeletionArchive,
			LegalBasis:        BasisLegitimateInterest,
			AutomatedDeletion: true,
			RequiresApproval:  false,
			ComplianceReqs:    []ComplianceFramework{ComplianceGDPR},
			Active:            true,
		},
		{
			ID:                "analytics_policy",
			Name:              "Analytics and Usage Data",
			Description:       "Usage analytics and behavioral data",
			DataTypes:         []DataType{DataTypeAnalyticsData, DataTypeDeviceData},
			RetentionPeriod:   26 * 30 * 24 * time.Hour, // 26 months (GDPR requirement)
			GracePeriod:       7 * 24 * time.Hour,        // 7 days
			DeletionMethod:    DeletionAnonymize,
			LegalBasis:        BasisLegitimateInterest,
			AutomatedDeletion: true,
			RequiresApproval:  false,
			ComplianceReqs:    []ComplianceFramework{ComplianceGDPR, ComplianceCCPA},
			Active:            true,
		},
		{
			ID:                "audit_logs_policy",
			Name:              "Security Audit Logs",
			Description:       "Security and compliance audit logs",
			DataTypes:         []DataType{DataTypeAuditLogs},
			RetentionPeriod:   7 * 365 * 24 * time.Hour, // 7 years
			GracePeriod:       0,                         // No grace period for audit logs
			DeletionMethod:    DeletionArchive,
			LegalBasis:        BasisLegalObligation,
			AutomatedDeletion: false, // Manual review required
			RequiresApproval:  true,
			ComplianceReqs:    []ComplianceFramework{ComplianceSOC2, ComplianceISO27001},
			Active:            true,
		},
		{
			ID:                "session_data_policy",
			Name:              "Session and Temporary Data",
			Description:       "Session tokens and temporary data",
			DataTypes:         []DataType{DataTypeSessionData},
			RetentionPeriod:   30 * 24 * time.Hour, // 30 days
			GracePeriod:       0,                    // No grace period
			DeletionMethod:    DeletionHard,
			LegalBasis:        BasisLegitimateInterest,
			AutomatedDeletion: true,
			RequiresApproval:  false,
			ComplianceReqs:    []ComplianceFramework{ComplianceGDPR},
			Active:            true,
		},
		{
			ID:                "payment_data_policy",
			Name:              "Payment and Financial Data",
			Description:       "Payment transactions and financial records",
			DataTypes:         []DataType{DataTypePaymentData},
			RetentionPeriod:   7 * 365 * 24 * time.Hour, // 7 years (legal requirement)
			GracePeriod:       0,                         // No grace period
			DeletionMethod:    DeletionArchive,
			LegalBasis:        BasisLegalObligation,
			AutomatedDeletion: false, // Manual review required
			RequiresApproval:  true,
			ComplianceReqs:    []ComplianceFramework{CompliancePCI, ComplianceSOC2},
			Exceptions: []RetentionException{
				{
					Condition:      "audit_investigation",
					ExtendedPeriod: 2 * 365 * 24 * time.Hour, // Additional 2 years
					Reason:         "Ongoing audit or investigation",
					LegalBasis:     BasisLegalObligation,
				},
			},
			Active: true,
		},
	}
	
	for _, policy := range policies {
		policy.CreatedAt = time.Now()
		policy.UpdatedAt = time.Now()
		d.policies[policy.ID] = policy
		d.storePolicyInRedis(policy)
	}
}

// AddDataRecord registers a new data record for retention tracking
func (d *DataRetentionService) AddDataRecord(userID string, dataType DataType, recordID string, metadata map[string]interface{}) (*DataRecord, error) {
	// Find applicable policy
	policy := d.findPolicyForDataType(dataType)
	if policy == nil {
		return nil, fmt.Errorf("no retention policy found for data type: %s", dataType)
	}
	
	now := time.Now()
	retentionDate := now.Add(policy.RetentionPeriod)
	
	record := &DataRecord{
		ID:             recordID,
		UserID:         userID,
		DataType:       dataType,
		CreatedAt:      now,
		LastAccessedAt: now,
		RetentionDate:  retentionDate,
		DeletionMethod: policy.DeletionMethod,
		LegalHold:      false,
		Archived:       false,
		PolicyID:       policy.ID,
		Metadata:       metadata,
	}
	
	if err := d.storeDataRecord(record); err != nil {
		return nil, fmt.Errorf("failed to store data record: %w", err)
	}
	
	d.logger.Debug("Data record registered for retention",
		zap.String("record_id", recordID),
		zap.String("user_id", userID),
		zap.String("data_type", string(dataType)),
		zap.Time("retention_date", retentionDate),
	)
	
	return record, nil
}

// UpdateLastAccessed updates the last accessed time for a data record
func (d *DataRetentionService) UpdateLastAccessed(recordID string) error {
	record, err := d.getDataRecord(recordID)
	if err != nil {
		return err
	}
	
	record.LastAccessedAt = time.Now()
	return d.storeDataRecord(record)
}

// SetLegalHold places or removes legal hold on data records
func (d *DataRetentionService) SetLegalHold(recordIDs []string, hold bool, reason string, setBy string) error {
	for _, recordID := range recordIDs {
		record, err := d.getDataRecord(recordID)
		if err != nil {
			d.logger.Error("Failed to get data record for legal hold",
				zap.String("record_id", recordID),
				zap.Error(err),
			)
			continue
		}
		
		record.LegalHold = hold
		if err := d.storeDataRecord(record); err != nil {
			return fmt.Errorf("failed to update legal hold for record %s: %w", recordID, err)
		}
		
		// Audit log
		action := "legal_hold_set"
		if !hold {
			action = "legal_hold_removed"
		}
		
		d.auditLogger.LogDataProcessing(AuditEvent{
			UserID:   record.UserID,
			Action:   action,
			Resource: "data_record",
			Details: map[string]interface{}{
				"record_id": recordID,
				"reason":    reason,
				"set_by":    setBy,
			},
			Risk:     RiskHigh,
			Category: CategoryCompliance,
		})
	}
	
	return nil
}

// ScanForExpiredData identifies data records ready for deletion
func (d *DataRetentionService) ScanForExpiredData() ([]DataRecord, error) {
	now := time.Now()
	var expiredRecords []DataRecord
	
	// This would typically scan the database
	// For now, we'll simulate by checking Redis
	ctx := context.Background()
	pattern := "data_record:*"
	
	keys, err := d.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to scan for expired data: %w", err)
	}
	
	for _, key := range keys {
		data, err := d.redisClient.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		
		var record DataRecord
		if err := json.Unmarshal([]byte(data), &record); err != nil {
			continue
		}
		
		// Check if record has expired
		if now.After(record.RetentionDate) && !record.LegalHold && record.DeletionDate == nil {
			expiredRecords = append(expiredRecords, record)
		}
	}
	
	d.logger.Info("Expired data scan completed",
		zap.Int("expired_records", len(expiredRecords)),
	)
	
	return expiredRecords, nil
}

// CreateDeletionJob creates a job to delete expired data
func (d *DataRetentionService) CreateDeletionJob(records []DataRecord, method DeletionMethod) (*DeletionJob, error) {
	recordIDs := make([]string, len(records))
	for i, record := range records {
		recordIDs[i] = record.ID
	}
	
	job := &DeletionJob{
		ID:           fmt.Sprintf("deletion_job_%d", time.Now().UnixNano()),
		DataRecords:  recordIDs,
		ScheduledFor: time.Now().Add(24 * time.Hour), // Schedule for next day
		Status:       JobStatusPending,
		Method:       method,
		CreatedAt:    time.Now(),
	}
	
	// Check if approval is required
	requiresApproval := false
	for _, record := range records {
		policy := d.policies[record.PolicyID]
		if policy != nil && policy.RequiresApproval {
			requiresApproval = true
			break
		}
	}
	
	if requiresApproval {
		job.Status = JobStatusPending // Awaiting approval
	} else {
		job.Status = JobStatusScheduled
	}
	
	if err := d.storeDeletionJob(job); err != nil {
		return nil, fmt.Errorf("failed to store deletion job: %w", err)
	}
	
	// Audit log
	d.auditLogger.LogDataProcessing(AuditEvent{
		Action:   "deletion_job_created",
		Resource: "deletion_job",
		Details: map[string]interface{}{
			"job_id":       job.ID,
			"record_count": len(recordIDs),
			"method":       string(method),
			"requires_approval": requiresApproval,
		},
		Risk:     RiskMedium,
		Category: CategoryCompliance,
	})
	
	d.logger.Info("Deletion job created",
		zap.String("job_id", job.ID),
		zap.Int("record_count", len(recordIDs)),
		zap.String("method", string(method)),
	)
	
	return job, nil
}

// ApproveDeletionJob approves a pending deletion job
func (d *DataRetentionService) ApproveDeletionJob(jobID, approverID string) error {
	job, err := d.getDeletionJob(jobID)
	if err != nil {
		return err
	}
	
	if job.Status != JobStatusPending {
		return fmt.Errorf("job %s is not in pending status", jobID)
	}
	
	now := time.Now()
	job.Status = JobStatusScheduled
	job.ApprovedBy = approverID
	job.ApprovedAt = &now
	
	if err := d.storeDeletionJob(job); err != nil {
		return fmt.Errorf("failed to update deletion job: %w", err)
	}
	
	// Audit log
	d.auditLogger.LogDataProcessing(AuditEvent{
		UserID:   approverID,
		Action:   "deletion_job_approved",
		Resource: "deletion_job",
		Details: map[string]interface{}{
			"job_id": jobID,
		},
		Risk:     RiskHigh,
		Category: CategoryCompliance,
	})
	
	d.logger.Info("Deletion job approved",
		zap.String("job_id", jobID),
		zap.String("approved_by", approverID),
	)
	
	return nil
}

// ExecuteDeletionJob executes a scheduled deletion job
func (d *DataRetentionService) ExecuteDeletionJob(jobID string) error {
	job, err := d.getDeletionJob(jobID)
	if err != nil {
		return err
	}
	
	if job.Status != JobStatusScheduled {
		return fmt.Errorf("job %s is not scheduled for execution", jobID)
	}
	
	// Update job status
	now := time.Now()
	job.Status = JobStatusRunning
	job.StartedAt = &now
	d.storeDeletionJob(job)
	
	// Execute deletion for each record
	successCount := 0
	errorCount := 0
	
	for _, recordID := range job.DataRecords {
		if err := d.executeRecordDeletion(recordID, job.Method); err != nil {
			d.logger.Error("Failed to delete record",
				zap.String("record_id", recordID),
				zap.Error(err),
			)
			errorCount++
		} else {
			successCount++
		}
	}
	
	// Update job completion
	completed := time.Now()
	job.CompletedAt = &completed
	
	if errorCount > 0 {
		job.Status = JobStatusFailed
		job.Error = fmt.Sprintf("Failed to delete %d out of %d records", errorCount, len(job.DataRecords))
	} else {
		job.Status = JobStatusCompleted
	}
	
	d.storeDeletionJob(job)
	
	// Audit log
	d.auditLogger.LogDataProcessing(AuditEvent{
		Action:   "deletion_job_executed",
		Resource: "deletion_job",
		Details: map[string]interface{}{
			"job_id":        jobID,
			"success_count": successCount,
			"error_count":   errorCount,
			"method":        string(job.Method),
		},
		Risk:     RiskMedium,
		Category: CategoryCompliance,
	})
	
	d.logger.Info("Deletion job executed",
		zap.String("job_id", jobID),
		zap.Int("success_count", successCount),
		zap.Int("error_count", errorCount),
	)
	
	return nil
}

// executeRecordDeletion executes deletion for a single record
func (d *DataRetentionService) executeRecordDeletion(recordID string, method DeletionMethod) error {
	record, err := d.getDataRecord(recordID)
	if err != nil {
		return err
	}
	
	// Check legal hold
	if record.LegalHold {
		return fmt.Errorf("record %s is under legal hold", recordID)
	}
	
	switch method {
	case DeletionSoft:
		return d.executeSoftDeletion(record)
	case DeletionHard:
		return d.executeHardDeletion(record)
	case DeletionArchive:
		return d.executeArchival(record)
	case DeletionAnonymize:
		return d.executeAnonymization(record)
	case DeletionPseudonymize:
		return d.executePseudonymization(record)
	default:
		return fmt.Errorf("unsupported deletion method: %s", method)
	}
}

// Deletion method implementations (placeholders for actual implementation)

func (d *DataRetentionService) executeSoftDeletion(record *DataRecord) error {
	// Mark as deleted but keep data for recovery
	now := time.Now()
	record.DeletionDate = &now
	return d.storeDataRecord(record)
}

func (d *DataRetentionService) executeHardDeletion(record *DataRecord) error {
	// Permanently delete data from all systems
	// This would integrate with actual data stores
	d.logger.Info("Hard deletion executed",
		zap.String("record_id", record.ID),
		zap.String("data_type", string(record.DataType)),
	)
	return nil
}

func (d *DataRetentionService) executeArchival(record *DataRecord) error {
	// Move data to long-term archival storage
	record.Archived = true
	now := time.Now()
	record.DeletionDate = &now
	return d.storeDataRecord(record)
}

func (d *DataRetentionService) executeAnonymization(record *DataRecord) error {
	// Remove all PII while keeping aggregated data
	d.logger.Info("Anonymization executed",
		zap.String("record_id", record.ID),
		zap.String("data_type", string(record.DataType)),
	)
	return nil
}

func (d *DataRetentionService) executePseudonymization(record *DataRecord) error {
	// Replace identifiers with pseudonyms
	d.logger.Info("Pseudonymization executed",
		zap.String("record_id", record.ID),
		zap.String("data_type", string(record.DataType)),
	)
	return nil
}

// Helper methods for storage (Redis implementations)

func (d *DataRetentionService) findPolicyForDataType(dataType DataType) *RetentionPolicy {
	for _, policy := range d.policies {
		if !policy.Active {
			continue
		}
		for _, policyDataType := range policy.DataTypes {
			if policyDataType == dataType {
				return policy
			}
		}
	}
	return nil
}

func (d *DataRetentionService) storeDataRecord(record *DataRecord) error {
	ctx := context.Background()
	key := fmt.Sprintf("data_record:%s", record.ID)
	
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	
	return d.redisClient.Set(ctx, key, data, 0).Err()
}

func (d *DataRetentionService) getDataRecord(recordID string) (*DataRecord, error) {
	ctx := context.Background()
	key := fmt.Sprintf("data_record:%s", recordID)
	
	data, err := d.redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	
	var record DataRecord
	if err := json.Unmarshal([]byte(data), &record); err != nil {
		return nil, err
	}
	
	return &record, nil
}

func (d *DataRetentionService) storeDeletionJob(job *DeletionJob) error {
	ctx := context.Background()
	key := fmt.Sprintf("deletion_job:%s", job.ID)
	
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	
	return d.redisClient.Set(ctx, key, data, 0).Err()
}

func (d *DataRetentionService) getDeletionJob(jobID string) (*DeletionJob, error) {
	ctx := context.Background()
	key := fmt.Sprintf("deletion_job:%s", jobID)
	
	data, err := d.redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	
	var job DeletionJob
	if err := json.Unmarshal([]byte(data), &job); err != nil {
		return nil, err
	}
	
	return &job, nil
}

func (d *DataRetentionService) storePolicyInRedis(policy *RetentionPolicy) error {
	ctx := context.Background()
	key := fmt.Sprintf("retention_policy:%s", policy.ID)
	
	data, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	
	return d.redisClient.Set(ctx, key, data, 0).Err()
}

// GetRetentionPolicies returns all active retention policies
func (d *DataRetentionService) GetRetentionPolicies() []*RetentionPolicy {
	var policies []*RetentionPolicy
	for _, policy := range d.policies {
		if policy.Active {
			policies = append(policies, policy)
		}
	}
	return policies
}

// GetPendingDeletionJobs returns deletion jobs pending approval
func (d *DataRetentionService) GetPendingDeletionJobs() ([]*DeletionJob, error) {
	// Implementation would query Redis for pending jobs
	return []*DeletionJob{}, nil
}

// RunRetentionScheduler runs the automated retention process
func (d *DataRetentionService) RunRetentionScheduler() error {
	d.logger.Info("Running data retention scheduler")
	
	// Scan for expired data
	expiredRecords, err := d.ScanForExpiredData()
	if err != nil {
		return fmt.Errorf("failed to scan for expired data: %w", err)
	}
	
	if len(expiredRecords) == 0 {
		d.logger.Info("No expired data found")
		return nil
	}
	
	// Group records by deletion method
	recordsByMethod := make(map[DeletionMethod][]DataRecord)
	for _, record := range expiredRecords {
		recordsByMethod[record.DeletionMethod] = append(recordsByMethod[record.DeletionMethod], record)
	}
	
	// Create deletion jobs for each method
	for method, records := range recordsByMethod {
		job, err := d.CreateDeletionJob(records, method)
		if err != nil {
			d.logger.Error("Failed to create deletion job",
				zap.String("method", string(method)),
				zap.Error(err),
			)
			continue
		}
		
		// Auto-execute jobs that don't require approval
		if job.Status == JobStatusScheduled {
			if err := d.ExecuteDeletionJob(job.ID); err != nil {
				d.logger.Error("Failed to execute deletion job",
					zap.String("job_id", job.ID),
					zap.Error(err),
				)
			}
		}
	}
	
	return nil
}