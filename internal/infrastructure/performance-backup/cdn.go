package performance

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudfront"
	"go.uber.org/zap"
)

// CDNConfig holds CDN configuration
type CDNConfig struct {
	Provider       string            // "cloudfront", "cloudflare", "fastly"
	DistributionID string            // CloudFront distribution ID
	Domain         string            // CDN domain
	Origins        map[string]string // Origin mappings
	CachePolicies  map[string]CachePolicy
	SecurityPolicy SecurityPolicy
}

// CachePolicy defines caching behavior for different content types
type CachePolicy struct {
	TTL             time.Duration
	MaxTTL          time.Duration
	Headers         []string // Headers to include in cache key
	QueryStrings    []string // Query parameters to include in cache key
	Cookies         []string // Cookies to include in cache key
	CompressObjects bool
}

// SecurityPolicy defines security settings for CDN
type SecurityPolicy struct {
	EnableWAF       bool
	AllowedMethods  []string
	AllowedOrigins  []string
	SecurityHeaders map[string]string
}

// CDNManager manages CDN operations
type CDNManager struct {
	config     CDNConfig
	cloudfront *cloudfront.CloudFront
	logger     *zap.Logger
}

// NewCDNManager creates a new CDN manager
func NewCDNManager(config CDNConfig, logger *zap.Logger) (*CDNManager, error) {
	var cf *cloudfront.CloudFront

	if config.Provider == "cloudfront" {
		sess, err := session.NewSession(&aws.Config{
			Region: aws.String("us-east-1"), // CloudFront is global but requires us-east-1
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create AWS session: %w", err)
		}
		cf = cloudfront.New(sess)
	}

	return &CDNManager{
		config:     config,
		cloudfront: cf,
		logger:     logger,
	}, nil
}

// InvalidateCache invalidates CDN cache for specified paths
func (c *CDNManager) InvalidateCache(ctx context.Context, paths []string) (*InvalidationResult, error) {
	switch c.config.Provider {
	case "cloudfront":
		return c.invalidateCloudFront(ctx, paths)
	case "cloudflare":
		return c.invalidateCloudflare(ctx, paths)
	default:
		return nil, fmt.Errorf("unsupported CDN provider: %s", c.config.Provider)
	}
}

// invalidateCloudFront creates CloudFront invalidation
func (c *CDNManager) invalidateCloudFront(ctx context.Context, paths []string) (*InvalidationResult, error) {
	callerReference := fmt.Sprintf("alchemorsel-%d", time.Now().Unix())

	// Convert paths to CloudFront format
	cfPaths := make([]*string, len(paths))
	for i, path := range paths {
		if path[0] != '/' {
			path = "/" + path
		}
		cfPaths[i] = aws.String(path)
	}

	input := &cloudfront.CreateInvalidationInput{
		DistributionId: aws.String(c.config.DistributionID),
		InvalidationBatch: &cloudfront.InvalidationBatch{
			CallerReference: aws.String(callerReference),
			Paths: &cloudfront.Paths{
				Quantity: aws.Int64(int64(len(cfPaths))),
				Items:    cfPaths,
			},
		},
	}

	result, err := c.cloudfront.CreateInvalidationWithContext(ctx, input)
	if err != nil {
		c.logger.Error("CloudFront invalidation failed", 
			zap.Strings("paths", paths), 
			zap.Error(err))
		return nil, err
	}

	c.logger.Info("CloudFront invalidation created",
		zap.String("invalidation_id", *result.Invalidation.Id),
		zap.Strings("paths", paths))

	return &InvalidationResult{
		ID:     *result.Invalidation.Id,
		Status: *result.Invalidation.Status,
		Paths:  paths,
	}, nil
}

// invalidateCloudflare creates Cloudflare cache purge
func (c *CDNManager) invalidateCloudflare(ctx context.Context, paths []string) (*InvalidationResult, error) {
	// Cloudflare implementation would go here
	return nil, fmt.Errorf("cloudflare invalidation not implemented")
}

// GetCacheStatistics retrieves CDN cache performance statistics
func (c *CDNManager) GetCacheStatistics(ctx context.Context, startTime, endTime time.Time) (*CacheStatistics, error) {
	switch c.config.Provider {
	case "cloudfront":
		return c.getCloudFrontStatistics(ctx, startTime, endTime)
	default:
		return nil, fmt.Errorf("statistics not supported for provider: %s", c.config.Provider)
	}
}

// getCloudFrontStatistics retrieves CloudFront statistics
func (c *CDNManager) getCloudFrontStatistics(ctx context.Context, startTime, endTime time.Time) (*CacheStatistics, error) {
	// This would typically integrate with CloudWatch to get CloudFront metrics
	// For now, return mock data
	return &CacheStatistics{
		CacheHitRatio:    0.85,
		TotalRequests:    100000,
		CacheHits:        85000,
		CacheMisses:      15000,
		OriginRequests:   15000,
		BytesServed:      1024 * 1024 * 500, // 500MB
		AverageLatency:   time.Millisecond * 50,
		ErrorRate:        0.001,
	}, nil
}

// PurgeAllCache purges entire CDN cache
func (c *CDNManager) PurgeAllCache(ctx context.Context) error {
	return c.InvalidateCache(ctx, []string{"/*"})
}

// PurgeByTags purges cache by tags (if supported by CDN provider)
func (c *CDNManager) PurgeByTags(ctx context.Context, tags []string) error {
	switch c.config.Provider {
	case "cloudflare":
		// Cloudflare supports purge by tags
		return c.purgeCloudflareByTags(ctx, tags)
	default:
		return fmt.Errorf("purge by tags not supported for provider: %s", c.config.Provider)
	}
}

// purgeCloudflareByTags purges Cloudflare cache by tags
func (c *CDNManager) purgeCloudflareByTags(ctx context.Context, tags []string) error {
	// Cloudflare tag-based purge implementation would go here
	return fmt.Errorf("cloudflare tag purge not implemented")
}

// SetCacheHeaders sets appropriate cache headers for responses
func (c *CDNManager) SetCacheHeaders(contentType string, path string) map[string]string {
	headers := make(map[string]string)

	// Get cache policy for content type
	policy := c.getCachePolicyForContent(contentType, path)

	// Set Cache-Control header
	cacheControl := fmt.Sprintf("public, max-age=%d", int(policy.TTL.Seconds()))
	if policy.MaxTTL > 0 {
		cacheControl += fmt.Sprintf(", s-maxage=%d", int(policy.MaxTTL.Seconds()))
	}
	headers["Cache-Control"] = cacheControl

	// Set ETag for cache validation
	headers["ETag"] = fmt.Sprintf("\"%x\"", time.Now().Unix())

	// Set Vary header if needed
	if len(policy.Headers) > 0 {
		vary := "Accept-Encoding"
		for _, header := range policy.Headers {
			vary += ", " + header
		}
		headers["Vary"] = vary
	}

	// Add security headers if configured
	for key, value := range c.config.SecurityPolicy.SecurityHeaders {
		headers[key] = value
	}

	return headers
}

// getCachePolicyForContent determines cache policy based on content type and path
func (c *CDNManager) getCachePolicyForContent(contentType, path string) CachePolicy {
	// Check for specific path-based policies
	for pattern, policy := range c.config.CachePolicies {
		if matchPath(path, pattern) {
			return policy
		}
	}

	// Default policies based on content type
	switch {
	case isStaticAsset(contentType):
		return CachePolicy{
			TTL:             24 * time.Hour,
			MaxTTL:          7 * 24 * time.Hour,
			CompressObjects: true,
		}
	case isAPIResponse(contentType):
		return CachePolicy{
			TTL:             5 * time.Minute,
			MaxTTL:          1 * time.Hour,
			Headers:         []string{"Authorization"},
			CompressObjects: true,
		}
	case isHTML(contentType):
		return CachePolicy{
			TTL:             1 * time.Hour,
			MaxTTL:          24 * time.Hour,
			CompressObjects: true,
		}
	default:
		return CachePolicy{
			TTL:             1 * time.Hour,
			MaxTTL:          24 * time.Hour,
			CompressObjects: true,
		}
	}
}

// Edge computing functions for CDN
type EdgeFunction struct {
	Name        string
	Code        string
	Triggers    []string // Request paths that trigger this function
	Runtime     string   // "javascript", "wasm", etc.
	MemoryLimit int      // Memory limit in MB
	Timeout     time.Duration
}

// DeployEdgeFunction deploys function to CDN edge locations
func (c *CDNManager) DeployEdgeFunction(ctx context.Context, function EdgeFunction) error {
	switch c.config.Provider {
	case "cloudflare":
		return c.deployCloudflareWorker(ctx, function)
	case "cloudfront":
		return c.deployCloudFrontFunction(ctx, function)
	default:
		return fmt.Errorf("edge functions not supported for provider: %s", c.config.Provider)
	}
}

// deployCloudflareWorker deploys Cloudflare Worker
func (c *CDNManager) deployCloudflareWorker(ctx context.Context, function EdgeFunction) error {
	// Cloudflare Workers API implementation would go here
	c.logger.Info("Deploying Cloudflare Worker", zap.String("function", function.Name))
	return nil
}

// deployCloudFrontFunction deploys CloudFront Function
func (c *CDNManager) deployCloudFrontFunction(ctx context.Context, function EdgeFunction) error {
	// CloudFront Functions API implementation would go here
	c.logger.Info("Deploying CloudFront Function", zap.String("function", function.Name))
	return nil
}

// CDN optimization strategies
type OptimizationStrategy struct {
	ImageOptimization    ImageOptimization
	ContentOptimization  ContentOptimization
	PrefetchingStrategy  PrefetchingStrategy
}

type ImageOptimization struct {
	EnableWebP      bool
	EnableAVIF      bool
	QualitySettings map[string]int // device type -> quality
	ResponsiveImages bool
}

type ContentOptimization struct {
	MinifyHTML       bool
	MinifyCSS        bool
	MinifyJavaScript bool
	CombineFiles     bool
	RemoveComments   bool
}

type PrefetchingStrategy struct {
	DNSPrefetch    []string // Domains to prefetch DNS for
	Preconnect     []string // Origins to preconnect to
	ResourceHints  []ResourceHint
}

type ResourceHint struct {
	URL  string
	Type string // "preload", "prefetch", "prerender"
	As   string // Resource type: "script", "style", "image", etc.
}

// ApplyOptimizations applies CDN optimizations
func (c *CDNManager) ApplyOptimizations(strategy OptimizationStrategy) error {
	c.logger.Info("Applying CDN optimizations")

	// Implementation would configure CDN optimization settings
	// This is provider-specific and would involve API calls to configure:
	// - Image optimization settings
	// - Content minification
	// - Prefetching rules
	// - Compression settings

	return nil
}

// Data structures
type InvalidationResult struct {
	ID     string
	Status string
	Paths  []string
}

type CacheStatistics struct {
	CacheHitRatio    float64
	TotalRequests    int64
	CacheHits        int64
	CacheMisses      int64
	OriginRequests   int64
	BytesServed      int64
	AverageLatency   time.Duration
	ErrorRate        float64
}

// Utility functions
func matchPath(path, pattern string) bool {
	// Simple pattern matching (could be enhanced with regex)
	if pattern == "/*" {
		return true
	}
	return path == pattern
}

func isStaticAsset(contentType string) bool {
	return contentType == "image/jpeg" ||
		contentType == "image/png" ||
		contentType == "image/gif" ||
		contentType == "image/webp" ||
		contentType == "text/css" ||
		contentType == "application/javascript" ||
		contentType == "font/woff" ||
		contentType == "font/woff2"
}

func isAPIResponse(contentType string) bool {
	return contentType == "application/json" ||
		contentType == "application/xml"
}

func isHTML(contentType string) bool {
	return contentType == "text/html"
}

// CDN middleware for adding appropriate headers
func CDNHeadersMiddleware(cdnManager *CDNManager) func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Next() // Process the request first

		// Get content type from response
		contentType := c.Writer.Header().Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		// Set CDN cache headers
		headers := cdnManager.SetCacheHeaders(contentType, c.Request.URL.Path)
		for key, value := range headers {
			c.Writer.Header().Set(key, value)
		}
	}
}