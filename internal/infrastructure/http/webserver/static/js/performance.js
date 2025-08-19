/* Performance Monitoring for Alchemorsel */

(function() {
    'use strict';
    
    // Performance metrics collection
    const metrics = {
        pageLoadStart: window.performanceStart || performance.now(),
        navigationTiming: {},
        resourceTiming: [],
        userTiming: {}
    };
    
    // Collect navigation timing
    window.addEventListener('load', function() {
        if (performance.navigation && performance.timing) {
            const timing = performance.timing;
            metrics.navigationTiming = {
                domContentLoaded: timing.domContentLoadedEventEnd - timing.navigationStart,
                loadComplete: timing.loadEventEnd - timing.navigationStart,
                firstByte: timing.responseStart - timing.navigationStart,
                domReady: timing.domComplete - timing.navigationStart
            };
        }
        
        // Collect resource timing
        if (performance.getEntriesByType) {
            metrics.resourceTiming = performance.getEntriesByType('resource').map(function(entry) {
                return {
                    name: entry.name,
                    duration: entry.duration,
                    size: entry.transferSize || 0,
                    type: entry.initiatorType
                };
            });
        }
        
        // Log performance summary
        console.group('🚀 Alchemorsel Performance Metrics');
        console.log('Page Load:', metrics.navigationTiming.loadComplete + 'ms');
        console.log('DOM Ready:', metrics.navigationTiming.domReady + 'ms');
        console.log('First Byte:', metrics.navigationTiming.firstByte + 'ms');
        console.log('Resources:', metrics.resourceTiming.length);
        console.groupEnd();
        
        // Send to analytics if available
        if (typeof gtag !== 'undefined') {
            gtag('event', 'page_performance', {
                load_time: metrics.navigationTiming.loadComplete,
                dom_ready: metrics.navigationTiming.domReady,
                first_byte: metrics.navigationTiming.firstByte
            });
        }
    });
    
    // Core Web Vitals monitoring
    if ('PerformanceObserver' in window) {
        // Largest Contentful Paint
        try {
            const lcpObserver = new PerformanceObserver(function(list) {
                const entries = list.getEntries();
                const lcp = entries[entries.length - 1];
                console.log('LCP:', lcp.startTime + 'ms');
                
                if (typeof gtag !== 'undefined') {
                    gtag('event', 'web_vitals', {
                        metric_name: 'LCP',
                        metric_value: Math.round(lcp.startTime)
                    });
                }
            });
            lcpObserver.observe({ entryTypes: ['largest-contentful-paint'] });
        } catch (e) {
            console.debug('LCP monitoring not available');
        }
        
        // First Input Delay
        try {
            const fidObserver = new PerformanceObserver(function(list) {
                const entries = list.getEntries();
                entries.forEach(function(entry) {
                    console.log('FID:', entry.processingStart - entry.startTime + 'ms');
                    
                    if (typeof gtag !== 'undefined') {
                        gtag('event', 'web_vitals', {
                            metric_name: 'FID',
                            metric_value: Math.round(entry.processingStart - entry.startTime)
                        });
                    }
                });
            });
            fidObserver.observe({ entryTypes: ['first-input'] });
        } catch (e) {
            console.debug('FID monitoring not available');
        }
    }
    
    // Expose metrics for debugging
    window.AlchemorselMetrics = metrics;
})();