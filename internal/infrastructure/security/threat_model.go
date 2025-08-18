// Package security provides threat modeling and risk assessment capabilities
package security

import (
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// ThreatLevel represents the severity of a threat
type ThreatLevel string

const (
	ThreatLevelLow      ThreatLevel = "low"
	ThreatLevelMedium   ThreatLevel = "medium"
	ThreatLevelHigh     ThreatLevel = "high"
	ThreatLevelCritical ThreatLevel = "critical"
)

// ThreatCategory represents different types of threats
type ThreatCategory string

const (
	CategorySpoofing         ThreatCategory = "spoofing"
	CategoryTampering        ThreatCategory = "tampering"
	CategoryRepudiation      ThreatCategory = "repudiation"
	CategoryInfoDisclosure   ThreatCategory = "information_disclosure"
	CategoryDenialOfService  ThreatCategory = "denial_of_service"
	CategoryElevationPriv    ThreatCategory = "elevation_of_privilege"
	CategoryDataBreach       ThreatCategory = "data_breach"
	CategoryInjection        ThreatCategory = "injection"
	CategoryBrokenAuth       ThreatCategory = "broken_authentication"
	CategorySensitiveData    ThreatCategory = "sensitive_data_exposure"
	CategoryXXE             ThreatCategory = "xxe"
	CategoryBrokenAccess     ThreatCategory = "broken_access_control"
	CategorySecurityConfig   ThreatCategory = "security_misconfiguration"
	CategoryXSS             ThreatCategory = "xss"
	CategoryInsecureDesign   ThreatCategory = "insecure_design"
)

// Asset represents a system asset that needs protection
type Asset struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Type         AssetType   `json:"type"`
	Value        AssetValue  `json:"value"`
	Owner        string      `json:"owner"`
	Location     string      `json:"location"`
	Dependencies []string    `json:"dependencies"`
}

// AssetType represents different types of assets
type AssetType string

const (
	AssetTypeData         AssetType = "data"
	AssetTypeApplication  AssetType = "application"
	AssetTypeInfra        AssetType = "infrastructure"
	AssetTypeNetwork      AssetType = "network"
	AssetTypePeople       AssetType = "people"
	AssetTypeProcess      AssetType = "process"
)

// AssetValue represents the business value of an asset
type AssetValue string

const (
	AssetValueLow      AssetValue = "low"
	AssetValueMedium   AssetValue = "medium"
	AssetValueHigh     AssetValue = "high"
	AssetValueCritical AssetValue = "critical"
)

// Threat represents a potential security threat
type Threat struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Category      ThreatCategory `json:"category"`
	Level         ThreatLevel    `json:"level"`
	Likelihood    int            `json:"likelihood"`    // 1-5 scale
	Impact        int            `json:"impact"`        // 1-5 scale
	RiskScore     int            `json:"risk_score"`    // Likelihood * Impact
	TargetAssets  []string       `json:"target_assets"`
	AttackVectors []string       `json:"attack_vectors"`
	Mitigations   []Mitigation   `json:"mitigations"`
	References    []string       `json:"references"`
	LastUpdated   time.Time      `json:"last_updated"`
}

// Mitigation represents a security control or countermeasure
type Mitigation struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	Type           MitigationType   `json:"type"`
	Status         MitigationStatus `json:"status"`
	Effectiveness  int              `json:"effectiveness"` // 1-5 scale
	Implementation string           `json:"implementation"`
	Owner          string           `json:"owner"`
	DueDate        *time.Time       `json:"due_date,omitempty"`
	CompletedDate  *time.Time       `json:"completed_date,omitempty"`
}

// MitigationType represents different types of mitigations
type MitigationType string

const (
	MitigationPreventive  MitigationType = "preventive"
	MitigationDetective   MitigationType = "detective"
	MitigationCorrective  MitigationType = "corrective"
	MitigationRecovery    MitigationType = "recovery"
	MitigationCompensating MitigationType = "compensating"
)

// MitigationStatus represents the status of a mitigation
type MitigationStatus string

const (
	StatusPlanned      MitigationStatus = "planned"
	StatusInProgress   MitigationStatus = "in_progress"
	StatusImplemented  MitigationStatus = "implemented"
	StatusVerified     MitigationStatus = "verified"
	StatusNotEffective MitigationStatus = "not_effective"
)

