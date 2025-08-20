# Alchemorsel v3 - Network Optimization & Native Mobile Strategy

## Executive Summary

This document outlines the network optimization strategy and native mobile development plan for Alchemorsel v3, designed for AI-assisted development using Claude Code. The strategy emphasizes building backwards from the end goal: high-performance native iOS and Android applications supported by an optimized Go backend architecture.

### Key Decisions
- **Native Mobile Development**: iOS (SwiftUI) and Android (Jetpack Compose) for optimal performance
- **Network-First Design**: API optimization for mobile clients from day one  
- **Scalable Architecture**: Grassroots (100 users) to enterprise (100K+ users) without architectural changes
- **AI Development**: Leveraging Claude Code for sophisticated native implementations

---

## Network Architecture Strategy

### API Design Principles

#### Multi-Client API Structure
```
/api/v1/
├── ios/          # iOS-optimized endpoints with Core Data sync support
├── android/      # Android-optimized endpoints with Room integration
├── mobile/       # Shared mobile optimizations (smaller payloads)
├── web/          # HTMX web interface endpoints
└── shared/       # Common functionality across all platforms
```

#### Response Optimization Patterns

**Progressive Data Loading:**
- Recipe basics first: `GET /api/v1/mobile/recipes/{id}/summary`
- Full details on demand: `GET /api/v1/mobile/recipes/{id}/full`
- Images separately: `GET /api/v1/mobile/recipes/{id}/images`

**Batch Operations:**
```go
// Single call for recipe + user context
GET /api/v1/mobile/recipes/{id}?include=rating,favorite,nutrition
```

**Field Selection:**
```go
// Sparse fieldsets for mobile bandwidth optimization
GET /api/v1/mobile/recipes?fields=id,title,image,cookTime,difficulty
```

### Caching Strategy

#### Three-Tier Caching Architecture

1. **Application Level (Go)**: In-memory LRU cache for hot data
2. **Redis Layer**: User sessions, API responses, search results
3. **Client Level**: Native platform caches (URLCache, OkHttp)

#### Cache TTL Strategy
- **Popular recipes**: 4 hours
- **User preferences**: 24 hours  
- **Search results**: 1 hour
- **User-generated content**: 15 minutes

### Performance Targets

#### API Response Times
- **Recipe listing**: <100ms (p95)
- **Recipe details**: <150ms (p95)
- **Search results**: <200ms (p95)
- **User actions** (favorite, rate): <50ms (p95)

#### Data Transfer Optimization
- **JSON compression**: gzip/brotli (70-80% reduction)
- **Image optimization**: WebP/AVIF with multiple resolutions
- **Payload sizes**: <50KB for recipe lists, <200KB for full recipe details

---

## Native Mobile Development Plan

### iOS Implementation Strategy

#### Core Technologies
- **UI Framework**: SwiftUI for modern, reactive interfaces
- **Networking**: URLSession with HTTP/2, connection reuse
- **Local Storage**: Core Data with CloudKit sync capability
- **Reactive Programming**: Combine for data flow management
- **Image Handling**: SDWebImage for advanced caching

#### iOS-Specific Network Optimizations
```swift
// HTTP/2 connection reuse and caching
let session = URLSession(configuration: .default)
session.configuration.urlCache = URLCache(memoryCapacity: 50MB, diskCapacity: 200MB)
session.configuration.httpMaximumConnectionsPerHost = 4

// Background refresh for recipe sync
func scheduleBackgroundRefresh() {
    let request = BGAppRefreshTaskRequest(identifier: "recipe-sync")
    request.earliestBeginDate = Date(timeIntervalSinceNow: 15 * 60)
    try? BGTaskScheduler.shared.submit(request)
}
```

#### Offline-First Architecture
- **Core Data**: Local recipe storage with sync timestamps
- **Background sync**: Recipe updates, user preferences
- **Conflict resolution**: Last-write-wins with user notification
- **Image caching**: Intelligent prefetching based on user behavior

### Android Implementation Strategy  

#### Core Technologies
- **UI Framework**: Jetpack Compose for modern Android UI
- **Networking**: Retrofit + OkHttp with connection pooling
- **Local Storage**: Room database with reactive queries
- **Background Processing**: WorkManager for sync operations
- **Image Loading**: Glide with memory/disk caching

