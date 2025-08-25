// Package performance provides Interaction to Next Paint (INP) optimization
package performance

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"time"
)

// INPEnhancer optimizes Interaction to Next Paint performance for HTMX
type INPEnhancer struct {
	config                INPConfig
	interactionOptimizer  *InteractionOptimizer
	htmxOptimizer        *HTMXINPOptimizer
	taskScheduler        *TaskScheduler
	performanceMetrics   INPMetrics
}

// INPConfig configures INP optimization
type INPConfig struct {
	EnableTaskScheduling       bool          // Enable task scheduling for better INP
	EnableHTMXOptimization     bool          // Enable HTMX-specific optimizations
	EnableProgressiveLoading   bool          // Enable progressive loading
	TargetINP                 time.Duration // Target INP (200ms for "Good")
	MaxTaskDuration           time.Duration // Maximum task duration before yielding
	EnableUserInputPriority   bool          // Prioritize user input handling
	EnableMainThreadUnblocking bool          // Enable main thread unblocking
	EnableVirtualScrolling    bool          // Enable virtual scrolling for long lists
	DebounceDelay            time.Duration // Debounce delay for rapid interactions
}

// InteractionOptimizer handles general interaction optimizations
type InteractionOptimizer struct {
	debouncedEvents    map[string]time.Duration
	throttledEvents    map[string]time.Duration
	priorityEvents     []string
	inputOptimizations []InputOptimization
	touchOptimizations []TouchOptimization
}

// HTMXINPOptimizer handles HTMX-specific INP optimizations
type HTMXINPOptimizer struct {
	requestQueue         []HTMXRequest
	responseCache        map[string]CachedResponse
	progressiveElements  []ProgressiveElement
	optimisticUpdates    map[string]OptimisticUpdate
	requestDebouncing    map[string]time.Duration
}

// TaskScheduler manages JavaScript task scheduling for better INP
type TaskScheduler struct {
	taskQueue         []ScheduledTask
	maxTaskDuration   time.Duration
	yieldThreshold    time.Duration
	priorityLevels    map[string]int
}

// InputOptimization represents an input optimization strategy
type InputOptimization struct {
	Selector    string
	EventType   string
	Strategy    string // debounce, throttle, immediate
	Delay       time.Duration
	Priority    int
}

// TouchOptimization represents touch interaction optimizations
type TouchOptimization struct {
	Selector        string
	EnablePassive   bool
	EnableFastTap   bool
	PreventDefault  bool
	TouchAction     string
}

// HTMXRequest represents an HTMX request for optimization
type HTMXRequest struct {
	ID          string
	URL         string
	Method      string
	Element     string
	Priority    int
	Timestamp   time.Time
	Debounce    time.Duration
}

// CachedResponse represents a cached HTMX response
type CachedResponse struct {
	Content   string
	Timestamp time.Time
	TTL       time.Duration
	Headers   map[string]string
}

// ProgressiveElement represents an element with progressive enhancement
type ProgressiveElement struct {
	Selector      string
	LoadStrategy  string
	Dependencies  []string
	FallbackHTML  string
	Priority      int
}

// OptimisticUpdate represents an optimistic UI update
type OptimisticUpdate struct {
	Element        string
	UpdateHTML     string
	RollbackHTML   string
	Timestamp      time.Time
	Confirmed      bool
}

// ScheduledTask represents a scheduled JavaScript task
type ScheduledTask struct {
	ID          string
	Function    string
	Priority    int
	MaxDuration time.Duration
	Timestamp   time.Time
}

// INPMetrics tracks INP optimization performance
type INPMetrics struct {
	TotalInteractions       int
	OptimizedInteractions   int
	AverageINPImprovement   time.Duration
	HTMXRequestsOptimized   int
	TasksScheduled          int
	MainThreadBlocks        int
	DebouncedEvents         int
	ThrottledEvents         int
	ProgressiveLoads        int
	OptimisticUpdates       int
	LastOptimization        time.Time
}

// DefaultINPConfig returns sensible INP optimization defaults
func DefaultINPConfig() INPConfig {
	return INPConfig{
		EnableTaskScheduling:       true,
		EnableHTMXOptimization:     true,
		EnableProgressiveLoading:   true,
		TargetINP:                 200 * time.Millisecond, // 200ms Google "Good" threshold
		MaxTaskDuration:           16 * time.Millisecond,  // ~60fps
		EnableUserInputPriority:   true,
		EnableMainThreadUnblocking: true,
		EnableVirtualScrolling:    true,
		DebounceDelay:            100 * time.Millisecond,
	}
}