// ThreatModel represents the complete threat model for the system
type ThreatModel struct {
	SystemName    string    `json:"system_name"`
	Version       string    `json:"version"`
	LastUpdated   time.Time `json:"last_updated"`
	Assets        []Asset   `json:"assets"`
	Threats       []Threat  `json:"threats"`
	Architecture  string    `json:"architecture"`
	DataFlows     []DataFlow `json:"data_flows"`
	TrustBoundaries []TrustBoundary `json:"trust_boundaries"`
}

// DataFlow represents data movement in the system
type DataFlow struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Source      string   `json:"source"`
	Destination string   `json:"destination"`
	DataTypes   []string `json:"data_types"`
	Protocol    string   `json:"protocol"`
	Encryption  bool     `json:"encryption"`
	Authentication bool  `json:"authentication"`
}

// TrustBoundary represents security boundaries in the system
type TrustBoundary struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Components  []string `json:"components"`
	Controls    []string `json:"controls"`
}

// ThreatModelService provides threat modeling capabilities
type ThreatModelService struct {
	logger *zap.Logger
	model  *ThreatModel
}

// NewThreatModelService creates a new threat modeling service
func NewThreatModelService(logger *zap.Logger) *ThreatModelService {
	service := &ThreatModelService{
		logger: logger,
	}
	
	// Initialize with Alchemorsel v3 threat model
	service.initializeAlchemorselThreatModel()
	
	return service
}

// initializeAlchemorselThreatModel creates the threat model for Alchemorsel v3
func (t *ThreatModelService) initializeAlchemorselThreatModel() {
	t.model = &ThreatModel{
		SystemName:  "Alchemorsel v3",
		Version:     "3.0.0",
		LastUpdated: time.Now(),
		Architecture: "Hexagonal Architecture with DDD",
		Assets: []Asset{
			{
				ID:          "user-data",
				Name:        "User Personal Data",
				Description: "User profiles, preferences, dietary restrictions",
				Type:        AssetTypeData,
				Value:       AssetValueHigh,
				Owner:       "Data Protection Officer",
				Location:    "PostgreSQL Database",
				Dependencies: []string{"database", "encryption"},
			},
			{
				ID:          "recipe-data",
				Name:        "Recipe Database",
				Description: "Recipe content, images, nutritional data",
				Type:        AssetTypeData,
				Value:       AssetValueMedium,
				Owner:       "Product Team",
				Location:    "PostgreSQL Database, S3 Storage",
				Dependencies: []string{"database", "storage"},
			},
			{
				ID:          "ai-interactions",
				Name:        "AI Interaction Data",
				Description: "User interactions with AI services, prompts, responses",
				Type:        AssetTypeData,
				Value:       AssetValueHigh,
				Owner:       "AI Team",
				Location:    "Application Logs, Analytics",
				Dependencies: []string{"logging", "analytics"},
			},
			{
				ID:          "payment-data",
				Name:        "Payment Information",
				Description: "Subscription data, payment methods (tokenized)",
				Type:        AssetTypeData,
				Value:       AssetValueCritical,
				Owner:       "Finance Team",
				Location:    "Third-party Payment Processor",
				Dependencies: []string{"payment-gateway"},
			},
			{
				ID:          "api-keys",
				Name:        "API Keys and Secrets",
				Description: "Third-party API keys, JWT secrets, encryption keys",
				Type:        AssetTypeData,
				Value:       AssetValueCritical,
				Owner:       "Security Team",
				Location:    "Key Management Service",
				Dependencies: []string{"kms", "secrets-manager"},
			},
		},
		
		DataFlows: []DataFlow{
			{
				ID:          "user-auth",
				Name:        "User Authentication",
				Source:      "Web Client",
				Destination: "Auth Service",
				DataTypes:   []string{"credentials", "tokens"},
				Protocol:    "HTTPS",
				Encryption:  true,
				Authentication: true,
			},
			{
				ID:          "api-requests",
				Name:        "API Requests",
				Source:      "Web/Mobile Client",
				Destination: "API Gateway",
				DataTypes:   []string{"user-data", "recipe-data"},
				Protocol:    "HTTPS",
				Encryption:  true,
				Authentication: true,
			},
			{
				ID:          "ai-service",
				Name:        "AI Service Integration",
				Source:      "Application",
				Destination: "External AI APIs",
				DataTypes:   []string{"prompts", "responses"},
				Protocol:    "HTTPS",
				Encryption:  true,
				Authentication: true,
			},
		},
		
		TrustBoundaries: []TrustBoundary{
			{
				ID:          "internet",
				Name:        "Internet Boundary",
				Description: "Public internet to application boundary",
				Components:  []string{"load-balancer", "waf", "cdn"},
				Controls:    []string{"https", "rate-limiting", "ddos-protection"},
			},
			{
				ID:          "application",
				Name:        "Application Boundary",
				Description: "Application layer trust boundary",
				Components:  []string{"api-gateway", "auth-service", "business-logic"},
				Controls:    []string{"authentication", "authorization", "input-validation"},
			},
			{
				ID:          "data",
				Name:        "Data Layer Boundary",
				Description: "Data persistence layer boundary",
				Components:  []string{"database", "cache", "storage"},
				Controls:    []string{"encryption", "access-control", "audit-logging"},
			},
		},
		
		Threats: t.getAlchemorselThreats(),
	}
}

