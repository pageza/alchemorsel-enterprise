package network

import (
	"context"
	"fmt"
	"net"
	"sort"
	"time"

	"go.uber.org/zap"
)

// GlobalNetworkConfig holds global network optimization configuration
type GlobalNetworkConfig struct {
	Regions              []Region
	LoadBalancingStrategy LoadBalancingStrategy
	FailoverPolicy       FailoverPolicy
	LatencyThresholds    LatencyThresholds
	CDNConfig            CDNConfiguration
	DatabaseReplication  DatabaseReplicationConfig
	EdgeComputing        EdgeComputingConfig
}

// Region represents a geographical region with network endpoints
type Region struct {
	Name         string
	Code         string
	Country      string
	Continent    string
	Endpoints    []Endpoint
	Priority     int
	IsActive     bool
	HealthCheck  HealthCheckConfig
	Capacity     ResourceCapacity
}

// Endpoint represents a network endpoint in a region
type Endpoint struct {
	URL          string
	Type         EndpointType
	Weight       int
	HealthStatus HealthStatus
	Latency      time.Duration
	Location     GeoLocation
}

type EndpointType string

const (
	EndpointTypeAPI      EndpointType = "api"
	EndpointTypeDatabase EndpointType = "database"
	EndpointTypeCache    EndpointType = "cache"
	EndpointTypeCDN      EndpointType = "cdn"
	EndpointTypeEdge     EndpointType = "edge"
)

type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// GeoLocation represents geographical coordinates
type GeoLocation struct {
	Latitude  float64
	Longitude float64
	City      string
	Country   string
}

// LoadBalancingStrategy defines how traffic is distributed
type LoadBalancingStrategy struct {
	Algorithm    LoadBalancingAlgorithm
	StickySession bool
	HealthChecks bool
	Weights      map[string]int
}

type LoadBalancingAlgorithm string

const (
	RoundRobin        LoadBalancingAlgorithm = "round_robin"
	WeightedRoundRobin LoadBalancingAlgorithm = "weighted_round_robin"
	LeastConnections  LoadBalancingAlgorithm = "least_connections"
	LatencyBased      LoadBalancingAlgorithm = "latency_based"
	GeographyBased    LoadBalancingAlgorithm = "geography_based"
	WeightedLatency   LoadBalancingAlgorithm = "weighted_latency"
)

// FailoverPolicy defines failover behavior
type FailoverPolicy struct {
	EnableAutoFailover bool
	FailoverThreshold  time.Duration
	RetryAttempts      int
	RetryBackoff       time.Duration
	PrimaryRegion      string
	SecondaryRegions   []string
}

// LatencyThresholds define acceptable latency limits
type LatencyThresholds struct {
	Excellent time.Duration // < 50ms
	Good      time.Duration // < 100ms
	Acceptable time.Duration // < 200ms
	Poor      time.Duration // < 500ms
}

// GlobalNetworkOptimizer manages global network routing and optimization
type GlobalNetworkOptimizer struct {
	config         GlobalNetworkConfig
	logger         *zap.Logger
	latencyMonitor *LatencyMonitor
	routingTable   *RoutingTable
	metrics        *NetworkMetrics
}

// NewGlobalNetworkOptimizer creates a new global network optimizer
func NewGlobalNetworkOptimizer(config GlobalNetworkConfig, logger *zap.Logger) *GlobalNetworkOptimizer {
	return &GlobalNetworkOptimizer{
		config:         config,
		logger:         logger,
		latencyMonitor: NewLatencyMonitor(logger),
		routingTable:   NewRoutingTable(),
		metrics:        NewNetworkMetrics(),
	}
}

