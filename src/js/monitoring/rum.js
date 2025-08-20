/**
 * Real User Monitoring (RUM) for Alchemorsel v3
 * Tracks frontend performance, user interactions, and Core Web Vitals
 */

class RealUserMonitoring {
    constructor(config = {}) {
        this.config = {
            endpoint: config.endpoint || '/api/v1/metrics/rum',
            sessionTimeout: config.sessionTimeout || 30 * 60 * 1000, // 30 minutes
            batchSize: config.batchSize || 10,
            flushInterval: config.flushInterval || 5000, // 5 seconds
            enableWebVitals: config.enableWebVitals !== false,
            enableUserTiming: config.enableUserTiming !== false,
            enableResourceTiming: config.enableResourceTiming !== false,
            enableErrorTracking: config.enableErrorTracking !== false,
            enableClickTracking: config.enableClickTracking !== false,
            samplingRate: config.samplingRate || 1.0,
            environment: config.environment || 'production',
            version: config.version || '1.0.0',
            ...config
        };

        this.sessionId = this.generateSessionId();
        this.userId = this.getUserId();
        this.pageViewId = this.generatePageViewId();
        this.startTime = performance.now();
        
        this.metrics = [];
        this.vitals = {};
        this.errors = [];
        this.interactions = [];
        
        this.init();
    }

    init() {
        if (Math.random() > this.config.samplingRate) {
            return; // Skip based on sampling rate
        }

        this.trackPageLoad();
        this.setupPerformanceObservers();
        
        if (this.config.enableWebVitals) {
            this.trackWebVitals();
        }
        
        if (this.config.enableUserTiming) {
            this.trackUserTiming();
        }
        
        if (this.config.enableResourceTiming) {
            this.trackResourceTiming();
        }
        
        if (this.config.enableErrorTracking) {
            this.trackErrors();
        }
        
        if (this.config.enableClickTracking) {
            this.trackClicks();
        }

        // Set up periodic flushing
        setInterval(() => this.flush(), this.config.flushInterval);
        
        // Flush on page unload
        window.addEventListener('beforeunload', () => this.flush(true));
        
        // Track session duration
        this.trackSession();
        
        console.log('RUM initialized', { sessionId: this.sessionId, userId: this.userId });
    }

    generateSessionId() {
        return 'session_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
    }

    generatePageViewId() {
        return 'pageview_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
    }

    getUserId() {
        // Get user ID from various sources
        const storedUserId = localStorage.getItem('userId') || 
                           sessionStorage.getItem('userId') ||
                           document.querySelector('[data-user-id]')?.getAttribute('data-user-id');
        
        if (storedUserId) {
            return storedUserId;
        }
        
        // Generate anonymous user ID
        let anonymousId = localStorage.getItem('anonymousUserId');
        if (!anonymousId) {
            anonymousId = 'anon_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
            localStorage.setItem('anonymousUserId', anonymousId);
        }
        return anonymousId;
    }

    trackPageLoad() {
        const navigation = performance.getEntriesByType('navigation')[0];
        if (!navigation) return;

        this.recordMetric('page.load', {
            type: 'navigation',
            url: window.location.href,
            referrer: document.referrer,
            title: document.title,
            timing: {
                dns_lookup: navigation.domainLookupEnd - navigation.domainLookupStart,
                tcp_connection: navigation.connectEnd - navigation.connectStart,
                tls_negotiation: navigation.secureConnectionStart > 0 ? 
                    navigation.connectEnd - navigation.secureConnectionStart : 0,
                request_response: navigation.responseEnd - navigation.requestStart,
                dom_processing: navigation.domComplete - navigation.responseEnd,
                load_complete: navigation.loadEventEnd - navigation.navigationStart,
                first_byte: navigation.responseStart - navigation.requestStart,
                dom_ready: navigation.domContentLoadedEventEnd - navigation.navigationStart,
                dom_interactive: navigation.domInteractive - navigation.navigationStart
            },
            sizes: {
                transfer_size: navigation.transferSize || 0,
                encoded_size: navigation.encodedBodySize || 0,
                decoded_size: navigation.decodedBodySize || 0
            },
            connection: this.getConnectionInfo(),
            device: this.getDeviceInfo()
        });
    }