// getAlchemorselThreats returns the threat catalog for Alchemorsel v3
func (t *ThreatModelService) getAlchemorselThreats() []Threat {
	now := time.Now()
	
	return []Threat{
		{
			ID:          "T001",
			Name:        "SQL Injection in Recipe Search",
			Description: "Attacker injects malicious SQL through recipe search parameters",
			Category:    CategoryInjection,
			Level:       ThreatLevelHigh,
			Likelihood:  3,
			Impact:      4,
			RiskScore:   12,
			TargetAssets: []string{"recipe-data", "user-data"},
			AttackVectors: []string{"search-forms", "api-parameters"},
			Mitigations: []Mitigation{
				{
					ID:            "M001",
					Name:          "Parameterized Queries",
					Description:   "Use parameterized queries and ORM",
					Type:          MitigationPreventive,
					Status:        StatusImplemented,
					Effectiveness: 5,
					Implementation: "GORM with prepared statements",
					Owner:         "Development Team",
				},
				{
					ID:            "M002",
					Name:          "Input Validation",
					Description:   "Validate and sanitize all user inputs",
					Type:          MitigationPreventive,
					Status:        StatusPlanned,
					Effectiveness: 4,
					Implementation: "Custom validation middleware",
					Owner:         "Development Team",
				},
			},
			References:   []string{"OWASP-A03", "CWE-89"},
			LastUpdated:  now,
		},
		{
			ID:          "T002",
			Name:        "Cross-Site Scripting (XSS) in Recipe Content",
			Description: "Stored XSS through malicious recipe content or comments",
			Category:    CategoryXSS,
			Level:       ThreatLevelMedium,
			Likelihood:  4,
			Impact:      3,
			RiskScore:   12,
			TargetAssets: []string{"user-data", "recipe-data"},
			AttackVectors: []string{"recipe-forms", "comments", "user-profiles"},
			Mitigations: []Mitigation{
				{
					ID:            "M003",
					Name:          "Content Security Policy",
					Description:   "Implement strict CSP headers",
					Type:          MitigationPreventive,
					Status:        StatusInProgress,
					Effectiveness: 4,
					Implementation: "HTTP middleware",
					Owner:         "Security Team",
				},
				{
					ID:            "M004",
					Name:          "Output Encoding",
					Description:   "HTML encode all user-generated content",
					Type:          MitigationPreventive,
					Status:        StatusPlanned,
					Effectiveness: 5,
					Implementation: "Template engine sanitization",
					Owner:         "Development Team",
				},
			},
			References:   []string{"OWASP-A07", "CWE-79"},
			LastUpdated:  now,
		},
		{
			ID:          "T003",
			Name:        "Broken Authentication",
			Description: "Weak password policies, session management vulnerabilities",
			Category:    CategoryBrokenAuth,
			Level:       ThreatLevelHigh,
			Likelihood:  3,
			Impact:      5,
			RiskScore:   15,
			TargetAssets: []string{"user-data", "api-keys"},
			AttackVectors: []string{"login-forms", "session-tokens", "password-reset"},
			Mitigations: []Mitigation{
				{
					ID:            "M005",
					Name:          "Multi-Factor Authentication",
					Description:   "Implement TOTP-based MFA",
					Type:          MitigationPreventive,
					Status:        StatusPlanned,
					Effectiveness: 5,
					Implementation: "Authentication service integration",
					Owner:         "Security Team",
				},
				{
					ID:            "M006",
					Name:          "Strong Password Policy",
					Description:   "Enforce strong password requirements",
					Type:          MitigationPreventive,
					Status:        StatusImplemented,
					Effectiveness: 3,
					Implementation: "Password validation middleware",
					Owner:         "Development Team",
				},
			},
			References:   []string{"OWASP-A07", "CWE-287"},
			LastUpdated:  now,
		},
		{
			ID:          "T004",
			Name:        "Sensitive Data Exposure",
			Description: "PII exposure through logs, error messages, or unencrypted storage",
			Category:    CategorySensitiveData,
			Level:       ThreatLevelCritical,
			Likelihood:  2,
			Impact:      5,
			RiskScore:   10,
			TargetAssets: []string{"user-data", "payment-data"},
			AttackVectors: []string{"database-access", "log-files", "error-messages"},
			Mitigations: []Mitigation{
				{
					ID:            "M007",
					Name:          "Database Encryption",
					Description:   "Encrypt sensitive data at rest",
					Type:          MitigationPreventive,
					Status:        StatusPlanned,
					Effectiveness: 5,
					Implementation: "AES-256 encryption",
					Owner:         "Infrastructure Team",
				},
				{
					ID:            "M008",
					Name:          "Data Classification",
					Description:   "Classify and label sensitive data",
					Type:          MitigationPreventive,
					Status:        StatusPlanned,
					Effectiveness: 3,
					Implementation: "Data governance framework",
					Owner:         "Security Team",
				},
			},
			References:   []string{"OWASP-A02", "CWE-200"},
			LastUpdated:  now,
		},
		{
			ID:          "T005",
			Name:        "Broken Access Control",
			Description: "Unauthorized access to recipes, user data, or admin functions",
			Category:    CategoryBrokenAccess,
			Level:       ThreatLevelHigh,
			Likelihood:  4,
			Impact:      4,
			RiskScore:   16,
			TargetAssets: []string{"user-data", "recipe-data", "api-keys"},
			AttackVectors: []string{"api-endpoints", "admin-panels", "direct-object-references"},
			Mitigations: []Mitigation{
				{
					ID:            "M009",
					Name:          "Role-Based Access Control",
					Description:   "Implement RBAC system",
					Type:          MitigationPreventive,
					Status:        StatusImplemented,
					Effectiveness: 5,
					Implementation: "Custom RBAC middleware",
					Owner:         "Security Team",
				},
				{
					ID:            "M010",
					Name:          "Authorization Testing",
					Description:   "Automated authorization testing",
					Type:          MitigationDetective,
					Status:        StatusPlanned,
					Effectiveness: 4,
					Implementation: "Security test suite",
					Owner:         "QA Team",
				},
			},
			References:   []string{"OWASP-A01", "CWE-862"},
			LastUpdated:  now,
		},
		{
			ID:          "T006",
			Name:        "AI Prompt Injection",
			Description: "Malicious prompts to manipulate AI responses or extract training data",
			Category:    CategoryInjection,
			Level:       ThreatLevelMedium,
			Likelihood:  3,
			Impact:      3,
			RiskScore:   9,
			TargetAssets: []string{"ai-interactions", "user-data"},
			AttackVectors: []string{"recipe-generation", "ai-chat", "ingredient-suggestions"},
			Mitigations: []Mitigation{
				{
					ID:            "M011",
					Name:          "Prompt Sanitization",
					Description:   "Sanitize and validate AI prompts",
					Type:          MitigationPreventive,
					Status:        StatusPlanned,
					Effectiveness: 3,
					Implementation: "AI security filters",
					Owner:         "AI Team",
				},
				{
					ID:            "M012",
					Name:          "AI Response Monitoring",
					Description:   "Monitor AI responses for anomalies",
					Type:          MitigationDetective,
					Status:        StatusPlanned,
					Effectiveness: 3,
					Implementation: "ML monitoring system",
					Owner:         "AI Team",
				},
			},
			References:   []string{"OWASP-LLM-01"},
			LastUpdated:  now,
		},
		{
			ID:          "T007",
			Name:        "DDoS Attack",
			Description: "Distributed denial of service targeting API endpoints",
			Category:    CategoryDenialOfService,
			Level:       ThreatLevelMedium,
			Likelihood:  3,
			Impact:      3,
			RiskScore:   9,
			TargetAssets: []string{"api-gateway", "database"},
			AttackVectors: []string{"http-flooding", "slowloris", "application-layer"},
			Mitigations: []Mitigation{
				{
					ID:            "M013",
					Name:          "Rate Limiting",
					Description:   "Implement progressive rate limiting",
					Type:          MitigationPreventive,
					Status:        StatusImplemented,
					Effectiveness: 4,
					Implementation: "Redis-based rate limiter",
					Owner:         "Infrastructure Team",
				},
				{
					ID:            "M014",
					Name:          "DDoS Protection Service",
					Description:   "Use cloud-based DDoS protection",
					Type:          MitigationPreventive,
					Status:        StatusPlanned,
					Effectiveness: 5,
					Implementation: "Cloudflare/AWS Shield",
					Owner:         "Infrastructure Team",
				},
			},
			References:   []string{"CWE-400"},
			LastUpdated:  now,
		},
		{
			ID:          "T008",
			Name:        "Insecure Direct Object References",
			Description: "Direct access to internal objects via predictable identifiers",
			Category:    CategoryBrokenAccess,
			Level:       ThreatLevelMedium,
			Likelihood:  4,
			Impact:      3,
			RiskScore:   12,
			TargetAssets: []string{"user-data", "recipe-data"},
			AttackVectors: []string{"api-parameters", "url-manipulation"},
			Mitigations: []Mitigation{
				{
					ID:            "M015",
					Name:          "UUID Identifiers",
					Description:   "Use UUIDs instead of sequential IDs",
					Type:          MitigationPreventive,
					Status:        StatusImplemented,
					Effectiveness: 3,
					Implementation: "Database schema design",
					Owner:         "Development Team",
				},
				{
					ID:            "M016",
					Name:          "Object-Level Authorization",
					Description:   "Check authorization for every object access",
					Type:          MitigationPreventive,
					Status:        StatusInProgress,
					Effectiveness: 5,
					Implementation: "Authorization middleware",
					Owner:         "Security Team",
				},
			},
			References:   []string{"OWASP-A01", "CWE-639"},
			LastUpdated:  now,
		},
	}
}