// OptimizeRouting optimizes network routing for a client request
func (gno *GlobalNetworkOptimizer) OptimizeRouting(ctx context.Context, clientRequest ClientRequest) (*RoutingDecision, error) {
	start := time.Now()
	defer func() {
		gno.metrics.RecordRoutingDecisionTime(time.Since(start))
	}()

	// Determine client location
	clientLocation, err := gno.determineClientLocation(clientRequest.ClientIP)
	if err != nil {
		gno.logger.Warn("Failed to determine client location", 
			zap.String("ip", clientRequest.ClientIP),
			zap.Error(err))
		clientLocation = &GeoLocation{} // Use default/unknown location
	}

	// Find optimal regions based on various factors
	regionCandidates := gno.findRegionCandidates(clientLocation, clientRequest.RequestType)

	// Score and rank regions
	rankedRegions := gno.rankRegions(clientLocation, regionCandidates, clientRequest)

	// Select the best region
	selectedRegion := gno.selectOptimalRegion(rankedRegions, clientRequest)

	// Create routing decision
	decision := &RoutingDecision{
		ClientIP:       clientRequest.ClientIP,
		ClientLocation: clientLocation,
		SelectedRegion: selectedRegion,
		Alternatives:   rankedRegions[:min(3, len(rankedRegions))], // Top 3 alternatives
		DecisionTime:   time.Now(),
		Reasoning:     gno.generateReasoningLog(selectedRegion, rankedRegions),
		TTL:           5 * time.Minute, // Cache routing decisions
	}

	gno.logger.Debug("Routing decision made",
		zap.String("client_ip", clientRequest.ClientIP),
		zap.String("selected_region", selectedRegion.Name),
		zap.Duration("decision_time", time.Since(start)))

	return decision, nil
}

// findRegionCandidates finds regions that can serve the request
func (gno *GlobalNetworkOptimizer) findRegionCandidates(clientLocation *GeoLocation, requestType RequestType) []RegionScore {
	candidates := make([]RegionScore, 0)

	for _, region := range gno.config.Regions {
		if !region.IsActive {
			continue
		}

		// Check if region has required endpoint type
		hasRequiredEndpoint := false
		for _, endpoint := range region.Endpoints {
			if gno.supportsRequestType(endpoint.Type, requestType) && 
			   endpoint.HealthStatus == HealthStatusHealthy {
				hasRequiredEndpoint = true
				break
			}
		}

		if !hasRequiredEndpoint {
			continue
		}

		// Calculate base score
		score := RegionScore{
			Region:      &region,
			TotalScore:  0,
			Scores:      make(map[string]float64),
		}

		candidates = append(candidates, score)
	}

	return candidates
}

// rankRegions ranks regions based on multiple criteria
func (gno *GlobalNetworkOptimizer) rankRegions(clientLocation *GeoLocation, candidates []RegionScore, request ClientRequest) []RegionScore {
	for i := range candidates {
		region := candidates[i].Region
		score := &candidates[i]

		// Geographic distance score (0-100, higher is better)
		distanceScore := gno.calculateDistanceScore(clientLocation, region)
		score.Scores["distance"] = distanceScore

		// Latency score (0-100, higher is better)
		latencyScore := gno.calculateLatencyScore(region, request.RequestType)
		score.Scores["latency"] = latencyScore

		// Capacity score (0-100, higher is better)
		capacityScore := gno.calculateCapacityScore(region)
		score.Scores["capacity"] = capacityScore

		// Health score (0-100, higher is better)
		healthScore := gno.calculateHealthScore(region)
		score.Scores["health"] = healthScore

		// Cost score (0-100, higher is better, lower cost is better)
		costScore := gno.calculateCostScore(region, request.RequestType)
		score.Scores["cost"] = costScore

		// Performance score based on historical data
		performanceScore := gno.calculatePerformanceScore(region, request.RequestType)
		score.Scores["performance"] = performanceScore

		// Calculate weighted total score
		score.TotalScore = gno.calculateWeightedScore(score.Scores, request.RequestType)
	}

	// Sort by total score (descending)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].TotalScore > candidates[j].TotalScore
	})

	return candidates
}