// NewINPEnhancer creates a new INP enhancer
func NewINPEnhancer(config INPConfig) *INPEnhancer {
	interactionOptimizer := &InteractionOptimizer{
		debouncedEvents: map[string]time.Duration{
			"input":    250 * time.Millisecond,
			"keyup":    300 * time.Millisecond,
			"scroll":   100 * time.Millisecond,
			"resize":   150 * time.Millisecond,
		},
		throttledEvents: map[string]time.Duration{
			"mousemove": 16 * time.Millisecond,  // ~60fps
			"touchmove": 16 * time.Millisecond,
			"scroll":    16 * time.Millisecond,
		},
		priorityEvents: []string{
			"click", "submit", "keydown", "touchstart",
		},
		inputOptimizations: []InputOptimization{
			{
				Selector:  "input[type='search']",
				EventType: "input",
				Strategy:  "debounce",
				Delay:     300 * time.Millisecond,
				Priority:  1,
			},
			{
				Selector:  "form",
				EventType: "submit",
				Strategy:  "immediate",
				Priority:  10,
			},
		},
		touchOptimizations: []TouchOptimization{
			{
				Selector:       "button, .btn, [role='button']",
				EnablePassive:  false,
				EnableFastTap:  true,
				PreventDefault: false,
				TouchAction:    "manipulation",
			},
			{
				Selector:       ".scroll-container",
				EnablePassive:  true,
				TouchAction:    "pan-y",
			},
		},
	}

	htmxOptimizer := &HTMXINPOptimizer{
		requestQueue:    []HTMXRequest{},
		responseCache:   make(map[string]CachedResponse),
		optimisticUpdates: make(map[string]OptimisticUpdate),
		requestDebouncing: map[string]time.Duration{
			"hx-get":     200 * time.Millisecond,
			"hx-post":    100 * time.Millisecond,
			"hx-delete":  50 * time.Millisecond,
			"hx-trigger": 150 * time.Millisecond,
		},
	}

	taskScheduler := &TaskScheduler{
		taskQueue:       []ScheduledTask{},
		maxTaskDuration: config.MaxTaskDuration,
		yieldThreshold:  5 * time.Millisecond,
		priorityLevels: map[string]int{
			"user-input":     10,
			"htmx-response":  8,
			"animation":      6,
			"background":     2,
		},
	}

	return &INPEnhancer{
		config:               config,
		interactionOptimizer: interactionOptimizer,
		htmxOptimizer:       htmxOptimizer,
		taskScheduler:       taskScheduler,
		performanceMetrics:  INPMetrics{},
	}
}

// OptimizeHTML optimizes HTML for better INP performance
func (inp *INPEnhancer) OptimizeHTML(html string) (string, error) {
	optimized := html

	// Step 1: Optimize HTMX interactions
	if inp.config.EnableHTMXOptimization {
		optimized = inp.optimizeHTMXInteractions(optimized)
	}

	// Step 2: Add input optimizations
	optimized = inp.optimizeInputHandling(optimized)

	// Step 3: Add touch optimizations
	optimized = inp.optimizeTouchInteractions(optimized)

	// Step 4: Add task scheduling
	if inp.config.EnableTaskScheduling {
		optimized = inp.addTaskScheduling(optimized)
	}

	// Step 5: Add progressive enhancement
	if inp.config.EnableProgressiveLoading {
		optimized = inp.addProgressiveEnhancement(optimized)
	}

	// Step 6: Add virtual scrolling for long lists
	if inp.config.EnableVirtualScrolling {
		optimized = inp.addVirtualScrolling(optimized)
	}

	// Update metrics
	inp.updateMetrics()

	return optimized, nil
}

// optimizeHTMXInteractions optimizes HTMX interactions for better INP
func (inp *INPEnhancer) optimizeHTMXInteractions(html string) string {
	optimized := html

	// Add debouncing to HTMX triggers
	optimized = inp.addHTMXDebouncing(optimized)

	// Add optimistic updates
	optimized = inp.addOptimisticUpdates(optimized)

	// Add request caching
	optimized = inp.addHTMXCaching(optimized)

	// Add loading states
	optimized = inp.addLoadingStates(optimized)

	return optimized
}