#### Android-Specific Network Optimizations
```kotlin
// OkHttp client with advanced caching
val client = OkHttpClient.Builder()
    .cache(Cache(File(cacheDir, "http"), 50L * 1024L * 1024L)) // 50MB
    .connectionPool(ConnectionPool(5, 30, TimeUnit.SECONDS))
    .addInterceptor(HttpLoggingInterceptor())
    .build()

// Network-aware image loading
Glide.with(context)
    .load(recipe.imageUrl)
    .apply(when (networkQuality) {
        NetworkQuality.POOR -> RequestOptions().override(300, 200)
        NetworkQuality.GOOD -> RequestOptions().override(800, 600)
        else -> RequestOptions()
    })
```

#### Background Synchronization
```kotlin
// WorkManager for reliable background sync
class RecipeSyncWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {
    override suspend fun doWork(): Result {
        return try {
            syncRecipes()
            Result.success()
        } catch (exception: Exception) {
            Result.retry()
        }
    }
}
```

---

## Scaling Strategy: Grassroots to Enterprise

### Phase 1: Grassroots (0-100 users)
**Infrastructure:**
- Single Go server instance
- PostgreSQL with basic indexing
- Redis for session management
- Basic monitoring with Prometheus

**Network Optimizations:**
- Response compression middleware
- Database connection pooling (25-50 connections)
- Basic rate limiting per user
- HTMX web interface for SEO/sharing

### Phase 2: Growth (100-1K users)
**Infrastructure:**
- Horizontal scaling with load balancer
- Database read replicas
- CDN for static assets (images, CSS)
- Enhanced monitoring and alerting

**Network Optimizations:**
- Advanced caching strategies
- API response versioning
- Mobile app launch (iOS/Android)
- Push notification infrastructure

### Phase 3: Scale (1K-10K users)
**Infrastructure:**
- Microservices extraction (optional)
- Database sharding considerations
- Geographic distribution (multi-region)
- Advanced observability (Jaeger tracing)

**Network Optimizations:**
- Edge caching for API responses
- Real-time features (WebSocket/SSE)
- Advanced image optimization
- Predictive prefetching

### Phase 4: Enterprise (10K+ users)
**Infrastructure:**
- Full microservices architecture
- Event-driven communication
- Auto-scaling infrastructure
- Advanced security and compliance

**Network Optimizations:**
- GraphQL for sophisticated client queries
- HTTP/3 where beneficial
- Advanced AI-driven prefetching
- Global edge computing

---

## Performance Targets & Monitoring

### Key Performance Indicators

#### Network Performance
- **API Latency**: p50 <50ms, p95 <200ms, p99 <500ms
- **Time to Interactive**: Mobile apps <2s, Web <3s
- **Image Load Time**: <1s for recipe photos
- **Offline Capability**: Core features work without network

#### User Experience Metrics
- **Recipe Browse**: <1s to display recipe list
- **Recipe Detail**: <1.5s to full recipe display
- **Search Results**: <2s for query results
- **Sync Time**: <5s for background user data sync

### Monitoring Implementation

#### Backend Metrics (Prometheus)
```go
// Custom metrics for network optimization
var (
    apiRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "api_request_duration_seconds",
            Help: "API request duration in seconds",
        },
        []string{"endpoint", "method", "status", "client_type"},
    )
    
    payloadSize = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "api_response_size_bytes",
            Help: "API response payload size in bytes",
        },
        []string{"endpoint", "client_type"},
    )
)
```

#### Client-Side Monitoring
- **iOS**: Use MetricKit for performance tracking
- **Android**: Firebase Performance Monitoring
- **Web**: Browser Performance API with custom metrics

---

## Implementation Phases

### Phase 1: Backend API Optimization (Weeks 1-2)
**Tasks:**
1. Add response compression middleware (gzip/brotli)
2. Implement multi-format API responses (JSON/HTML)
3. Add comprehensive request/response logging
4. Implement rate limiting and basic security
5. Database query optimization and indexing

**Deliverables:**
- Optimized API endpoints for mobile consumption
- Performance monitoring baseline
- Security hardening complete

### Phase 2: iOS Native Development (Weeks 3-5)
**Tasks:**
1. SwiftUI app architecture with MVVM pattern
2. URLSession networking layer with caching
3. Core Data integration for offline storage
4. Background sync implementation
5. Image caching and optimization

**Deliverables:**
- iOS app with full recipe browsing capability
- Offline-first architecture implemented
- App Store submission ready