// calculateDistanceScore calculates score based on geographic distance
func (gno *GlobalNetworkOptimizer) calculateDistanceScore(clientLocation *GeoLocation, region *Region) float64 {
	if clientLocation.Latitude == 0 && clientLocation.Longitude == 0 {
		return 50.0 // Default score for unknown location
	}

	// Find closest endpoint in the region
	minDistance := float64(20000) // Max distance on Earth
	for _, endpoint := range region.Endpoints {
		distance := gno.calculateDistance(clientLocation, &endpoint.Location)
		if distance < minDistance {
			minDistance = distance
		}
	}

	// Convert distance to score (closer = higher score)
	// Assume max useful distance is 10,000km
	maxDistance := 10000.0
	if minDistance > maxDistance {
		minDistance = maxDistance
	}

	return (1.0 - (minDistance / maxDistance)) * 100.0
}

// calculateLatencyScore calculates score based on measured latency
func (gno *GlobalNetworkOptimizer) calculateLatencyScore(region *Region, requestType RequestType) float64 {
	avgLatency := gno.latencyMonitor.GetAverageLatency(region.Name, string(requestType))
	
	// Convert latency to score
	thresholds := gno.config.LatencyThresholds
	switch {
	case avgLatency <= thresholds.Excellent:
		return 100.0
	case avgLatency <= thresholds.Good:
		return 80.0
	case avgLatency <= thresholds.Acceptable:
		return 60.0
	case avgLatency <= thresholds.Poor:
		return 40.0
	default:
		return 20.0
	}
}

// calculateCapacityScore calculates score based on available capacity
func (gno *GlobalNetworkOptimizer) calculateCapacityScore(region *Region) float64 {
	capacity := region.Capacity
	
	// Calculate utilization percentage
	cpuUtil := float64(capacity.CPUUsed) / float64(capacity.CPUTotal)
	memUtil := float64(capacity.MemoryUsed) / float64(capacity.MemoryTotal)
	netUtil := float64(capacity.NetworkUsed) / float64(capacity.NetworkTotal)
	
	// Average utilization
	avgUtil := (cpuUtil + memUtil + netUtil) / 3.0
	
	// Convert to score (lower utilization = higher score)
	return (1.0 - avgUtil) * 100.0
}

// calculateHealthScore calculates score based on endpoint health
func (gno *GlobalNetworkOptimizer) calculateHealthScore(region *Region) float64 {
	totalEndpoints := len(region.Endpoints)
	if totalEndpoints == 0 {
		return 0.0
	}

	healthyCount := 0
	degradedCount := 0
	
	for _, endpoint := range region.Endpoints {
		switch endpoint.HealthStatus {
		case HealthStatusHealthy:
			healthyCount++
		case HealthStatusDegraded:
			degradedCount++
		}
	}

	// Calculate weighted health score
	healthyWeight := 1.0
	degradedWeight := 0.5
	
	weightedHealthy := float64(healthyCount) * healthyWeight
	weightedDegraded := float64(degradedCount) * degradedWeight
	
	return ((weightedHealthy + weightedDegraded) / float64(totalEndpoints)) * 100.0
}

// calculateCostScore calculates score based on operational cost
func (gno *GlobalNetworkOptimizer) calculateCostScore(region *Region, requestType RequestType) float64 {
	// Cost factors: compute, bandwidth, storage
	// This would typically come from cloud provider pricing APIs
	baseCost := gno.getRegionBaseCost(region.Name)
	requestCost := gno.getRequestTypeCost(string(requestType))
	
	totalCost := baseCost + requestCost
	
	// Normalize cost to score (lower cost = higher score)
	maxCost := 1.0 // Adjust based on actual cost ranges
	return (1.0 - (totalCost / maxCost)) * 100.0
}