// addHTMXDebouncing adds debouncing to HTMX triggers
func (inp *INPEnhancer) addHTMXDebouncing(html string) string {
	// Find HTMX elements with triggers that benefit from debouncing
	htmxRegex := regexp.MustCompile(`<([^>]*?\bhx-trigger="([^"]*?)"[^>]*?)>`)
	
	return htmxRegex.ReplaceAllStringFunc(html, func(match string) string {
		triggerMatch := regexp.MustCompile(`hx-trigger="([^"]*?)"`).FindStringSubmatch(match)
		if len(triggerMatch) < 2 {
			return match
		}

		trigger := triggerMatch[1]
		
		// Add debouncing for input events
		if strings.Contains(trigger, "input") || strings.Contains(trigger, "keyup") {
			if !strings.Contains(trigger, "delay:") {
				newTrigger := strings.Replace(trigger, "input", "input delay:300ms", 1)
				newTrigger = strings.Replace(newTrigger, "keyup", "keyup delay:250ms", 1)
				return strings.Replace(match, fmt.Sprintf(`hx-trigger="%s"`, trigger), 
					fmt.Sprintf(`hx-trigger="%s"`, newTrigger), 1)
			}
		}

		// Add throttling for scroll events
		if strings.Contains(trigger, "scroll") && !strings.Contains(trigger, "throttle:") {
			newTrigger := strings.Replace(trigger, "scroll", "scroll throttle:16ms", 1)
			return strings.Replace(match, fmt.Sprintf(`hx-trigger="%s"`, trigger), 
				fmt.Sprintf(`hx-trigger="%s"`, newTrigger), 1)
		}

		return match
	})
}

// addOptimisticUpdates adds optimistic UI updates for better perceived performance
func (inp *INPEnhancer) addOptimisticUpdates(html string) string {
	// Find forms that benefit from optimistic updates
	formRegex := regexp.MustCompile(`<form([^>]*?\bhx-post[^>]*?)>`)
	
	return formRegex.ReplaceAllStringFunc(html, func(match string) string {
		if !strings.Contains(match, "hx-indicator") {
			// Add loading indicator
			match = strings.Replace(match, "<form", 
				`<form hx-indicator="#loading-indicator"`, 1)
		}

		// Add optimistic update attributes
		if !strings.Contains(match, "hx-swap") {
			match = strings.Replace(match, "<form", 
				`<form hx-swap="outerHTML show:no-scroll"`, 1)
		}

		return match
	})
}

// addHTMXCaching adds response caching for HTMX requests
func (inp *INPEnhancer) addHTMXCaching(html string) string {
	// Add caching headers and attributes
	htmxGetRegex := regexp.MustCompile(`<([^>]*?\bhx-get[^>]*?)>`)
	
	return htmxGetRegex.ReplaceAllStringFunc(html, func(match string) string {
		// Add cache headers for GET requests
		if !strings.Contains(match, "hx-headers") {
			match = strings.Replace(match, "hx-get", 
				`hx-headers='{"Cache-Control": "max-age=60"}' hx-get`, 1)
		}

		return match
	})
}

// addLoadingStates adds loading states to prevent layout shift during interactions
func (inp *INPEnhancer) addLoadingStates(html string) string {
	optimized := html

	// Add global loading indicator CSS if not present
	if !strings.Contains(optimized, "htmx-loading") {
		loadingCSS := `
<style>
.htmx-loading {
  opacity: 0.7;
  transition: opacity 0.2s ease;
  pointer-events: none;
}

.htmx-request .htmx-indicator {
  display: inline-block;
}

.htmx-indicator {
  display: none;
  width: 16px;
  height: 16px;
  border: 2px solid transparent;
  border-top: 2px solid #3498db;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.htmx-request {
  position: relative;
}

.htmx-request::after {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(255, 255, 255, 0.1);
  pointer-events: none;
}
</style>`

		// Insert CSS before closing head tag
		headEndRegex := regexp.MustCompile(`</head>`)
		optimized = headEndRegex.ReplaceAllString(optimized, loadingCSS+"\n</head>")
	}

	return optimized
}

// optimizeInputHandling optimizes input handling for better INP
func (inp *INPEnhancer) optimizeInputHandling(html string) string {
	optimized := html

	// Add input optimizations
	for _, opt := range inp.interactionOptimizer.inputOptimizations {
		optimized = inp.applyInputOptimization(optimized, opt)
	}

	return optimized
}

