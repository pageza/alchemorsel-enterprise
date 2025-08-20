# Enterprise AI Service Documentation

## Overview

The Enterprise AI Service provides production-ready AI features with comprehensive cost tracking, usage analytics, quality monitoring, and business intelligence for the Alchemorsel v3 platform.

## Features

### Core AI Features
- **Recipe Generation**: Create recipes from ingredients and preferences
- **Ingredient Suggestions**: Smart ingredient recommendations
- **Nutritional Analysis**: Comprehensive nutrition content analysis
- **Recipe Optimization**: Optimize recipes for health, cost, taste, or time
- **Dietary Adaptation**: Adapt recipes for dietary restrictions
- **Meal Planning**: Generate comprehensive meal plans with shopping lists

### Enterprise Features
- **Cost Tracking**: Real-time cost monitoring and budget management
- **Usage Analytics**: Detailed usage patterns and performance metrics
- **Rate Limiting**: Request throttling and quota management
- **Quality Monitoring**: AI response quality assessment and improvement
- **Business Intelligence**: Executive dashboards and insights
- **Alerting System**: Proactive alerts for issues and thresholds

## Architecture

### Components

```
Enterprise AI Service
├── Core AI Service Layer
│   ├── Recipe Generation
│   ├── Ingredient Suggestions
│   ├── Nutritional Analysis
│   ├── Recipe Optimization
│   └── Meal Planning
├── Cost Management
│   ├── Cost Tracker
│   ├── Budget Monitoring
│   └── Usage Attribution
├── Analytics & Monitoring
│   ├── Usage Analytics
│   ├── Performance Metrics
│   └── Quality Monitor
├── Operational
│   ├── Rate Limiter
│   ├── Alert Manager
│   └── Health Checks
└── Business Intelligence
    ├── Dashboard Data
    ├── Reporting
    └── Insights
```

### Provider Support
- **Primary**: Ollama (containerized, self-hosted)
- **Fallback**: OpenAI GPT-4
- **Mock**: For testing and development

## Configuration

### Enterprise Configuration

```yaml
ai_service:
  primary_provider: "ollama"
  fallback_providers: ["openai"]
  
  # Cost Management
  daily_budget_cents: 10000      # $100 daily budget
  monthly_budget_cents: 300000   # $3000 monthly budget
  cost_alert_thresholds: [0.7, 0.9, 1.0]
  
  # Rate Limiting
  requests_per_minute: 60
  requests_per_hour: 3600
  requests_per_day: 86400
  
  # Quality Control
  min_quality_score: 0.7
  quality_check_enabled: true
  
  # Caching
  cache_enabled: true
  cache_ttl: 2h
  
  # Features
  metrics_enabled: true
  alerts_enabled: true
```

### Model Configuration

```yaml
model_settings:
  llama3.2:3b:
    max_tokens: 2048
    temperature: 0.7
    top_p: 0.9
    cost_per_token: 0.001  # $0.001 per token
    request_timeout: 30s
    quality_weight: 1.0
  
  gpt-4:
    max_tokens: 4096
    temperature: 0.7
    top_p: 0.9
    cost_per_token: 0.03   # $0.03 per token
    request_timeout: 60s
    quality_weight: 1.2
```

## API Endpoints

### Recipe Generation

#### Generate Recipe
```http
POST /api/v1/ai/recipe/generate
Content-Type: application/json

{
  "prompt": "Create a healthy pasta dish with vegetables",
  "constraints": {
    "max_calories": 600,
    "dietary": ["vegetarian"],
    "cuisine": "italian",
    "serving_size": 4,
    "cooking_time": 30,
    "skill_level": "intermediate"
  },
  "user_id": "uuid"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "title": "Mediterranean Veggie Pasta",
    "description": "A colorful and nutritious pasta dish...",
    "ingredients": [
      {
        "name": "Whole wheat pasta",
        "amount": 12,
        "unit": "oz"
      }
    ],
    "instructions": [
      "Cook pasta according to package directions...",
      "Heat olive oil in a large skillet..."
    ],
    "nutrition": {
      "calories": 485,
      "protein": 18.5,
      "carbs": 58.0,
      "fat": 20.5,
      "fiber": 3.2,
      "sugar": 6.8,
      "sodium": 680.0
    },
    "tags": ["vegetarian", "mediterranean", "healthy"],
    "confidence": 0.92
  }
}
```