// calculatePerformanceScore calculates score based on historical performance
func (gno *GlobalNetworkOptimizer) calculatePerformanceScore(region *Region, requestType RequestType) float64 {
	// Get historical performance metrics
	metrics := gno.metrics.GetRegionPerformance(region.Name, string(requestType))
	
	// Factors: success rate, average response time, throughput
	successRate := metrics.SuccessRate
	avgResponseTime := metrics.AverageResponseTime
	throughput := metrics.Throughput
	
	// Convert metrics to scores
	successScore := successRate * 100.0
	
	responseTimeScore := 100.0
	if avgResponseTime > gno.config.LatencyThresholds.Excellent {
		responseTimeScore = 100.0 * (gno.config.LatencyThresholds.Poor.Seconds() - avgResponseTime.Seconds()) / 
			gno.config.LatencyThresholds.Poor.Seconds()
	}
	
	throughputScore := min(throughput / 1000.0 * 100.0, 100.0) // Normalize to 1000 RPS = 100%
	
	// Weighted average
	return (successScore*0.5 + responseTimeScore*0.3 + throughputScore*0.2)
}

// calculateWeightedScore calculates the weighted total score
func (gno *GlobalNetworkOptimizer) calculateWeightedScore(scores map[string]float64, requestType RequestType) float64 {
	// Different weights for different request types
	weights := gno.getWeightsForRequestType(requestType)
	
	totalScore := 0.0
	for metric, score := range scores {
		if weight, exists := weights[metric]; exists {
			totalScore += score * weight
		}
	}
	
	return totalScore
}

// getWeightsForRequestType returns scoring weights for different request types
func (gno *GlobalNetworkOptimizer) getWeightsForRequestType(requestType RequestType) map[string]float64 {
	switch requestType {
	case RequestTypeAPI:
		return map[string]float64{
			"latency":     0.30,
			"health":      0.25,
			"capacity":    0.20,
			"distance":    0.15,
			"performance": 0.10,
		}
	case RequestTypeStaticContent:
		return map[string]float64{
			"distance":    0.35,
			"capacity":    0.25,
			"cost":        0.20,
			"health":      0.15,
			"latency":     0.05,
		}
	case RequestTypeDatabase:
		return map[string]float64{
			"latency":     0.35,
			"health":      0.30,
			"performance": 0.20,
			"capacity":    0.15,
		}
	default:
		// Balanced weights
		return map[string]float64{
			"latency":     0.25,
			"health":      0.20,
			"capacity":    0.20,
			"distance":    0.15,
			"performance": 0.15,
			"cost":        0.05,
		}
	}
}

// selectOptimalRegion selects the best region from ranked candidates
func (gno *GlobalNetworkOptimizer) selectOptimalRegion(rankedRegions []RegionScore, request ClientRequest) *Region {
	if len(rankedRegions) == 0 {
		return nil
	}

	// Consider failover policies
	if gno.config.FailoverPolicy.EnableAutoFailover {
		for _, candidate := range rankedRegions {
			if gno.isRegionAvailable(candidate.Region) {
				return candidate.Region
			}
		}
	}

	// Return the highest scored region
	return rankedRegions[0].Region
}

// Utility functions
func (gno *GlobalNetworkOptimizer) determineClientLocation(clientIP string) (*GeoLocation, error) {
	// This would typically use a GeoIP service like MaxMind
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", clientIP)
	}

	// Mock implementation - would query GeoIP database
	return &GeoLocation{
		Latitude:  37.7749,  // San Francisco
		Longitude: -122.4194,
		City:      "San Francisco",
		Country:   "US",
	}, nil
}

func (gno *GlobalNetworkOptimizer) calculateDistance(loc1, loc2 *GeoLocation) float64 {
	// Haversine formula to calculate distance between two points
	const earthRadius = 6371 // km

	lat1 := loc1.Latitude * (3.14159 / 180)
	lat2 := loc2.Latitude * (3.14159 / 180)
	deltaLat := (loc2.Latitude - loc1.Latitude) * (3.14159 / 180)
	deltaLng := (loc2.Longitude - loc1.Longitude) * (3.14159 / 180)

	a := 0.5 - 0.5*((lat2-lat1)/2) + 
		0.5*lat1*0.5*lat2*
		(1-((deltaLng)/2))

	return earthRadius * 2 * (a + (1-a))
}