// GetThreatModel returns the current threat model
func (t *ThreatModelService) GetThreatModel() *ThreatModel {
	return t.model
}

// CalculateRiskScore calculates risk score based on likelihood and impact
func (t *ThreatModelService) CalculateRiskScore(likelihood, impact int) int {
	return likelihood * impact
}

// GetHighRiskThreats returns threats with risk score above threshold
func (t *ThreatModelService) GetHighRiskThreats(threshold int) []Threat {
	var highRiskThreats []Threat
	
	for _, threat := range t.model.Threats {
		if threat.RiskScore >= threshold {
			highRiskThreats = append(highRiskThreats, threat)
		}
	}
	
	return highRiskThreats
}

// GetMitigationsByStatus returns mitigations filtered by status
func (t *ThreatModelService) GetMitigationsByStatus(status MitigationStatus) []Mitigation {
	var mitigations []Mitigation
	
	for _, threat := range t.model.Threats {
		for _, mitigation := range threat.Mitigations {
			if mitigation.Status == status {
				mitigations = append(mitigations, mitigation)
			}
		}
	}
	
	return mitigations
}

// ExportThreatModel exports the threat model as JSON
func (t *ThreatModelService) ExportThreatModel() ([]byte, error) {
	return json.MarshalIndent(t.model, "", "  ")
}