#### Ingredient Suggestions
```http
POST /api/v1/ai/ingredients/suggest
Content-Type: application/json

{
  "partial": ["tomatoes", "basil"],
  "cuisine": "italian",
  "dietary": ["vegetarian"],
  "user_id": "uuid"
}
```

#### Nutritional Analysis
```http
POST /api/v1/ai/nutrition/analyze
Content-Type: application/json

{
  "ingredients": ["chicken breast", "broccoli", "rice"],
  "servings": 2,
  "user_id": "uuid"
}
```

#### Recipe Optimization
```http
POST /api/v1/ai/recipe/optimize
Content-Type: application/json

{
  "recipe_id": "uuid",
  "optimization_type": "health",  // health, cost, taste, time
  "user_id": "uuid"
}
```

#### Dietary Adaptation
```http
POST /api/v1/ai/recipe/adapt
Content-Type: application/json

{
  "recipe_id": "uuid",
  "dietary_restrictions": ["vegan", "gluten-free"],
  "user_id": "uuid"
}
```

#### Meal Plan Generation
```http
POST /api/v1/ai/meal-plan/generate
Content-Type: application/json

{
  "days": 7,
  "dietary": ["vegetarian"],
  "budget": 150.00,
  "user_id": "uuid"
}
```

### Analytics & Monitoring

#### Cost Analytics
```http
GET /api/v1/ai/analytics/cost?period=monthly

Response:
{
  "success": true,
  "data": {
    "period": "monthly",
    "total_cost_cents": 187550,
    "cost_by_provider": {
      "ollama": 15000,
      "openai": 172550
    },
    "cost_by_feature": {
      "recipe_generation": 120000,
      "ingredient_suggestions": 30000,
      "nutrition_analysis": 25000,
      "optimization": 12550
    },
    "budget_utilization": 0.625,
    "projections": {
      "daily_projection": 6251.67,
      "weekly_projection": 43761.67,
      "monthly_projection": 187550,
      "confidence": 0.85
    }
  }
}
```

#### Usage Analytics
```http
GET /api/v1/ai/analytics/usage?period=daily

Response:
{
  "success": true,
  "data": {
    "period": "daily",
    "total_requests": 12847,
    "requests_by_type": {
      "recipe_generation": 5780,
      "ingredient_suggestions": 3200,
      "nutrition_analysis": 2890,
      "optimization": 977
    },
    "average_latency": "1.2s",
    "cache_hit_rate": 0.847,
    "error_rate": 0.023,
    "top_users": [
      {
        "user_id": "uuid",
        "request_count": 245,
        "total_cost": 15.75,
        "average_latency": "0.98s"
      }
    ]
  }
}
```

#### Quality Metrics
```http
GET /api/v1/ai/analytics/quality?period=daily

Response:
{
  "success": true,
  "data": {
    "period": "daily",
    "average_quality_score": 0.891,
    "quality_by_feature": {
      "recipe_generation": 0.895,
      "optimization": 0.878,
      "nutrition_analysis": 0.902
    },
    "low_quality_alerts": 12,
    "improvement_suggestions": [
      "Review prompts for recipe optimization feature",
      "Consider additional training data for edge cases"
    ]
  }
}
```

