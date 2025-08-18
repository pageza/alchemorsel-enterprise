/* Alchemorsel v3 - Performance Measurement and Validation */
(function() {
    'use strict';

    // Performance monitoring class
    class PerformanceMonitor {
        constructor() {
            this.metrics = {
                navigationStart: 0,
                firstPaint: 0,
                firstContentfulPaint: 0,
                domContentLoaded: 0,
                loadComplete: 0,
                firstPacketSize: 0,
                resourcesLoaded: {},
                interactionMetrics: [],
                vitals: {}
            };
            
            this.thresholds = {
                firstContentfulPaint: 1800, // 1.8s
                largestContentfulPaint: 2500, // 2.5s
                firstInputDelay: 100, // 100ms
                cumulativeLayoutShift: 0.1, // 0.1
                firstPacketSize: 14336 // 14KB in bytes
            };

            this.init();
        }

        init() {
            this.measureNavigationTiming();
            this.measurePaintTiming();
            this.measureResourceTiming();
            this.measureWebVitals();
            this.measureInteractionMetrics();
            this.validateFirstPacketOptimization();
            this.setupReporting();
        }

        // Navigation Timing API
        measureNavigationTiming() {
            if (performance.timing) {
                const timing = performance.timing;
                this.metrics.navigationStart = timing.navigationStart;
                this.metrics.domContentLoaded = timing.domContentLoadedEventEnd - timing.navigationStart;
                this.metrics.loadComplete = timing.loadEventEnd - timing.navigationStart;
            }

            // Performance Observer for more accurate data
            if ('PerformanceObserver' in window) {
                const observer = new PerformanceObserver((list) => {
                    for (const entry of list.getEntries()) {
                        if (entry.entryType === 'navigation') {
                            this.metrics.domContentLoaded = entry.domContentLoadedEventEnd;
                            this.metrics.loadComplete = entry.loadEventEnd;
                        }
                    }
                });
                observer.observe({ entryTypes: ['navigation'] });
            }
        }

        // Paint Timing API
        measurePaintTiming() {
            if ('PerformanceObserver' in window) {
                const observer = new PerformanceObserver((list) => {
                    for (const entry of list.getEntries()) {
                        if (entry.name === 'first-paint') {
                            this.metrics.firstPaint = entry.startTime;
                        } else if (entry.name === 'first-contentful-paint') {
                            this.metrics.firstContentfulPaint = entry.startTime;
                        }
                    }
                });
                observer.observe({ entryTypes: ['paint'] });
            }
        }

        // Resource Timing API
        measureResourceTiming() {
            if ('PerformanceObserver' in window) {
                const observer = new PerformanceObserver((list) => {
                    for (const entry of list.getEntries()) {
                        const resourceInfo = {
                            name: entry.name,
                            size: entry.transferSize || entry.encodedBodySize || 0,
                            duration: entry.duration,
                            startTime: entry.startTime,
                            type: this.getResourceType(entry.name)
                        };
                        
                        this.metrics.resourcesLoaded[entry.name] = resourceInfo;
                        
                        // Calculate first packet size for critical resources
                        if (this.isCriticalResource(entry.name)) {
                            this.metrics.firstPacketSize += resourceInfo.size;
                        }
                    }
                });
                observer.observe({ entryTypes: ['resource'] });
            }
        }

        // Web Vitals measurement
        measureWebVitals() {
            // Largest Contentful Paint
            if ('PerformanceObserver' in window) {
                const lcpObserver = new PerformanceObserver((list) => {
                    const entries = list.getEntries();
                    const lastEntry = entries[entries.length - 1];
                    this.metrics.vitals.largestContentfulPaint = lastEntry.startTime;
                });
                lcpObserver.observe({ entryTypes: ['largest-contentful-paint'] });

                // First Input Delay
                const fidObserver = new PerformanceObserver((list) => {
                    for (const entry of list.getEntries()) {
                        this.metrics.vitals.firstInputDelay = entry.processingStart - entry.startTime;
                    }
                });
                fidObserver.observe({ entryTypes: ['first-input'] });

                // Cumulative Layout Shift
                let clsValue = 0;
                const clsObserver = new PerformanceObserver((list) => {
                    for (const entry of list.getEntries()) {
                        if (!entry.hadRecentInput) {
                            clsValue += entry.value;
                        }
                    }
                    this.metrics.vitals.cumulativeLayoutShift = clsValue;
                });
                clsObserver.observe({ entryTypes: ['layout-shift'] });
            }
        }

        // Interaction metrics (click, scroll, keyboard)
        measureInteractionMetrics() {
            const interactionTypes = ['click', 'keydown', 'scroll'];
            
            interactionTypes.forEach(type => {
                document.addEventListener(type, (event) => {
                    const startTime = performance.now();
                    
                    // Measure interaction response time
                    requestAnimationFrame(() => {
                        const endTime = performance.now();
                        const interactionTime = endTime - startTime;
                        
                        this.metrics.interactionMetrics.push({
                            type: type,
                            target: event.target.tagName,
                            time: interactionTime,
                            timestamp: Date.now()
                        });
                    });
                }, { passive: true });
            });
        }

        // Validate 14KB first packet optimization
        validateFirstPacketOptimization() {
            window.addEventListener('load', () => {
                setTimeout(() => {
                    const criticalSize = this.calculateCriticalResourceSize();
                    this.metrics.firstPacketSize = criticalSize;
                    
                    const isOptimized = criticalSize <= this.thresholds.firstPacketSize;
                    
                    console.log(`🚀 First Packet Optimization:`, {
                        size: `${(criticalSize / 1024).toFixed(2)}KB`,
                        threshold: `${(this.thresholds.firstPacketSize / 1024).toFixed(2)}KB`,
                        optimized: isOptimized ? '✅ PASSED' : '❌ FAILED',
                        breakdown: this.getCriticalResourceBreakdown()
                    });
                    
                    // Report to service worker
                    this.reportToServiceWorker();
                }, 1000);
            });
        }

        // Calculate critical resource size
        calculateCriticalResourceSize() {
            let totalSize = 0;
            const criticalResources = Object.values(this.metrics.resourcesLoaded)
                .filter(resource => this.isCriticalResource(resource.name));
            
            criticalResources.forEach(resource => {
                totalSize += resource.size;
            });
            
            // Add estimated HTML size (main document)
            const navigationEntry = performance.getEntriesByType('navigation')[0];
            if (navigationEntry) {
                totalSize += navigationEntry.transferSize || 8192; // Default 8KB estimate
            }
            
            return totalSize;
        }

        // Get breakdown of critical resources
        getCriticalResourceBreakdown() {
            const breakdown = {};
            
            Object.values(this.metrics.resourcesLoaded).forEach(resource => {
                if (this.isCriticalResource(resource.name)) {
                    const type = resource.type;
                    if (!breakdown[type]) {
                        breakdown[type] = { count: 0, size: 0 };
                    }
                    breakdown[type].count++;
                    breakdown[type].size += resource.size;
                }
            });
            
            // Format for display
            const formatted = {};
            Object.entries(breakdown).forEach(([type, data]) => {
                formatted[type] = `${data.count} files, ${(data.size / 1024).toFixed(2)}KB`;
            });
            
            return formatted;
        }

        // Check if resource is critical for first packet
        isCriticalResource(url) {
            const criticalPatterns = [
                '/static/css/critical.css',
                '/static/js/htmx.min.js',
                '/static/js/app.js',
                'data:' // Inline resources
            ];
            
            return criticalPatterns.some(pattern => url.includes(pattern)) ||
                   url === location.href; // Main document
        }

        // Determine resource type
        getResourceType(url) {
            if (url.includes('.css')) return 'css';
            if (url.includes('.js')) return 'javascript';
            if (url.includes('.png') || url.includes('.jpg') || url.includes('.svg')) return 'image';
            if (url.includes('.woff') || url.includes('.ttf')) return 'font';
            if (url === location.href) return 'document';
            return 'other';
        }

        // Performance reporting setup
        setupReporting() {
            // Report on page visibility change (user leaving)
            document.addEventListener('visibilitychange', () => {
                if (document.visibilityState === 'hidden') {
                    this.sendReport();
                }
            });

            // Report on page unload
            window.addEventListener('beforeunload', () => {
                this.sendReport();
            });

            // Report periodically for long sessions
            setInterval(() => {
                this.sendReport();
            }, 30000); // Every 30 seconds
        }

        // Send performance report
        sendReport() {
            const report = this.generateReport();
            
            // Send to performance endpoint
            if (navigator.sendBeacon) {
                navigator.sendBeacon('/performance', JSON.stringify(report));
            } else {
                fetch('/performance', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(report),
                    keepalive: true
                }).catch(error => {
                    console.error('Failed to send performance report:', error);
                });
            }
        }

        // Generate performance report
        generateReport() {
            return {
                url: location.href,
                timestamp: Date.now(),
                userAgent: navigator.userAgent,
                connection: this.getConnectionInfo(),
                metrics: {
                    ...this.metrics,
                    score: this.calculatePerformanceScore()
                },
                thresholds: this.thresholds,
                optimizationStatus: {
                    firstPacketOptimized: this.metrics.firstPacketSize <= this.thresholds.firstPacketSize,
                    vitalsPass: this.checkWebVitalsPass(),
                    accessibilityFeatures: this.checkAccessibilityFeatures()
                }
            };
        }

        // Get connection information
        getConnectionInfo() {
            if ('connection' in navigator) {
                return {
                    effectiveType: navigator.connection.effectiveType,
                    downlink: navigator.connection.downlink,
                    rtt: navigator.connection.rtt
                };
            }
            return null;
        }

        // Calculate overall performance score
        calculatePerformanceScore() {
            let score = 100;
            
            // Penalize for slow FCP
            if (this.metrics.firstContentfulPaint > this.thresholds.firstContentfulPaint) {
                score -= 20;
            }
            
            // Penalize for large first packet
            if (this.metrics.firstPacketSize > this.thresholds.firstPacketSize) {
                score -= 25;
            }
            
            // Penalize for poor Web Vitals
            if (!this.checkWebVitalsPass()) {
                score -= 30;
            }
            
            // Penalize for slow interactions
            const avgInteractionTime = this.getAverageInteractionTime();
            if (avgInteractionTime > 50) {
                score -= 15;
            }
            
            return Math.max(0, score);
        }

        // Check if Web Vitals pass thresholds
        checkWebVitalsPass() {
            const vitals = this.metrics.vitals;
            return (
                (vitals.largestContentfulPaint || 0) <= this.thresholds.largestContentfulPaint &&
                (vitals.firstInputDelay || 0) <= this.thresholds.firstInputDelay &&
                (vitals.cumulativeLayoutShift || 0) <= this.thresholds.cumulativeLayoutShift
            );
        }

        // Check accessibility features
        checkAccessibilityFeatures() {
            return {
                skipLinks: !!document.querySelector('.skip-link'),
                ariaLabels: document.querySelectorAll('[aria-label]').length > 0,
                altText: Array.from(document.querySelectorAll('img')).every(img => img.alt !== undefined),
                focusManagement: !!window.A11yController,
                keyboardNavigation: document.body.classList.contains('keyboard-navigation')
            };
        }

        // Get average interaction time
        getAverageInteractionTime() {
            if (this.metrics.interactionMetrics.length === 0) return 0;
            
            const totalTime = this.metrics.interactionMetrics.reduce((sum, metric) => sum + metric.time, 0);
            return totalTime / this.metrics.interactionMetrics.length;
        }

        // Report to service worker
        reportToServiceWorker() {
            if ('serviceWorker' in navigator && navigator.serviceWorker.controller) {
                navigator.serviceWorker.controller.postMessage({
                    type: 'PERFORMANCE_MEASURE',
                    metrics: this.metrics
                });
            }
        }

        // Public API for manual measurements
        mark(name) {
            performance.mark(name);
        }

        measure(name, startMark, endMark) {
            performance.measure(name, startMark, endMark);
            
            const measures = performance.getEntriesByName(name, 'measure');
            if (measures.length > 0) {
                console.log(`⏱️  ${name}: ${measures[0].duration.toFixed(2)}ms`);
            }
        }

        // Get current metrics
        getMetrics() {
            return { ...this.metrics };
        }

        // Display performance dashboard
        showDashboard() {
            const dashboard = this.createDashboard();
            document.body.appendChild(dashboard);
        }

        createDashboard() {
            const dashboard = document.createElement('div');
            dashboard.className = 'performance-dashboard';
            dashboard.style.cssText = `
                position: fixed;
                top: 50%;
                left: 50%;
                transform: translate(-50%, -50%);
                background: white;
                border: 1px solid #ccc;
                border-radius: 8px;
                padding: 2rem;
                box-shadow: 0 10px 25px rgba(0,0,0,0.3);
                z-index: 10001;
                max-width: 500px;
                max-height: 80vh;
                overflow-y: auto;
            `;
            
            const score = this.calculatePerformanceScore();
            const scoreColor = score >= 80 ? '#22c55e' : score >= 60 ? '#f59e0b' : '#ef4444';
            
            dashboard.innerHTML = `
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
                    <h3>Performance Dashboard</h3>
                    <button onclick="this.closest('.performance-dashboard').remove()">×</button>
                </div>
                
                <div style="text-align: center; margin-bottom: 2rem;">
                    <div style="font-size: 3rem; font-weight: bold; color: ${scoreColor};">${score}</div>
                    <div>Performance Score</div>
                </div>
                
                <div style="margin-bottom: 1rem;">
                    <h4>14KB First Packet Optimization</h4>
                    <div style="display: flex; justify-content: space-between;">
                        <span>Current Size:</span>
                        <span>${(this.metrics.firstPacketSize / 1024).toFixed(2)}KB</span>
                    </div>
                    <div style="display: flex; justify-content: space-between;">
                        <span>Target:</span>
                        <span>14KB</span>
                    </div>
                    <div style="color: ${this.metrics.firstPacketSize <= this.thresholds.firstPacketSize ? '#22c55e' : '#ef4444'};">
                        ${this.metrics.firstPacketSize <= this.thresholds.firstPacketSize ? '✅ Optimized' : '❌ Needs Optimization'}
                    </div>
                </div>
                
                <div style="margin-bottom: 1rem;">
                    <h4>Core Web Vitals</h4>
                    <div>FCP: ${(this.metrics.firstContentfulPaint || 0).toFixed(0)}ms</div>
                    <div>LCP: ${(this.metrics.vitals.largestContentfulPaint || 0).toFixed(0)}ms</div>
                    <div>FID: ${(this.metrics.vitals.firstInputDelay || 0).toFixed(0)}ms</div>
                    <div>CLS: ${(this.metrics.vitals.cumulativeLayoutShift || 0).toFixed(3)}</div>
                </div>
                
                <div>
                    <h4>Resource Breakdown</h4>
                    ${Object.entries(this.getCriticalResourceBreakdown()).map(([type, info]) => 
                        `<div style="display: flex; justify-content: space-between;">
                            <span>${type}:</span>
                            <span>${info}</span>
                        </div>`
                    ).join('')}
                </div>
            `;
            
            return dashboard;
        }
    }

    // Initialize performance monitoring
    const performanceMonitor = new PerformanceMonitor();

    // Export for global access
    window.PerformanceMonitor = performanceMonitor;

    // Add keyboard shortcut to show dashboard (Ctrl/Cmd + Shift + P)
    document.addEventListener('keydown', (e) => {
        if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'P') {
            e.preventDefault();
            performanceMonitor.showDashboard();
        }
    });

    // Console commands for debugging
    console.log('🚀 Alchemorsel Performance Monitor loaded');
    console.log('📊 Use PerformanceMonitor.showDashboard() to view metrics');
    console.log('⌨️  Press Ctrl/Cmd + Shift + P to show performance dashboard');

})();