// GenerateRiskMatrix generates a risk assessment matrix
func (t *ThreatModelService) GenerateRiskMatrix() map[string]int {
	matrix := make(map[string]int)
	
	for _, threat := range t.model.Threats {
		key := fmt.Sprintf("L%d-I%d", threat.Likelihood, threat.Impact)
		matrix[key]++
	}
	
	return matrix
}

// UpdateThreat updates an existing threat
func (t *ThreatModelService) UpdateThreat(threatID string, updates Threat) error {
	for i, threat := range t.model.Threats {
		if threat.ID == threatID {
			updates.LastUpdated = time.Now()
			updates.RiskScore = t.CalculateRiskScore(updates.Likelihood, updates.Impact)
			t.model.Threats[i] = updates
			return nil
		}
	}
	
	return fmt.Errorf("threat not found: %s", threatID)
}

// AddThreat adds a new threat to the model
func (t *ThreatModelService) AddThreat(threat Threat) {
	threat.LastUpdated = time.Now()
	threat.RiskScore = t.CalculateRiskScore(threat.Likelihood, threat.Impact)
	t.model.Threats = append(t.model.Threats, threat)
}

// GetThreatsByCategory returns threats filtered by category
func (t *ThreatModelService) GetThreatsByCategory(category ThreatCategory) []Threat {
	var threats []Threat
	
	for _, threat := range t.model.Threats {
		if threat.Category == category {
			threats = append(threats, threat)
		}
	}
	
	return threats
}