#### Rate Limit Status
```http
GET /api/v1/ai/rate-limit/status?user_id=uuid

Response:
{
  "success": true,
  "data": {
    "user_id": "uuid",
    "requests_this_minute": 15,
    "requests_this_hour": 245,
    "requests_this_day": 1847,
    "minute_limit": 60,
    "hour_limit": 3600,
    "day_limit": 86400,
    "minute_reset": "2024-01-15T10:45:00Z",
    "hour_reset": "2024-01-15T11:00:00Z",
    "day_reset": "2024-01-16T00:00:00Z",
    "is_limited": false
  }
}
```

### Business Intelligence

#### Dashboard Data
```http
GET /api/v1/ai/dashboard?period=daily

Response:
{
  "success": true,
  "data": {
    "cost_analytics": { /* cost data */ },
    "usage_analytics": { /* usage data */ },
    "quality_metrics": { /* quality data */ },
    "health_status": { /* health data */ },
    "period": "daily",
    "generated_at": "2024-01-15T10:30:00Z"
  }
}
```

#### Business Insights
```http
GET /api/v1/ai/insights?period=weekly

Response:
{
  "success": true,
  "data": {
    "period": "weekly",
    "revenue_impact": "High - AI features driving user engagement",
    "cost_efficiency": "Good - 85% cache hit rate reducing costs",
    "quality_trends": "Stable - Maintaining 90%+ quality scores",
    "user_adoption": "Growing - 25% increase in AI feature usage",
    "recommendations": [
      "Consider expanding recipe optimization features",
      "Investigate meal planning feature popularity",
      "Monitor cost growth trajectory"
    ],
    "kpis": {
      "ai_requests_per_day": 12500,
      "average_response_time": "1.2s",
      "cost_per_request": "$0.015",
      "user_satisfaction": 4.2,
      "feature_adoption_rate": "78%"
    }
  }
}
```

#### Generate Reports
```http
POST /api/v1/ai/reports/generate
Content-Type: application/json

{
  "report_type": "comprehensive",  // cost, usage, quality, comprehensive
  "start_date": "2024-01-01T00:00:00Z",
  "end_date": "2024-01-31T23:59:59Z",
  "format": "json"  // json, csv
}
```

### Configuration Management

#### Update Configuration
```http
PUT /api/v1/ai/config
Content-Type: application/json

{
  "daily_budget_cents": 15000,
  "monthly_budget_cents": 400000,
  "requests_per_minute": 100,
  "min_quality_score": 0.8,
  "cache_enabled": true
}
```

### Health Checks

#### Service Health
```http
GET /api/v1/ai/health

Response:
{
  "success": true,
  "data": {
    "service_name": "enterprise-ai-service",
    "status": "healthy",
    "timestamp": "2024-01-15T10:30:00Z",
    "components": {
      "cost_tracker": {
        "status": "healthy",
        "message": "Cost tracker operational"
      },
      "usage_analytics": {
        "status": "healthy",
        "message": "Usage analytics operational"
      },
      "rate_limiter": {
        "status": "healthy",
        "message": "Rate limiter operational"
      },
      "quality_monitor": {
        "status": "healthy",
        "message": "Quality monitor operational"
      },
      "ai_providers": {
        "status": "healthy",
        "message": "All providers operational"
      }
    },
    "version": "1.0.0",
    "uptime": "72h30m"
  }
}
```

## Cost Management

### Cost Calculation

The service calculates costs based on:
- **Token Usage**: Input and output tokens with provider-specific rates
- **Request Base Cost**: Per-request overhead
- **Volume Discounts**: Automatic discounts for high-volume users
- **Provider Rates**: Different rates for different AI providers

### Budget Management

- **Daily Budgets**: Configurable daily spending limits
- **Monthly Budgets**: Configurable monthly spending limits
- **Alert Thresholds**: Proactive alerts at 70%, 90%, and 100% of budget
- **Automatic Cutoffs**: Service stops when budget is exceeded

### Cost Optimization