    setupPerformanceObservers() {
        // Largest Contentful Paint
        if ('PerformanceObserver' in window) {
            try {
                const lcpObserver = new PerformanceObserver((list) => {
                    const entries = list.getEntries();
                    const lastEntry = entries[entries.length - 1];
                    this.vitals.lcp = lastEntry.startTime;
                    this.recordMetric('web_vitals.lcp', {
                        value: lastEntry.startTime,
                        element: lastEntry.element?.tagName || 'unknown',
                        url: lastEntry.url || window.location.href
                    });
                });
                lcpObserver.observe({ entryTypes: ['largest-contentful-paint'] });
            } catch (e) {
                console.warn('LCP observer not supported');
            }

            // First Input Delay
            try {
                const fidObserver = new PerformanceObserver((list) => {
                    for (const entry of list.getEntries()) {
                        this.vitals.fid = entry.processingStart - entry.startTime;
                        this.recordMetric('web_vitals.fid', {
                            value: entry.processingStart - entry.startTime,
                            event_type: entry.name,
                            target: entry.target?.tagName || 'unknown'
                        });
                    }
                });
                fidObserver.observe({ entryTypes: ['first-input'] });
            } catch (e) {
                console.warn('FID observer not supported');
            }

            // Layout Shift
            try {
                let clsValue = 0;
                let clsEntries = [];
                let sessionValue = 0;
                let sessionEntries = [];

                const clsObserver = new PerformanceObserver((list) => {
                    for (const entry of list.getEntries()) {
                        if (!entry.hadRecentInput) {
                            const firstSessionEntry = sessionEntries[0];
                            const lastSessionEntry = sessionEntries[sessionEntries.length - 1];

                            if (!firstSessionEntry || 
                                entry.startTime - lastSessionEntry.startTime > 1000 ||
                                entry.startTime - firstSessionEntry.startTime > 5000) {
                                
                                if (sessionValue > clsValue) {
                                    clsValue = sessionValue;
                                    clsEntries = [...sessionEntries];
                                }
                                sessionValue = entry.value;
                                sessionEntries = [entry];
                            } else {
                                sessionValue += entry.value;
                                sessionEntries.push(entry);
                            }
                        }
                    }

                    if (sessionValue > clsValue) {
                        clsValue = sessionValue;
                        clsEntries = [...sessionEntries];
                    }

                    this.vitals.cls = clsValue;
                    this.recordMetric('web_vitals.cls', {
                        value: clsValue,
                        entries_count: clsEntries.length
                    });
                });
                clsObserver.observe({ entryTypes: ['layout-shift'] });
            } catch (e) {
                console.warn('CLS observer not supported');
            }
        }
    }

    trackWebVitals() {
        // Time to First Byte (TTFB)
        const navigation = performance.getEntriesByType('navigation')[0];
        if (navigation) {
            const ttfb = navigation.responseStart - navigation.requestStart;
            this.vitals.ttfb = ttfb;
            this.recordMetric('web_vitals.ttfb', { value: ttfb });
        }

        // First Contentful Paint
        const fcpEntry = performance.getEntriesByName('first-contentful-paint')[0];
        if (fcpEntry) {
            this.vitals.fcp = fcpEntry.startTime;
            this.recordMetric('web_vitals.fcp', { value: fcpEntry.startTime });
        }

        // Track when all vital metrics are collected
        setTimeout(() => {
            this.recordMetric('web_vitals.summary', {
                lcp: this.vitals.lcp || null,
                fid: this.vitals.fid || null,
                cls: this.vitals.cls || null,
                ttfb: this.vitals.ttfb || null,
                fcp: this.vitals.fcp || null
            });
        }, 3000);
    }

