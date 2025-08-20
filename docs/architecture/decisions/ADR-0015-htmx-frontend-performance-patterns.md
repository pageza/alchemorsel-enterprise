# ADR-0015: HTMX Frontend Performance Patterns

## Status
Accepted

## Context
Alchemorsel v3 uses HTMX for dynamic frontend interactions, providing a simpler alternative to heavy JavaScript frameworks while maintaining rich user experiences. However, HTMX applications require specific performance optimization patterns to achieve optimal Core Web Vitals and user experience.

HTMX advantages:
- Minimal JavaScript payload for better First Input Delay
- Server-side rendering for faster initial page loads
- Progressive enhancement with graceful degradation
- Reduced complexity compared to SPA frameworks

Performance challenges:
- Network requests for every interaction
- Potential layout shifts during content updates
- Lack of client-side caching for dynamic content
- Limited offline functionality

Current usage patterns:
- Form submissions with server validation
- Dynamic content loading for user interactions
- Real-time updates for notifications
- Progressive disclosure of complex interfaces

## Decision
We will implement HTMX-specific performance patterns optimized for Core Web Vitals and user experience while leveraging HTMX's strengths.

**HTMX Performance Patterns:**

**Optimized Request Patterns:**
```html
<!-- Prefetch on hover for instant navigation -->
<a href="/dashboard" 
   hx-get="/dashboard" 
   hx-trigger="mouseenter once"
   hx-prefetch>Dashboard</a>

<!-- Debounced search with loading states -->
<input type="text" 
       name="query"
       hx-get="/search" 
       hx-trigger="keyup changed delay:300ms"
       hx-indicator="#search-spinner"
       hx-target="#search-results">
```

**Layout Stability (CLS Prevention):**
```html
<!-- Reserve space for dynamic content -->
<div id="dynamic-content" 
     style="min-height: 200px;"
     hx-get="/content" 
     hx-trigger="load">
  <div class="skeleton-loader">Loading...</div>
</div>

<!-- Use CSS transitions for smooth updates -->
<div hx-swap="outerHTML swap:300ms">
  <!-- Content will transition smoothly -->
</div>
```

**Caching Strategies:**
```html
<!-- Cache frequently accessed content -->
<div hx-get="/user-profile" 
     hx-trigger="load once"
     hx-cache="true">
</div>

<!-- Conditional requests with ETags -->
<div hx-get="/content" 
     hx-headers='{"If-None-Match": "etag-value"}'>
</div>
```

**Performance Monitoring:**
```javascript
// Track HTMX request performance
htmx.on('htmx:beforeRequest', function(evt) {
    evt.detail.requestConfig.startTime = performance.now();
});

htmx.on('htmx:afterRequest', function(evt) {
    const duration = performance.now() - evt.detail.requestConfig.startTime;
    // Log performance metrics
    analytics.track('htmx_request', {
        url: evt.detail.requestConfig.path,
        duration: duration,
        success: evt.detail.successful
    });
});
```

**Critical Path Optimization:**
- Inline critical HTMX configuration in HTML head
- Preload HTMX library with high priority
- Use resource hints for predictable HTMX requests
- Optimize server response times for HTMX endpoints

**Bundle Optimization:**
```html
<!-- Minimal HTMX bundle for initial load -->
<script src="/js/htmx.min.js" defer></script>

<!-- Extensions loaded on demand -->
<div hx-ext="ws" 
     hx-trigger="intersect once"
     hx-on="htmx:load: htmx.loadExtension('ws')">
</div>
```

**Server-Side Optimization:**
- HTMX endpoint responses under 14KB when possible
- Gzip/Brotli compression for all HTMX responses
- Appropriate cache headers for HTMX content
- Fast server response times (<100ms for simple requests)

**Error Handling and Fallbacks:**
```html
<!-- Graceful degradation for HTMX failures -->
<form hx-post="/submit" 
      hx-target="#result"
      hx-on="htmx:error: this.submit()">
  <!-- Form still works without HTMX -->
  <button type="submit">Submit</button>
</form>
```

**Performance Targets:**
- HTMX request response time: <200ms (95th percentile)
- Layout shift during updates: <0.05 CLS impact
- JavaScript bundle size: <50KB (including HTMX)
- Time to Interactive: <3s on mobile devices

## Consequences

### Positive
- Excellent Core Web Vitals scores with minimal JavaScript
- Fast initial page loads with server-side rendering
- Progressive enhancement provides reliable fallbacks
- Reduced complexity compared to SPA frameworks
- Better SEO with server-rendered content

### Negative
- Network dependency for all dynamic interactions
- Limited offline functionality compared to SPAs
- Requires careful optimization for mobile performance
- HTMX-specific knowledge needed for development team

### Neutral
- Performance characteristics different from SPAs but competitive
- Caching strategies adapted for HTMX interaction patterns
- Monitoring and analytics require HTMX-specific implementations