### Phase 3: Android Native Development (Weeks 6-8)
**Tasks:**
1. Jetpack Compose UI implementation
2. Retrofit + OkHttp networking layer
3. Room database for local storage
4. WorkManager for background processing
5. Glide image loading optimization

**Deliverables:**
- Android app feature parity with iOS
- Advanced caching and sync capabilities
- Google Play Store submission ready

### Phase 4: Advanced Features (Weeks 9-12)
**Tasks:**
1. Push notification system (APNs + FCM)
2. Real-time features (WebSocket/SSE)
3. Advanced image processing and AR features
4. Social sharing and deep linking
5. Analytics and performance optimization

**Deliverables:**
- Production-ready native applications
- Comprehensive analytics dashboard
- Advanced user engagement features

---

## Technical Specifications

### API Response Formats

#### Mobile-Optimized Recipe List
```json
{
    "recipes": [
        {
            "id": "uuid",
            "title": "string",
            "image_url": "string",
            "image_thumb": "string", 
            "cook_time": "int (minutes)",
            "difficulty": "enum (easy|medium|hard)",
            "rating": "float",
            "is_favorite": "bool"
        }
    ],
    "pagination": {
        "cursor": "string",
        "has_next": "bool"
    }
}
```

#### Full Recipe Detail
```json
{
    "id": "uuid",
    "title": "string",
    "description": "string",
    "images": [
        {
            "url": "string",
            "width": "int",
            "height": "int",
            "format": "string"
        }
    ],
    "ingredients": [
        {
            "name": "string",
            "amount": "string",
            "unit": "string"
        }
    ],
    "instructions": [
        {
            "step": "int",
            "description": "string",
            "image_url": "string?"
        }
    ],
    "nutrition": {
        "calories": "int",
        "protein": "float",
        "carbs": "float",
        "fat": "float"
    },
    "metadata": {
        "created_at": "timestamp",
        "updated_at": "timestamp",
        "version": "int"
    }
}
```

### Caching Headers Strategy
```http
# For recipe data
Cache-Control: public, max-age=3600
ETag: "version-hash"
Last-Modified: "timestamp"

# For images
Cache-Control: public, max-age=86400
Content-Encoding: webp

# For user-specific data  
Cache-Control: private, max-age=300
Vary: Authorization
```

### Database Optimization

#### Essential Indexes
```sql
-- Recipe search optimization
CREATE INDEX idx_recipes_search ON recipes USING gin(to_tsvector('english', title || ' ' || description));

-- User favorites lookup
CREATE INDEX idx_user_favorites ON user_favorites(user_id, recipe_id);

-- Recipe filtering
CREATE INDEX idx_recipes_filter ON recipes(difficulty, cook_time, created_at);

-- User session management
CREATE INDEX idx_user_sessions ON user_sessions(user_id, expires_at);
```

#### Connection Pool Configuration
```go
config := pgxpool.Config{
    MaxConns:        30,
    MinConns:        5,
    MaxConnLifetime: time.Hour,
    MaxConnIdleTime: time.Minute * 30,
}
```

---

## Security Considerations

### API Security
- **JWT Authentication**: Access tokens (15min) + Refresh tokens (7 days)
- **Rate Limiting**: 1000 requests/hour per user, 10 requests/second burst
- **Input Validation**: Comprehensive request validation with go-playground/validator
- **CORS Configuration**: Strict origin policies for web clients

### Mobile Security
- **Certificate Pinning**: Pin API certificates in mobile apps
- **Biometric Authentication**: TouchID/FaceID and fingerprint/face unlock
- **Secure Storage**: Keychain (iOS) and KeyStore (Android) for tokens
- **Network Security**: TLS 1.3 minimum, HSTS enforcement

---

## Conclusion

This network optimization and native mobile strategy provides a comprehensive roadmap for building a high-performance, scalable recipe management platform. The approach emphasizes:

1. **Performance-First**: Network optimization built into the architecture
2. **Scale-Ready**: Designed to handle growth without major rewrites
3. **Platform-Native**: Leveraging iOS and Android strengths for optimal UX
4. **AI-Friendly**: Clear specifications for Claude Code implementation

The hexagonal architecture of the existing backend provides an excellent foundation for these optimizations, allowing network improvements to be added as adapters without impacting the core domain logic.

---

*Document Version: 1.0*  
*Last Updated: 2025-08-19*  
*Author: Network Architecture Team*