    trackUserTiming() {
        if ('PerformanceObserver' in window) {
            const userTimingObserver = new PerformanceObserver((list) => {
                for (const entry of list.getEntries()) {
                    this.recordMetric('user_timing', {
                        name: entry.name,
                        entry_type: entry.entryType,
                        duration: entry.duration || entry.startTime,
                        start_time: entry.startTime
                    });
                }
            });

            userTimingObserver.observe({ entryTypes: ['mark', 'measure'] });
        }
    }

    trackResourceTiming() {
        if ('PerformanceObserver' in window) {
            const resourceObserver = new PerformanceObserver((list) => {
                for (const entry of list.getEntries()) {
                    // Skip data URLs and blob URLs
                    if (entry.name.startsWith('data:') || entry.name.startsWith('blob:')) {
                        continue;
                    }

                    this.recordMetric('resource.timing', {
                        name: entry.name,
                        type: entry.initiatorType,
                        duration: entry.duration,
                        transfer_size: entry.transferSize || 0,
                        encoded_size: entry.encodedBodySize || 0,
                        decoded_size: entry.decodedBodySize || 0,
                        timing: {
                            dns_lookup: entry.domainLookupEnd - entry.domainLookupStart,
                            tcp_connection: entry.connectEnd - entry.connectStart,
                            request_response: entry.responseEnd - entry.requestStart
                        }
                    });
                }
            });

            resourceObserver.observe({ entryTypes: ['resource'] });
        }
    }

    trackErrors() {
        // JavaScript errors
        window.addEventListener('error', (event) => {
            this.recordError({
                type: 'javascript',
                message: event.message,
                filename: event.filename,
                line_number: event.lineno,
                column_number: event.colno,
                stack: event.error?.stack,
                timestamp: Date.now()
            });
        });

        // Promise rejections
        window.addEventListener('unhandledrejection', (event) => {
            this.recordError({
                type: 'unhandled_rejection',
                message: event.reason?.message || 'Unhandled Promise Rejection',
                stack: event.reason?.stack,
                timestamp: Date.now()
            });
        });

        // Resource loading errors
        window.addEventListener('error', (event) => {
            if (event.target !== window) {
                this.recordError({
                    type: 'resource',
                    message: `Failed to load ${event.target.tagName}`,
                    source: event.target.src || event.target.href,
                    timestamp: Date.now()
                });
            }
        }, true);
    }

    trackClicks() {
        document.addEventListener('click', (event) => {
            const element = event.target;
            const tagName = element.tagName.toLowerCase();
            
            // Only track meaningful interactions
            if (['button', 'a', 'input'].includes(tagName) || 
                element.hasAttribute('data-track-click')) {
                
                this.recordInteraction({
                    type: 'click',
                    element: tagName,
                    id: element.id || null,
                    class: element.className || null,
                    text: element.textContent?.slice(0, 100) || null,
                    href: element.href || null,
                    coordinates: {
                        x: event.clientX,
                        y: event.clientY
                    },
                    timestamp: Date.now()
                });
            }
        });
    }

    trackSession() {
        let lastActivity = Date.now();
        const sessionStart = Date.now();

        const updateActivity = () => {
            lastActivity = Date.now();
        };

        ['mousedown', 'mousemove', 'keypress', 'scroll', 'touchstart', 'click'].forEach(event => {
            document.addEventListener(event, updateActivity, true);
        });

        // Check session activity every minute
        setInterval(() => {
            const now = Date.now();
            const timeSinceActivity = now - lastActivity;
            
            if (timeSinceActivity > this.config.sessionTimeout) {
                this.recordMetric('session.ended', {
                    duration: lastActivity - sessionStart,
                    reason: 'timeout'
                });
                this.flush(true);
            }
        }, 60000);
    }

    getConnectionInfo() {
        const connection = navigator.connection || navigator.mozConnection || navigator.webkitConnection;
        if (!connection) return null;

        return {
            effective_type: connection.effectiveType,
            downlink: connection.downlink,
            rtt: connection.rtt,
            save_data: connection.saveData
        };
    }