// applyInputOptimization applies a specific input optimization
func (inp *INPEnhancer) applyInputOptimization(html string, opt InputOptimization) string {
	// This is a simplified implementation
	// In practice, you'd add event listeners with optimized handling
	return html
}

// optimizeTouchInteractions optimizes touch interactions for better INP
func (inp *INPEnhancer) optimizeTouchInteractions(html string) string {
	optimized := html

	// Add touch optimizations
	touchOptCSS := `
<style>
/* Touch optimization styles */
button, .btn, [role="button"] {
  touch-action: manipulation;
  user-select: none;
  -webkit-tap-highlight-color: transparent;
}

.scroll-container {
  touch-action: pan-y;
  overflow-scrolling: touch;
  -webkit-overflow-scrolling: touch;
}

.no-touch-delay {
  touch-action: manipulation;
}

/* Fast tap optimization */
.fast-tap {
  cursor: pointer;
  -webkit-user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  user-select: none;
}

.fast-tap:active {
  transform: scale(0.98);
  transition: transform 0.1s ease;
}
</style>`

	// Insert CSS before closing head tag
	headEndRegex := regexp.MustCompile(`</head>`)
	optimized = headEndRegex.ReplaceAllString(optimized, touchOptCSS+"\n</head>")

	// Apply touch optimizations to elements
	for _, touchOpt := range inp.interactionOptimizer.touchOptimizations {
		optimized = inp.applyTouchOptimization(optimized, touchOpt)
	}

	return optimized
}

// applyTouchOptimization applies touch optimizations to matching elements
func (inp *INPEnhancer) applyTouchOptimization(html string, opt TouchOptimization) string {
	// Add touch-action and other attributes to matching elements
	// This is a simplified implementation
	if opt.Selector == "button, .btn, [role='button']" {
		buttonRegex := regexp.MustCompile(`<button([^>]*?)>`)
		html = buttonRegex.ReplaceAllStringFunc(html, func(match string) string {
			if !strings.Contains(match, "class=") {
				return strings.Replace(match, "<button", `<button class="fast-tap no-touch-delay"`, 1)
			} else {
				classRegex := regexp.MustCompile(`class="([^"]*?)"`)
				return classRegex.ReplaceAllStringFunc(match, func(classMatch string) string {
					return strings.Replace(classMatch, `class="`, `class="fast-tap no-touch-delay `, 1)
				})
			}
		})
	}

	return html
}