- **Caching**: 2-hour TTL reduces repeated AI calls
- **Smart Routing**: Route to most cost-effective provider
- **Quality-Cost Balance**: Optimize for both quality and cost
- **Usage Patterns**: Analyze and optimize based on usage patterns

## Quality Assurance

### Quality Assessment

The service assesses quality across multiple dimensions:

1. **Completeness**: Are all required fields present?
2. **Clarity**: Are instructions clear and understandable?
3. **Practicality**: Are quantities and steps realistic?
4. **Safety**: Are food safety guidelines followed?
5. **Nutrition**: Are nutritional values accurate?
6. **Format**: Is the response properly formatted?

### Quality Monitoring

- **Real-time Assessment**: Every response is assessed
- **Quality Trends**: Track quality over time
- **Alert System**: Alerts when quality drops below thresholds
- **Improvement Suggestions**: Automated recommendations

## Rate Limiting

### Limits

- **Per Minute**: 60 requests (configurable)
- **Per Hour**: 3,600 requests (configurable)
- **Per Day**: 86,400 requests (configurable)
- **Quota**: Monthly request quotas per user

### Enforcement

- **Sliding Windows**: Precise rate limiting using sliding windows
- **User Isolation**: Per-user rate limiting
- **Progressive Penalties**: Temporary blocks for violations
- **Admin Override**: Manual quota adjustments

## Error Handling

### Error Response Format

```json
{
  "success": false,
  "error": "Error message description",
  "error_code": "RATE_LIMIT_EXCEEDED",
  "details": {
    "current_usage": 61,
    "limit": 60,
    "reset_time": "2024-01-15T10:45:00Z"
  }
}
```

### Common Error Codes

- `RATE_LIMIT_EXCEEDED`: Request rate limit exceeded
- `BUDGET_EXCEEDED`: Daily or monthly budget exceeded
- `QUALITY_THRESHOLD`: Response quality below threshold
- `INVALID_REQUEST`: Malformed request data
- `PROVIDER_ERROR`: AI provider unavailable
- `INTERNAL_ERROR`: Internal service error

## Monitoring & Alerting

### Metrics Collected

- **Request Metrics**: Count, latency, error rate
- **Cost Metrics**: Spending by user, provider, feature
- **Quality Metrics**: Score distribution, trends
- **Performance Metrics**: Response times, throughput
- **System Metrics**: Health, availability, errors

### Alert Types

1. **Cost Alerts**: Budget threshold breaches
2. **Quality Alerts**: Quality score drops
3. **Performance Alerts**: High latency or errors
4. **System Alerts**: Component failures

### Notification Channels

- **Email**: Executive reports and critical alerts
- **Webhook**: Integration with external systems
- **Slack**: Team notifications (configurable)

## Development & Testing

### Running Tests

```bash
# Run unit tests
go test ./internal/application/ai/...

# Run integration tests
go test -tags integration ./internal/application/ai/...

# Run benchmarks
go test -bench=. ./internal/application/ai/...

# Test coverage
go test -cover ./internal/application/ai/...
```

### Mock Services

The test suite includes comprehensive mocks for:
- Cache repository
- AI providers
- External services

### Load Testing

Use the provided benchmarks to test performance:

```bash
# Benchmark recipe generation
go test -bench=BenchmarkGenerateRecipe -benchtime=10s

# Benchmark cost tracking
go test -bench=BenchmarkCostTracking -benchtime=10s

# Benchmark quality assessment
go test -bench=BenchmarkQualityAssessment -benchtime=10s
```

## Deployment

### Environment Variables

