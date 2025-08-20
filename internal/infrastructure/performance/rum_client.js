/**
 * Real User Monitoring (RUM) Client for Alchemorsel v3
 * Collects Core Web Vitals and business metrics for performance optimization
 * Targets: LCP < 2.5s, CLS < 0.1, INP < 200ms
 */

class AlchemorselRUM {
  constructor(config = {}) {
    this.config = {
      endpoint: '/api/rum/collect',
      sampleRate: 0.05, // 5% sampling rate
      batchSize: 10,
      flushInterval: 30000, // 30 seconds
      enableDetailedMetrics: true,
      enableBusinessMetrics: true,
      enableHeatmaps: true,
      enableUserJourneys: true,
      ...config
    };

    this.sessionId = this.generateSessionId();
    this.pageViewId = this.generatePageViewId();
    this.userId = this.getUserId();
    this.measurements = [];
    this.interactions = [];
    this.businessEvents = [];
    
    this.deviceInfo = this.collectDeviceInfo();
    this.networkInfo = this.collectNetworkInfo();
    
    this.observers = new Map();
    this.startTime = performance.now();
    this.pageStartTime = Date.now();
    
    this.init();
  }

  init() {
    if (!this.shouldSample()) {
      return;
    }

    this.setupPerformanceObservers();
    this.setupInteractionTracking();
    this.setupBusinessMetrics();
    this.setupPageLifecycleTracking();
    this.setupHTMXTracking();
    this.startPeriodicFlush();
    
    // Send initial page load measurement
    this.scheduleFlush(5000); // Send after 5 seconds
  }

  shouldSample() {
    return Math.random() < this.config.sampleRate;
  }

  generateSessionId() {
    return sessionStorage.getItem('rum-session-id') || 
      this.setSessionId('session_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9));
  }

  setSessionId(id) {
    sessionStorage.setItem('rum-session-id', id);
    return id;
  }

  generatePageViewId() {
    return 'page_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
  }

  getUserId() {
    return localStorage.getItem('user-id') || 
           document.cookie.match(/user_id=([^;]+)/)?.[1] || 
           'anonymous';
  }

  collectDeviceInfo() {
    const ua = navigator.userAgent;
    return {
      type: this.getDeviceType(),
      model: this.getDeviceModel(ua),
      os: this.getOS(ua),
      osVersion: this.getOSVersion(ua),
      browser: this.getBrowser(ua),
      browserVersion: this.getBrowserVersion(ua),
      viewportWidth: window.innerWidth,
      viewportHeight: window.innerHeight,
      screenWidth: screen.width,
      screenHeight: screen.height,
      pixelRatio: window.devicePixelRatio || 1,
      colorDepth: screen.colorDepth,
      touchSupport: 'ontouchstart' in window,
      orientation: screen.orientation?.type || 'unknown'
    };
  }

  collectNetworkInfo() {
    const connection = navigator.connection || navigator.mozConnection || navigator.webkitConnection;
    return {
      type: connection?.type || 'unknown',
      effectiveType: connection?.effectiveType || 'unknown',
      downlink: connection?.downlink || 0,
      rtt: connection?.rtt || 0,
      saveData: connection?.saveData || false,
      connectionSpeed: this.categorizeConnectionSpeed(connection?.effectiveType)
    };
  }

  setupPerformanceObservers() {
    // Core Web Vitals observers
    this.setupLCPObserver();
    this.setupCLSObserver();
    this.setupINPObserver();
    this.setupFCPObserver();
    this.setupTTFBObserver();
    
    // Resource observers
    this.setupResourceObserver();
    this.setupNavigationObserver();
    this.setupMemoryObserver();
  }

  setupLCPObserver() {
    if ('PerformanceObserver' in window) {
      const observer = new PerformanceObserver((list) => {
        const entries = list.getEntries();
        const lastEntry = entries[entries.length - 1];
        
        this.recordMetric('LCP', lastEntry.startTime, {
          element: lastEntry.element?.tagName?.toLowerCase(),
          url: lastEntry.url,
          size: lastEntry.size,
          id: lastEntry.element?.id,
          className: lastEntry.element?.className
        });
      });
      
      try {
        observer.observe({ entryTypes: ['largest-contentful-paint'] });
        this.observers.set('lcp', observer);
      } catch (e) {
        console.warn('LCP observer not supported:', e);
      }
    }
  }

  setupCLSObserver() {
    if ('PerformanceObserver' in window) {
      let clsValue = 0;
      const observer = new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          if (!entry.hadRecentInput) {
            clsValue += entry.value;
          }
        }
        
        this.recordMetric('CLS', clsValue, {
          sessionValue: clsValue,
          hadRecentInput: entry.hadRecentInput
        });
      });
      