// addTaskScheduling adds JavaScript task scheduling for better INP
func (inp *INPEnhancer) addTaskScheduling(html string) string {
	taskSchedulingJS := `
<script>
// Task Scheduler for better INP
class TaskScheduler {
  constructor() {
    this.taskQueue = [];
    this.isProcessing = false;
    this.maxTaskTime = 16; // ~60fps
  }

  schedule(task, priority = 1) {
    this.taskQueue.push({ task, priority, timestamp: performance.now() });
    this.taskQueue.sort((a, b) => b.priority - a.priority);
    
    if (!this.isProcessing) {
      this.processTasks();
    }
  }

  async processTasks() {
    this.isProcessing = true;
    
    while (this.taskQueue.length > 0) {
      const start = performance.now();
      
      // Process tasks until time limit
      while (this.taskQueue.length > 0 && (performance.now() - start) < this.maxTaskTime) {
        const { task } = this.taskQueue.shift();
        try {
          await task();
        } catch (error) {
          console.error('Task execution error:', error);
        }
      }
      
      // Yield to browser if more tasks remain
      if (this.taskQueue.length > 0) {
        await this.yieldToMain();
      }
    }
    
    this.isProcessing = false;
  }

  yieldToMain() {
    return new Promise(resolve => {
      if ('scheduler' in window && 'postTask' in scheduler) {
        scheduler.postTask(resolve, { priority: 'user-blocking' });
      } else {
        setTimeout(resolve, 0);
      }
    });
  }
}

// Global task scheduler instance
window.taskScheduler = new TaskScheduler();

// Optimize HTMX requests
document.addEventListener('htmx:beforeRequest', function(event) {
  // Schedule HTMX processing with appropriate priority
  const priority = event.target.hasAttribute('hx-priority') ? 
    parseInt(event.target.getAttribute('hx-priority')) : 5;
    
  event.target.setAttribute('data-request-start', performance.now());
});

document.addEventListener('htmx:afterRequest', function(event) {
  const startTime = parseFloat(event.target.getAttribute('data-request-start'));
  const duration = performance.now() - startTime;
  
  // Schedule response processing
  window.taskScheduler.schedule(() => {
    // Process response with yielding for large responses
    if (event.detail.xhr.response.length > 10000) {
      return processLargeResponse(event.detail.xhr.response);
    }
  }, 8);
  
  event.target.removeAttribute('data-request-start');
});

function processLargeResponse(response) {
  return new Promise(resolve => {
    const chunks = chunkResponse(response, 1000);
    let index = 0;
    
    function processChunk() {
      if (index < chunks.length) {
        // Process chunk
        index++;
        window.taskScheduler.schedule(processChunk, 6);
      } else {
        resolve();
      }
    }
    
    processChunk();
  });
}

function chunkResponse(response, chunkSize) {
  const chunks = [];
  for (let i = 0; i < response.length; i += chunkSize) {
    chunks.push(response.slice(i, i + chunkSize));
  }
  return chunks;
}

// Input debouncing optimization
function createDebouncedHandler(handler, delay) {
  let timeoutId;
  return function(...args) {
    clearTimeout(timeoutId);
    timeoutId = setTimeout(() => {
      window.taskScheduler.schedule(() => handler.apply(this, args), 7);
    }, delay);
  };
}

// Auto-apply debouncing to inputs
document.addEventListener('DOMContentLoaded', function() {
  // Debounce search inputs
  document.querySelectorAll('input[type="search"], input[data-search]').forEach(input => {
    const originalHandler = input.oninput;
    if (originalHandler) {
      input.oninput = createDebouncedHandler(originalHandler, 300);
    }
  });
  
  // Optimize scroll handlers
  document.querySelectorAll('[data-scroll-optimized]').forEach(element => {
    let isScrolling = false;
    element.addEventListener('scroll', function(e) {
      if (!isScrolling) {
        isScrolling = true;
        window.taskScheduler.schedule(() => {
          // Process scroll event
          isScrolling = false;
        }, 3);
      }
    }, { passive: true });
  });
});

// Long task detection and splitting
function isLongTask(startTime) {
  return performance.now() - startTime > 50; // 50ms threshold
}

// Performance monitoring for INP
let inpMeasurements = [];

function measureINP(event) {
  const startTime = performance.now();
  
  // Use requestIdleCallback or setTimeout to measure after paint
  if ('requestIdleCallback' in window) {
    requestIdleCallback(() => {
      const inp = performance.now() - startTime;
      inpMeasurements.push({
        type: event.type,
        inp: inp,
        timestamp: startTime,
        target: event.target.tagName
      });
      
      // Keep only recent measurements
      if (inpMeasurements.length > 100) {
        inpMeasurements = inpMeasurements.slice(-100);
      }
    });
  }
}

// Monitor key interaction events
['click', 'keydown', 'input'].forEach(eventType => {
  document.addEventListener(eventType, measureINP, { passive: true });
});
</script>`

	// Insert script before closing body tag
	bodyEndRegex := regexp.MustCompile(`</body>`)
	return bodyEndRegex.ReplaceAllString(html, taskSchedulingJS+"\n</body>")
}