```bash
# AI Service Configuration
ALCHEMORSEL_AI_PRIMARY_PROVIDER=ollama
ALCHEMORSEL_AI_FALLBACK_PROVIDERS=openai

# Budget Configuration
ALCHEMORSEL_AI_DAILY_BUDGET_CENTS=10000
ALCHEMORSEL_AI_MONTHLY_BUDGET_CENTS=300000

# Rate Limiting
ALCHEMORSEL_AI_REQUESTS_PER_MINUTE=60
ALCHEMORSEL_AI_REQUESTS_PER_HOUR=3600

# Quality Control
ALCHEMORSEL_AI_MIN_QUALITY_SCORE=0.7
ALCHEMORSEL_AI_QUALITY_CHECK_ENABLED=true

# Features
ALCHEMORSEL_AI_CACHE_ENABLED=true
ALCHEMORSEL_AI_METRICS_ENABLED=true
ALCHEMORSEL_AI_ALERTS_ENABLED=true
```

### Docker Integration

The service integrates with the existing Docker Compose setup:

```yaml
services:
  enterprise-ai:
    build:
      context: .
      dockerfile: Dockerfile
    environment:
      - ALCHEMORSEL_AI_PRIMARY_PROVIDER=ollama
      - ALCHEMORSEL_AI_CACHE_ENABLED=true
    depends_on:
      - ollama
      - redis
      - postgres
```

### Health Checks

Docker health checks are included:

```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/api/v1/ai/health || exit 1
```

## Security

### API Security

- **Authentication**: JWT-based user authentication
- **Authorization**: Role-based access control
- **Rate Limiting**: Protection against abuse
- **Input Validation**: Comprehensive input sanitization

### Data Protection

- **PII Handling**: Careful handling of personal data
- **Data Retention**: Configurable data retention policies
- **Encryption**: Data encrypted in transit and at rest
- **Audit Logging**: Comprehensive audit trails

## Performance

### Optimization Strategies

1. **Caching**: Multi-layer caching strategy
2. **Connection Pooling**: Efficient database connections
3. **Asynchronous Processing**: Non-blocking operations
4. **Smart Routing**: Optimal provider selection

### Performance Targets

- **Response Time**: < 2 seconds (95th percentile)
- **Throughput**: 1000+ requests/second
- **Availability**: 99.9% uptime
- **Cache Hit Rate**: > 80%

## Troubleshooting

### Common Issues

1. **High Costs**: Check budget alerts, review usage patterns
2. **Poor Quality**: Review quality metrics, adjust thresholds
3. **Rate Limits**: Check user quotas, adjust limits
4. **Performance**: Monitor latency, check provider health

### Debug Endpoints

```bash
# Check service health
curl http://localhost:8080/api/v1/ai/health

# Check rate limit status
curl http://localhost:8080/api/v1/ai/rate-limit/status?user_id=uuid

# Check cost analytics
curl http://localhost:8080/api/v1/ai/analytics/cost?period=daily
```

### Log Analysis

Key log patterns to monitor:

```bash
# High error rates
grep "ERROR" logs/enterprise-ai.log | tail -100

# Rate limit violations
grep "rate limit exceeded" logs/enterprise-ai.log

# Quality alerts
grep "quality alert" logs/enterprise-ai.log

# Budget alerts
grep "budget.*exceeded" logs/enterprise-ai.log
```

## Roadmap

### Planned Features

1. **Advanced Analytics**: ML-powered usage predictions
2. **Custom Models**: Support for custom-trained models
3. **A/B Testing**: Built-in A/B testing framework
4. **Multi-tenancy**: Enhanced multi-tenant support
5. **Advanced Caching**: Semantic caching capabilities

### Integration Plans

1. **Business Intelligence**: Advanced BI dashboards
2. **External APIs**: Third-party recipe databases
3. **Mobile SDK**: Native mobile integration
4. **Voice Interface**: Voice-activated recipe assistance

## Support

### Documentation

- [API Reference](./API_REFERENCE.md)
- [Configuration Guide](./CONFIGURATION.md)
- [Deployment Guide](./DEPLOYMENT.md)
- [Troubleshooting Guide](./TROUBLESHOOTING.md)

### Contact

For technical support or questions:
- Email: support@alchemorsel.com
- Slack: #ai-service-support
- Documentation: https://docs.alchemorsel.com