      try {
        observer.observe({ entryTypes: ['layout-shift'] });
        this.observers.set('cls', observer);
      } catch (e) {
        console.warn('CLS observer not supported:', e);
      }
    }
  }

  setupINPObserver() {
    if ('PerformanceObserver' in window) {
      const observer = new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          if (entry.interactionId) {
            this.recordMetric('INP', entry.duration, {
              interactionType: entry.name,
              target: entry.target?.tagName?.toLowerCase(),
              startTime: entry.startTime
            });
          }
        }
      });
      
      try {
        observer.observe({ 
          entryTypes: ['event'], 
          durationThreshold: 16 // Only track interactions > 16ms
        });
        this.observers.set('inp', observer);
      } catch (e) {
        console.warn('INP observer not supported:', e);
      }
    }
  }

  setupFCPObserver() {
    if ('PerformanceObserver' in window) {
      const observer = new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          if (entry.name === 'first-contentful-paint') {
            this.recordMetric('FCP', entry.startTime);
          }
        }
      });
      
      try {
        observer.observe({ entryTypes: ['paint'] });
        this.observers.set('fcp', observer);
      } catch (e) {
        console.warn('FCP observer not supported:', e);
      }
    }
  }

  setupTTFBObserver() {
    // TTFB from Navigation Timing
    window.addEventListener('load', () => {
      const navTiming = performance.getEntriesByType('navigation')[0];
      if (navTiming) {
        const ttfb = navTiming.responseStart - navTiming.requestStart;
        this.recordMetric('TTFB', ttfb);
      }
    });
  }

  setupResourceObserver() {
    if ('PerformanceObserver' in window) {
      const observer = new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          if (entry.name.includes('recipe') || entry.name.includes('image') || entry.name.includes('font')) {
            this.recordResourceMetric(entry);
          }
        }
      });
      
      try {
        observer.observe({ entryTypes: ['resource'] });
        this.observers.set('resource', observer);
      } catch (e) {
        console.warn('Resource observer not supported:', e);
      }
    }
  }

  setupNavigationObserver() {
    window.addEventListener('load', () => {
      const navEntry = performance.getEntriesByType('navigation')[0];
      if (navEntry) {
        this.recordNavigationMetrics(navEntry);
      }
    });
  }

  setupMemoryObserver() {
    // Monitor memory usage periodically
    if ('memory' in performance) {
      const checkMemory = () => {
        const memory = performance.memory;
        this.recordMetric('MEMORY', memory.usedJSHeapSize, {
          total: memory.totalJSHeapSize,
          limit: memory.jsHeapSizeLimit,
          percentage: (memory.usedJSHeapSize / memory.totalJSHeapSize) * 100
        });
      };
      
      // Check memory every 30 seconds
      setInterval(checkMemory, 30000);
      
      // Check memory on page visibility change
      document.addEventListener('visibilitychange', checkMemory);
    }
  }

  setupInteractionTracking() {
    // Track clicks with response time
    document.addEventListener('click', (event) => {
      const startTime = performance.now();
      
      this.recordInteraction({
        type: 'click',
        timestamp: Date.now(),
        element: this.getElementSelector(event.target),
        startTime: startTime,
        coordinates: { x: event.clientX, y: event.clientY }
      });
      
      // Measure response time to next paint
      requestAnimationFrame(() => {
        const responseTime = performance.now() - startTime;
        this.updateInteractionResponseTime(startTime, responseTime);
      });
    }, { passive: true });

    // Track form interactions
    document.addEventListener('submit', (event) => {
      this.recordInteraction({
        type: 'form_submit',
        timestamp: Date.now(),
        element: this.getElementSelector(event.target),
        formData: this.getFormData(event.target)
      });
    }, { passive: true });

    // Track scroll depth
    let maxScrollDepth = 0;
    const trackScrollDepth = () => {
      const scrollDepth = Math.round(
        (window.scrollY / (document.documentElement.scrollHeight - window.innerHeight)) * 100
      );
      
      if (scrollDepth > maxScrollDepth) {
        maxScrollDepth = scrollDepth;
        this.recordMetric('SCROLL_DEPTH', maxScrollDepth);
      }
    };
    
    window.addEventListener('scroll', this.throttle(trackScrollDepth, 250), { passive: true });
  }

  setupBusinessMetrics() {
    if (!this.config.enableBusinessMetrics) return;

    // Track recipe views
    this.observeRecipeViews();
    
    // Track search interactions
    this.observeSearchInteractions();
    
    // Track user engagement
    this.observeEngagementMetrics();
    
    // Track conversion events
    this.observeConversionEvents();
  }

  observeRecipeViews() {
    // Track when recipes come into view
    if ('IntersectionObserver' in window) {
      const recipeObserver = new IntersectionObserver((entries) => {
        entries.forEach(entry => {
          if (entry.isIntersecting) {
            const recipeId = entry.target.getAttribute('data-recipe-id') || 
                           entry.target.id || 
                           'unknown';
            
            this.recordBusinessEvent('recipe_viewed', {
              recipeId: recipeId,
              viewTime: Date.now(),
              visibilityRatio: entry.intersectionRatio
            });
          }
        });
      }, { threshold: 0.5 });

      // Observe recipe cards
      document.querySelectorAll('.recipe-card, [data-recipe-id]').forEach(el => {
        recipeObserver.observe(el);
      });
    }
  }

  observeSearchInteractions() {
    // Track search queries
    const searchInputs = document.querySelectorAll('input[type="search"], input[data-search]');
    searchInputs.forEach(input => {
      const trackSearch = this.debounce((event) => {
        const query = event.target.value.trim();
        if (query.length > 2) {
          this.recordBusinessEvent('search_performed', {
            query: query,
            timestamp: Date.now(),
            queryLength: query.length
          });
        }
      }, 500);
      
      input.addEventListener('input', trackSearch);
    });
  }

  observeEngagementMetrics() {
    // Track time on page
    let engagementStart = Date.now();
    let isEngaged = true;
    
    const trackEngagement = () => {
      if (isEngaged) {
        const timeOnPage = Date.now() - engagementStart;
        this.recordMetric('TIME_ON_PAGE', timeOnPage);
      }
    };
    
    // Track when user becomes inactive
    document.addEventListener('visibilitychange', () => {
      if (document.hidden) {
        isEngaged = false;
        trackEngagement();
      } else {
        isEngaged = true;
        engagementStart = Date.now();
      }
    });
    
    // Track before page unload
    window.addEventListener('beforeunload', trackEngagement);
    
    // Periodic engagement tracking
    setInterval(trackEngagement, 15000); // Every 15 seconds
  }

  observeConversionEvents() {
    // Track recipe saves
    document.addEventListener('click', (event) => {
      if (event.target.matches('.save-recipe, [data-action="save"]')) {
        this.recordBusinessEvent('recipe_saved', {
          recipeId: event.target.getAttribute('data-recipe-id'),
          timestamp: Date.now()
        });
      }
    });
    
    // Track newsletter signups
    document.addEventListener('submit', (event) => {
      if (event.target.matches('.newsletter-form, [data-form="newsletter"]')) {
        this.recordBusinessEvent('newsletter_signup', {
          timestamp: Date.now(),
          source: window.location.pathname
        });
      }
    });
  }

  setupPageLifecycleTracking() {
    // Track page load complete
    window.addEventListener('load', () => {
      const loadTime = performance.now() - this.startTime;
      this.recordMetric('PAGE_LOAD_TIME', loadTime);
    });
    
    // Track page unload
    window.addEventListener('beforeunload', () => {
      this.flush(true); // Force immediate flush
    });
    
    // Track back/forward navigation
    window.addEventListener('pageshow', (event) => {
      if (event.persisted) {
        this.recordMetric('BF_CACHE_RESTORE', performance.now());
      }
    });
  }

  setupHTMXTracking() {
    if (typeof htmx !== 'undefined') {
      // Track HTMX request performance
      document.addEventListener('htmx:beforeRequest', (event) => {
        const startTime = performance.now();
        event.target.setAttribute('data-htmx-start', startTime);
      });
      
      document.addEventListener('htmx:afterRequest', (event) => {
        const startTime = parseFloat(event.target.getAttribute('data-htmx-start'));
        const duration = performance.now() - startTime;
        
        this.recordMetric('HTMX_REQUEST_TIME', duration, {
          method: event.detail.xhr.method || 'GET',
          url: event.detail.xhr.responseURL,
          status: event.detail.xhr.status,
          successful: event.detail.xhr.status < 400
        });
        
        event.target.removeAttribute('data-htmx-start');
      });
      
      // Track HTMX swap performance
      document.addEventListener('htmx:beforeSwap', (event) => {
        const startTime = performance.now();
        event.target.setAttribute('data-htmx-swap-start', startTime);
      });
      
      document.addEventListener('htmx:afterSwap', (event) => {
        const startTime = parseFloat(event.target.getAttribute('data-htmx-swap-start'));
        const duration = performance.now() - startTime;
        
        this.recordMetric('HTMX_SWAP_TIME', duration);
        event.target.removeAttribute('data-htmx-swap-start');
      });
    }
  }

  recordMetric(name, value, metadata = {}) {
    this.measurements.push({
      id: this.generateId(),
      sessionId: this.sessionId,
      pageViewId: this.pageViewId,
      userId: this.userId,
      timestamp: Date.now(),
      url: window.location.href,
      userAgent: navigator.userAgent,
      deviceInfo: this.deviceInfo,
      networkInfo: this.networkInfo,
      performanceData: {
        [name]: value,
        ...metadata
      },
      customData: {
        pageTitle: document.title,
        referrer: document.referrer,
        utmParameters: this.getUTMParameters(),
        experiments: this.getActiveExperiments(),
        featureFlags: this.getFeatureFlags()
      }
    });
    
    this.scheduleFlush();
  }

  recordInteraction(interaction) {
    this.interactions.push({
      ...interaction,
      sessionId: this.sessionId,
      pageViewId: this.pageViewId
    });
  }

  recordBusinessEvent(eventType, data) {
    this.businessEvents.push({
      eventType: eventType,
      timestamp: Date.now(),
      sessionId: this.sessionId,
      pageViewId: this.pageViewId,
      data: data
    });
    
    this.scheduleFlush();
  }

  scheduleFlush(delay = this.config.flushInterval) {
    if (this.flushTimeout) {
      clearTimeout(this.flushTimeout);
    }
    
    this.flushTimeout = setTimeout(() => {
      this.flush();
    }, delay);
  }

  startPeriodicFlush() {
    setInterval(() => {
      if (this.measurements.length > 0 || this.interactions.length > 0 || this.businessEvents.length > 0) {
        this.flush();
      }
    }, this.config.flushInterval);
  }

  async flush(immediate = false) {
    if (this.measurements.length === 0 && this.interactions.length === 0 && this.businessEvents.length === 0) {
      return;
    }
    
    const payload = {
      measurements: this.measurements.splice(0),
      interactions: this.interactions.splice(0),
      businessEvents: this.businessEvents.splice(0),
      metadata: {
        sessionId: this.sessionId,
        pageViewId: this.pageViewId,
        timestamp: Date.now(),
        url: window.location.href,
        immediate: immediate
      }
    };
    
    try {
      if (immediate && 'sendBeacon' in navigator) {
        // Use sendBeacon for immediate flush (page unload)
        navigator.sendBeacon(
          this.config.endpoint,
          JSON.stringify(payload)
        );
      } else {
        // Use fetch for regular flushes
        await fetch(this.config.endpoint, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(payload),
          keepalive: immediate
        });
      }
    } catch (error) {
      console.warn('Failed to send RUM data:', error);
      // Put data back in queue for retry
      if (!immediate) {
        this.measurements.unshift(...payload.measurements);
        this.interactions.unshift(...payload.interactions);
        this.businessEvents.unshift(...payload.businessEvents);
      }
    }
  }

  // Utility methods
  generateId() {
    return Date.now().toString(36) + Math.random().toString(36).substr(2);
  }

  getDeviceType() {
    const ua = navigator.userAgent;
    if (/tablet|ipad|playbook|silk/i.test(ua)) return 'tablet';
    if (/mobile|iphone|ipod|android|blackberry|opera|mini|windows\sce|palm|smartphone|iemobile/i.test(ua)) return 'mobile';
    return 'desktop';
  }

  getOS(ua) {
    if (ua.includes('Windows')) return 'Windows';
    if (ua.includes('Mac')) return 'macOS';
    if (ua.includes('Linux')) return 'Linux';
    if (ua.includes('Android')) return 'Android';
    if (ua.includes('iOS')) return 'iOS';
    return 'Unknown';
  }

  getBrowser(ua) {
    if (ua.includes('Chrome')) return 'Chrome';
    if (ua.includes('Firefox')) return 'Firefox';
    if (ua.includes('Safari')) return 'Safari';
    if (ua.includes('Edge')) return 'Edge';
    return 'Unknown';
  }

  categorizeConnectionSpeed(effectiveType) {
    switch (effectiveType) {
      case 'slow-2g':
      case '2g':
        return 'slow';
      case '3g':
        return 'medium';
      case '4g':
        return 'fast';
      default:
        return 'unknown';
    }
  }

  getElementSelector(element) {
    if (element.id) return `#${element.id}`;
    if (element.className) return `.${element.className.split(' ')[0]}`;
    return element.tagName.toLowerCase();
  }

  getUTMParameters() {
    const params = new URLSearchParams(window.location.search);
    const utm = {};
    for (const [key, value] of params) {
      if (key.startsWith('utm_')) {
        utm[key] = value;
      }
    }
    return utm;
  }

  getActiveExperiments() {
    // Placeholder for A/B testing integration
    return window.experiments || {};
  }

  getFeatureFlags() {
    // Placeholder for feature flag integration
    return window.featureFlags || {};
  }

  throttle(func, delay) {
    let timeoutId;
    let lastExecTime = 0;
    return function (...args) {
      const currentTime = Date.now();
      
      if (currentTime - lastExecTime > delay) {
        func.apply(this, args);
        lastExecTime = currentTime;
      } else {
        clearTimeout(timeoutId);
        timeoutId = setTimeout(() => {
          func.apply(this, args);
          lastExecTime = Date.now();
        }, delay - (currentTime - lastExecTime));
      }
    };
  }

  debounce(func, delay) {
    let timeoutId;
    return function (...args) {
      clearTimeout(timeoutId);
      timeoutId = setTimeout(() => func.apply(this, args), delay);
    };
  }
}

// Auto-initialize RUM if config is available
if (typeof window !== 'undefined') {
  window.AlchemorselRUM = AlchemorselRUM;
  
  // Auto-start if configuration is present
  if (window.rumConfig) {
    window.rum = new AlchemorselRUM(window.rumConfig);
  }
}

// Export for module systems
if (typeof module !== 'undefined' && module.exports) {
  module.exports = AlchemorselRUM;
}