func (gno *GlobalNetworkOptimizer) supportsRequestType(endpointType EndpointType, requestType RequestType) bool {
	switch requestType {
	case RequestTypeAPI:
		return endpointType == EndpointTypeAPI || endpointType == EndpointTypeEdge
	case RequestTypeDatabase:
		return endpointType == EndpointTypeDatabase
	case RequestTypeStaticContent:
		return endpointType == EndpointTypeCDN || endpointType == EndpointTypeEdge
	default:
		return true
	}
}

func (gno *GlobalNetworkOptimizer) isRegionAvailable(region *Region) bool {
	// Check if region meets availability criteria
	healthyEndpoints := 0
	for _, endpoint := range region.Endpoints {
		if endpoint.HealthStatus == HealthStatusHealthy {
			healthyEndpoints++
		}
	}
	
	// Require at least 50% healthy endpoints
	return float64(healthyEndpoints)/float64(len(region.Endpoints)) >= 0.5
}

func (gno *GlobalNetworkOptimizer) generateReasoningLog(selected *Region, alternatives []RegionScore) string {
	if selected == nil {
		return "No suitable region found"
	}
	
	reason := fmt.Sprintf("Selected %s (score: %.2f)", selected.Name, alternatives[0].TotalScore)
	if len(alternatives) > 1 {
		reason += fmt.Sprintf(", alternatives: %s (%.2f)", 
			alternatives[1].Region.Name, alternatives[1].TotalScore)
	}
	return reason
}

func (gno *GlobalNetworkOptimizer) getRegionBaseCost(regionName string) float64 {
	// Mock cost data - would come from cloud provider APIs
	costs := map[string]float64{
		"us-east-1":    0.10,
		"us-west-2":    0.12,
		"eu-west-1":    0.11,
		"ap-southeast-1": 0.13,
	}
	
	if cost, exists := costs[regionName]; exists {
		return cost
	}
	return 0.10 // Default cost
}

func (gno *GlobalNetworkOptimizer) getRequestTypeCost(requestType string) float64 {
	costs := map[string]float64{
		"api":            0.001,
		"static_content": 0.0001,
		"database":       0.01,
	}
	
	if cost, exists := costs[requestType]; exists {
		return cost
	}
	return 0.001 // Default cost
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Data structures
type ClientRequest struct {
	ClientIP    string
	UserAgent   string
	RequestType RequestType
	Path        string
	Headers     map[string]string
	Priority    int
}

type RequestType string

const (
	RequestTypeAPI           RequestType = "api"
	RequestTypeStaticContent RequestType = "static_content"
	RequestTypeDatabase      RequestType = "database"
	RequestTypeWebSocket     RequestType = "websocket"
)

type RoutingDecision struct {
	ClientIP       string
	ClientLocation *GeoLocation
	SelectedRegion *Region
	Alternatives   []RegionScore
	DecisionTime   time.Time
	Reasoning      string
	TTL            time.Duration
}

type RegionScore struct {
	Region     *Region
	TotalScore float64
	Scores     map[string]float64
}

type ResourceCapacity struct {
	CPUTotal     int64
	CPUUsed      int64
	MemoryTotal  int64
	MemoryUsed   int64
	NetworkTotal int64
	NetworkUsed  int64
}

type HealthCheckConfig struct {
	Enabled  bool
	Interval time.Duration
	Timeout  time.Duration
	Path     string
}

type CDNConfiguration struct {
	Provider       string
	DistributionID string
	CachePolicies  map[string]string
}

type DatabaseReplicationConfig struct {
	ReadReplicas  []string
	WriteRegion   string
	SyncMode      string
}

type EdgeComputingConfig struct {
	Enabled   bool
	Functions []EdgeFunction
}

type EdgeFunction struct {
	Name     string
	Runtime  string
	Code     string
	Triggers []string
}