// addProgressiveEnhancement adds progressive enhancement for better perceived performance
func (inp *INPEnhancer) addProgressiveEnhancement(html string) string {
	progressiveJS := `
<script>
// Progressive Enhancement for HTMX
class ProgressiveEnhancer {
  constructor() {
    this.enhancedElements = new Set();
    this.observer = new IntersectionObserver(
      this.handleIntersection.bind(this),
      { threshold: 0.1, rootMargin: '100px' }
    );
  }

  enhance(element) {
    if (this.enhancedElements.has(element)) return;
    
    this.enhancedElements.add(element);
    
    // Progressive HTMX loading
    if (element.hasAttribute('data-progressive-htmx')) {
      this.observer.observe(element);
    }
    
    // Lazy load HTMX content
    if (element.hasAttribute('data-lazy-htmx')) {
      this.setupLazyHTMX(element);
    }
  }

  handleIntersection(entries) {
    entries.forEach(entry => {
      if (entry.isIntersecting) {
        const element = entry.target;
        
        // Activate HTMX on intersection
        if (element.hasAttribute('data-progressive-htmx')) {
          const url = element.getAttribute('data-progressive-htmx');
          element.setAttribute('hx-get', url);
          element.setAttribute('hx-trigger', 'intersect once');
          
          if (window.htmx) {
            htmx.process(element);
          }
        }
        
        this.observer.unobserve(element);
      }
    });
  }

  setupLazyHTMX(element) {
    // Setup lazy loading with user interaction
    const trigger = element.getAttribute('data-lazy-trigger') || 'click';
    const url = element.getAttribute('data-lazy-htmx');
    
    element.addEventListener(trigger, function() {
      if (!element.hasAttribute('hx-get')) {
        element.setAttribute('hx-get', url);
        element.setAttribute('hx-swap', 'outerHTML');
        
        if (window.htmx) {
          htmx.process(element);
          htmx.trigger(element, trigger);
        }
      }
    }, { once: true });
  }
}

// Initialize progressive enhancer
window.progressiveEnhancer = new ProgressiveEnhancer();

// Auto-enhance elements on page load and HTMX updates
document.addEventListener('DOMContentLoaded', function() {
  document.querySelectorAll('[data-progressive-htmx], [data-lazy-htmx]').forEach(element => {
    window.progressiveEnhancer.enhance(element);
  });
});

document.addEventListener('htmx:afterSwap', function(event) {
  event.target.querySelectorAll('[data-progressive-htmx], [data-lazy-htmx]').forEach(element => {
    window.progressiveEnhancer.enhance(element);
  });
});
</script>`

	// Insert script before closing body tag
	bodyEndRegex := regexp.MustCompile(`</body>`)
	return bodyEndRegex.ReplaceAllString(html, progressiveJS+"\n</body>")
}

// addVirtualScrolling adds virtual scrolling for long lists
func (inp *INPEnhancer) addVirtualScrolling(html string) string {
	// Find long lists that would benefit from virtual scrolling
	listRegex := regexp.MustCompile(`<(?:ul|ol|div)\s+class="[^"]*(?:recipe-list|search-results|infinite-list)[^"]*"[^>]*>`)
	
	virtualScrollJS := `
<script>
// Virtual Scrolling for Long Lists
class VirtualScroller {
  constructor(container, options = {}) {
    this.container = container;
    this.itemHeight = options.itemHeight || 100;
    this.bufferSize = options.bufferSize || 5;
    this.items = [];
    this.visibleStart = 0;
    this.visibleEnd = 0;
    
    this.setupVirtualScrolling();
  }

  setupVirtualScrolling() {
    this.container.style.position = 'relative';
    this.container.style.overflow = 'auto';
    
    this.container.addEventListener('scroll', this.handleScroll.bind(this), { passive: true });
    
    // Initial render
    this.updateVisibleItems();
  }

  setItems(items) {
    this.items = items;
    this.updateVisibleItems();
  }

  handleScroll() {
    // Throttle scroll handling
    if (!this.scrollTimeout) {
      this.scrollTimeout = setTimeout(() => {
        this.updateVisibleItems();
        this.scrollTimeout = null;
      }, 16); // ~60fps
    }
  }

  updateVisibleItems() {
    const containerHeight = this.container.clientHeight;
    const scrollTop = this.container.scrollTop;
    
    this.visibleStart = Math.floor(scrollTop / this.itemHeight);
    this.visibleEnd = Math.min(
      this.items.length,
      this.visibleStart + Math.ceil(containerHeight / this.itemHeight) + this.bufferSize
    );
    
    this.renderVisibleItems();
  }

  renderVisibleItems() {
    // Clear container
    this.container.innerHTML = '';
    
    // Create spacer for items above viewport
    if (this.visibleStart > 0) {
      const topSpacer = document.createElement('div');
      topSpacer.style.height = (this.visibleStart * this.itemHeight) + 'px';
      this.container.appendChild(topSpacer);
    }
    
    // Render visible items
    for (let i = this.visibleStart; i < this.visibleEnd; i++) {
      if (this.items[i]) {
        const itemElement = this.createItemElement(this.items[i], i);
        this.container.appendChild(itemElement);
      }
    }
    
    // Create spacer for items below viewport
    const remainingItems = this.items.length - this.visibleEnd;
    if (remainingItems > 0) {
      const bottomSpacer = document.createElement('div');
      bottomSpacer.style.height = (remainingItems * this.itemHeight) + 'px';
      this.container.appendChild(bottomSpacer);
    }
  }

  createItemElement(item, index) {
    const element = document.createElement('div');
    element.className = 'virtual-scroll-item';
    element.style.height = this.itemHeight + 'px';
    element.innerHTML = item.html || item.toString();
    element.dataset.index = index;
    return element;
  }
}

// Auto-setup virtual scrolling for long lists
document.addEventListener('DOMContentLoaded', function() {
  document.querySelectorAll('.recipe-list, .search-results, .infinite-list').forEach(list => {
    // Only virtualize if many items
    const items = list.children.length;
    if (items > 20) {
      const virtualScroller = new VirtualScroller(list, {
        itemHeight: 120, // Adjust based on your item height
        bufferSize: 3
      });
      
      // Convert existing items to virtual format
      const existingItems = Array.from(list.children).map(child => ({
        html: child.outerHTML
      }));
      
      virtualScroller.setItems(existingItems);
    }
  });
});
</script>`

	// Insert script before closing body tag
	bodyEndRegex := regexp.MustCompile(`</body>`)
	return bodyEndRegex.ReplaceAllString(html, virtualScrollJS+"\n</body>")
}