    getDeviceInfo() {
        return {
            user_agent: navigator.userAgent,
            language: navigator.language,
            platform: navigator.platform,
            screen: {
                width: screen.width,
                height: screen.height,
                color_depth: screen.colorDepth
            },
            viewport: {
                width: window.innerWidth,
                height: window.innerHeight
            },
            memory: navigator.deviceMemory || null,
            cores: navigator.hardwareConcurrency || null
        };
    }

    recordMetric(name, data = {}) {
        const metric = {
            name,
            timestamp: Date.now(),
            session_id: this.sessionId,
            page_view_id: this.pageViewId,
            user_id: this.userId,
            url: window.location.href,
            environment: this.config.environment,
            version: this.config.version,
            data
        };

        this.metrics.push(metric);

        if (this.metrics.length >= this.config.batchSize) {
            this.flush();
        }
    }

    recordError(error) {
        const errorRecord = {
            ...error,
            session_id: this.sessionId,
            page_view_id: this.pageViewId,
            user_id: this.userId,
            url: window.location.href,
            user_agent: navigator.userAgent
        };

        this.errors.push(errorRecord);
        
        // Send errors immediately
        this.sendData('errors', [errorRecord]);
    }

    recordInteraction(interaction) {
        const interactionRecord = {
            ...interaction,
            session_id: this.sessionId,
            page_view_id: this.pageViewId,
            user_id: this.userId,
            url: window.location.href
        };

        this.interactions.push(interactionRecord);

        if (this.interactions.length >= 5) {
            this.sendData('interactions', [...this.interactions]);
            this.interactions = [];
        }
    }

    flush(immediate = false) {
        const promises = [];

        if (this.metrics.length > 0) {
            promises.push(this.sendData('metrics', [...this.metrics], immediate));
            this.metrics = [];
        }

        if (this.errors.length > 0) {
            promises.push(this.sendData('errors', [...this.errors], immediate));
            this.errors = [];
        }

        if (this.interactions.length > 0) {
            promises.push(this.sendData('interactions', [...this.interactions], immediate));
            this.interactions = [];
        }

        return Promise.all(promises);
    }

    sendData(type, data, immediate = false) {
        const payload = {
            type,
            data,
            metadata: {
                timestamp: Date.now(),
                user_agent: navigator.userAgent,
                url: window.location.href
            }
        };

        if (immediate && navigator.sendBeacon) {
            // Use sendBeacon for immediate sending (e.g., on page unload)
            return navigator.sendBeacon(
                `${this.config.endpoint}/${type}`,
                JSON.stringify(payload)
            );
        } else {
            // Use fetch for regular sending
            return fetch(`${this.config.endpoint}/${type}`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(payload)
            }).catch(error => {
                console.error('Failed to send RUM data:', error);
            });
        }
    }

    // Manual tracking methods
    startTiming(name) {
        performance.mark(`${name}-start`);
    }

    endTiming(name) {
        performance.mark(`${name}-end`);
        performance.measure(name, `${name}-start`, `${name}-end`);
    }

    trackCustomEvent(name, data = {}) {
        this.recordMetric(`custom.${name}`, data);
    }

    trackFeatureUsage(feature, action, metadata = {}) {
        this.recordMetric('feature.usage', {
            feature,
            action,
            ...metadata
        });
    }

    trackBusinessEvent(event, value = null, metadata = {}) {
        this.recordMetric('business.event', {
            event,
            value,
            ...metadata
        });
    }

    setUserId(userId) {
        this.userId = userId;
        localStorage.setItem('userId', userId);
    }

    setUserProperties(properties) {
        this.recordMetric('user.properties', properties);
    }
}

// Initialize RUM when the page loads
document.addEventListener('DOMContentLoaded', () => {
    // Check if RUM is enabled via configuration or meta tag
    const rumConfig = window.RUM_CONFIG || {};
    const rumEnabled = document.querySelector('meta[name="rum-enabled"]')?.content !== 'false';
    
    if (rumEnabled) {
        window.RUM = new RealUserMonitoring(rumConfig);
    }
});

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = RealUserMonitoring;
}

window.RealUserMonitoring = RealUserMonitoring;