// updateMetrics updates INP optimization metrics
func (inp *INPEnhancer) updateMetrics() {
	inp.performanceMetrics.TotalInteractions++
	inp.performanceMetrics.OptimizedInteractions++
	inp.performanceMetrics.LastOptimization = time.Now()
}

// GenerateReport generates an INP optimization report
func (inp *INPEnhancer) GenerateReport() string {
	metrics := inp.performanceMetrics
	
	return fmt.Sprintf(`=== INP Enhancement Report ===
Last Optimization: %s
Total Interactions: %d
Optimized Interactions: %d
HTMX Requests Optimized: %d
Tasks Scheduled: %d
Main Thread Blocks: %d
Debounced Events: %d
Throttled Events: %d
Progressive Loads: %d
Optimistic Updates: %d

=== Configuration ===
Target INP: %v
Task Scheduling: %t
HTMX Optimization: %t
Progressive Loading: %t
Virtual Scrolling: %t
User Input Priority: %t

=== Performance Impact ===
Average INP Improvement: %v
Main Thread Unblocking: %t
Debounce Delay: %v
Max Task Duration: %v
`,
		metrics.LastOptimization.Format(time.RFC3339),
		metrics.TotalInteractions,
		metrics.OptimizedInteractions,
		metrics.HTMXRequestsOptimized,
		metrics.TasksScheduled,
		metrics.MainThreadBlocks,
		metrics.DebouncedEvents,
		metrics.ThrottledEvents,
		metrics.ProgressiveLoads,
		metrics.OptimisticUpdates,
		inp.config.TargetINP,
		inp.config.EnableTaskScheduling,
		inp.config.EnableHTMXOptimization,
		inp.config.EnableProgressiveLoading,
		inp.config.EnableVirtualScrolling,
		inp.config.EnableUserInputPriority,
		metrics.AverageINPImprovement,
		inp.config.EnableMainThreadUnblocking,
		inp.config.DebounceDelay,
		inp.config.MaxTaskDuration,
	)
}

// GetMetrics returns current INP optimization metrics
func (inp *INPEnhancer) GetMetrics() INPMetrics {
	return inp.performanceMetrics
}

// TemplateFunction returns template functions for INP optimization
func (inp *INPEnhancer) TemplateFunction() template.FuncMap {
	return template.FuncMap{
		"optimizeINP": func(content string) template.HTML {
			optimized, err := inp.OptimizeHTML(content)
			if err != nil {
				return template.HTML(content)
			}
			return template.HTML(optimized)
		},
		"debouncedInput": func(selector string, delay int) template.HTML {
			return template.HTML(fmt.Sprintf(
				`<script>document.querySelector('%s').addEventListener('input', createDebouncedHandler(function(e) { /* handler */ }, %d));</script>`,
				selector, delay))
		},
		"progressiveHTMX": func(url string, trigger string) template.HTML {
			return template.HTML(fmt.Sprintf(
				`data-progressive-htmx="%s" data-lazy-trigger="%s"`,
				url, trigger))
		},
		"optimisticUpdate": func(content string) template.HTML {
			return template.HTML(fmt.Sprintf(
				`<div class="optimistic-update" data-content="%s">%s</div>`,
				template.HTMLEscapeString(content), content))
		},
	}
}