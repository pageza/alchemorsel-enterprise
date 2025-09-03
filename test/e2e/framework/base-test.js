/**
 * Base Test Infrastructure for Alchemorsel v3 E2E Testing
 * 
 * This is the foundation for comprehensive E2E testing that goes far beyond
 * basic user journeys to catch edge cases, race conditions, and integration issues.
 */

const puppeteer = require('puppeteer');
const fs = require('fs');
const path = require('path');

// Color codes for enhanced logging
const colors = {
  reset: '\x1b[0m',
  green: '\x1b[32m',
  red: '\x1b[31m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  magenta: '\x1b[35m',
  cyan: '\x1b[36m',
  gray: '\x1b[90m',
  bold: '\x1b[1m'
};

/**
 * Enhanced Test Configuration
 */
const TestConfig = {
  // Environment URLs
  BASE_URL: process.env.BASE_URL || 'http://web:8080',
  API_URL: process.env.API_URL || 'http://api:8080',
  
  // Browser settings
  BROWSER_CONFIG: {
    headless: true,
    args: [
      '--no-sandbox',
      '--disable-setuid-sandbox',
      '--disable-dev-shm-usage',
      '--disable-gpu',
      '--disable-web-security',
      '--disable-features=IsolateOrigins',
      '--disable-site-isolation-trials',
      '--disable-background-timer-throttling',
      '--disable-backgrounding-occluded-windows',
      '--disable-renderer-backgrounding'
    ],
    ignoreHTTPSErrors: true,
    defaultViewport: { width: 1280, height: 800 }
  },
  
  // Test timing
  TIMEOUTS: {
    navigation: 30000,
    element: 10000,
    api: 15000,
    llm: 60000, // AI responses can take longer
    screenshot: 5000
  },
  
  // Test data
  VIEWPORTS: {
    desktop: { width: 1280, height: 800 },
    tablet: { width: 768, height: 1024 },
    mobile: { width: 375, height: 812 },
    wide: { width: 1920, height: 1080 }
  },
  
  // Screenshots and artifacts
  ARTIFACTS_DIR: '/tmp/e2e-artifacts',
  SCREENSHOTS_DIR: '/tmp/e2e-screenshots',
  REPORTS_DIR: '/tmp/e2e-reports'
};

/**
 * Enhanced Logging System
 */
class TestLogger {
  constructor(testName = 'E2E') {
    this.testName = testName;
    this.startTime = Date.now();
    this.stepCounter = 0;
  }
  
  log(message, level = 'info', color = 'reset') {
    const timestamp = new Date().toISOString();
    const elapsed = Date.now() - this.startTime;
    const prefix = `[${timestamp}] [${this.testName}] [+${elapsed}ms]`;
    
    switch (level) {
      case 'step':
        this.stepCounter++;
        console.log(`${colors.blue}${prefix} 📋 Step ${this.stepCounter}: ${message}${colors.reset}`);
        break;
      case 'success':
        console.log(`${colors.green}${prefix} ✅ ${message}${colors.reset}`);
        break;
      case 'error':
        console.log(`${colors.red}${prefix} ❌ ${message}${colors.reset}`);
        break;
      case 'warning':
        console.log(`${colors.yellow}${prefix} ⚠️  ${message}${colors.reset}`);
        break;
      case 'info':
        console.log(`${colors.cyan}${prefix} 🔍 ${message}${colors.reset}`);
        break;
      case 'debug':
        console.log(`${colors.gray}${prefix} 🐛 ${message}${colors.reset}`);
        break;
      default:
        console.log(`${colors[color]}${prefix} ${message}${colors.reset}`);
    }
  }
  
  step(message) { this.log(message, 'step'); }
  success(message) { this.log(message, 'success'); }
  error(message) { this.log(message, 'error'); }
  warning(message) { this.log(message, 'warning'); }
  info(message) { this.log(message, 'info'); }
  debug(message) { this.log(message, 'debug'); }
}

/**
 * Enhanced Screenshot Manager
 */
class ScreenshotManager {
  constructor(testName, logger) {
    this.testName = testName;
    this.logger = logger;
    this.screenshotCounter = 0;
    
    // Ensure directories exist
    this.ensureDirectories();
  }
  
  ensureDirectories() {
    const dirs = [TestConfig.ARTIFACTS_DIR, TestConfig.SCREENSHOTS_DIR, TestConfig.REPORTS_DIR];
    dirs.forEach(dir => {
      if (!fs.existsSync(dir)) {
        fs.mkdirSync(dir, { recursive: true });
      }
    });
  }
  
  async capture(page, name, fullPage = true) {
    try {
      this.screenshotCounter++;
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
      const filename = `${this.testName}-${this.screenshotCounter.toString().padStart(3, '0')}-${name}-${timestamp}.png`;
      const filepath = path.join(TestConfig.SCREENSHOTS_DIR, filename);
      
      await page.screenshot({ 
        path: filepath, 
        fullPage,
        timeout: TestConfig.TIMEOUTS.screenshot
      });
      
      this.logger.info(`📸 Screenshot saved: ${filename}`);
      return filepath;
    } catch (error) {
      this.logger.error(`Failed to capture screenshot ${name}: ${error.message}`);
      return null;
    }
  }
  
  async captureElement(page, selector, name) {
    try {
      const element = await page.$(selector);
      if (!element) {
        this.logger.warning(`Element not found for screenshot: ${selector}`);
        return null;
      }
      
      this.screenshotCounter++;
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
      const filename = `${this.testName}-${this.screenshotCounter.toString().padStart(3, '0')}-${name}-element-${timestamp}.png`;
      const filepath = path.join(TestConfig.SCREENSHOTS_DIR, filename);
      
      await element.screenshot({ path: filepath });
      this.logger.info(`📸 Element screenshot saved: ${filename}`);
      return filepath;
    } catch (error) {
      this.logger.error(`Failed to capture element screenshot ${name}: ${error.message}`);
      return null;
    }
  }
}

/**
 * Performance Monitor
 */
class PerformanceMonitor {
  constructor(logger) {
    this.logger = logger;
    this.metrics = {};
    this.startTimes = {};
  }
  
  startTimer(name) {
    this.startTimes[name] = Date.now();
  }
  
  endTimer(name) {
    if (this.startTimes[name]) {
      const duration = Date.now() - this.startTimes[name];
      this.metrics[name] = duration;
      this.logger.debug(`⏱️  ${name}: ${duration}ms`);
      delete this.startTimes[name];
      return duration;
    }
    return null;
  }
  
  async measurePageLoad(page, url, name) {
    this.startTimer(name);
    await page.goto(url, { waitUntil: 'networkidle2', timeout: TestConfig.TIMEOUTS.navigation });
    return this.endTimer(name);
  }
  
  getMetrics() {
    return { ...this.metrics };
  }
  
  logMetrics() {
    this.logger.info('📊 Performance Metrics:');
    Object.entries(this.metrics).forEach(([name, duration]) => {
      this.logger.info(`   ${name}: ${duration}ms`);
    });
  }
}

/**
 * Base Test Class - Foundation for all E2E tests
 */
class BaseTest {
  constructor(testName, options = {}) {
    this.testName = testName;
    this.options = { ...TestConfig, ...options };
    this.logger = new TestLogger(testName);
    this.screenshots = new ScreenshotManager(testName, this.logger);
    this.performance = new PerformanceMonitor(this.logger);
    
    this.browser = null;
    this.page = null;
    this.errors = [];
    this.warnings = [];
    
    this.logger.info(`🚀 Initializing test: ${testName}`);
  }
  
  /**
   * Setup - Launch browser and create page
   */
  async setup() {
    try {
      this.logger.step('Setting up browser environment');
      
      this.browser = await puppeteer.launch(this.options.BROWSER_CONFIG);
      this.page = await this.browser.newPage();
      
      // Set default viewport
      await this.page.setViewport(this.options.VIEWPORTS.desktop);
      
      // Enhanced error handling
      this.page.on('console', msg => {
        const type = msg.type();
        if (type === 'error') {
          this.logger.error(`Browser console error: ${msg.text()}`);
          this.errors.push(`Console: ${msg.text()}`);
        } else if (type === 'warning') {
          this.logger.warning(`Browser console warning: ${msg.text()}`);
          this.warnings.push(`Console: ${msg.text()}`);
        }
      });
      
      this.page.on('pageerror', error => {
        this.logger.error(`Page error: ${error.message}`);
        this.errors.push(`Page: ${error.message}`);
      });
      
      this.page.on('response', response => {
        if (response.status() >= 400) {
          this.logger.warning(`HTTP ${response.status()}: ${response.url()}`);
          if (response.status() >= 500) {
            this.errors.push(`HTTP ${response.status()}: ${response.url()}`);
          }
        }
      });
      
      // Set timeouts
      this.page.setDefaultTimeout(this.options.TIMEOUTS.element);
      this.page.setDefaultNavigationTimeout(this.options.TIMEOUTS.navigation);
      
      this.logger.success('Browser environment ready');
    } catch (error) {
      this.logger.error(`Setup failed: ${error.message}`);
      throw error;
    }
  }
  
  /**
   * Teardown - Clean up resources
   */
  async teardown() {
    try {
      this.logger.step('Tearing down test environment');
      
      // Log performance metrics
      this.performance.logMetrics();
      
      // Log summary
      this.logger.info(`📊 Test Summary: ${this.errors.length} errors, ${this.warnings.length} warnings`);
      
      if (this.errors.length > 0) {
        this.logger.error('Errors encountered:');
        this.errors.forEach(error => this.logger.error(`  - ${error}`));
      }
      
      if (this.browser) {
        await this.browser.close();
      }
      
      this.logger.success('Teardown complete');
    } catch (error) {
      this.logger.error(`Teardown failed: ${error.message}`);
    }
  }
  
  /**
   * Enhanced navigation with retry logic
   */
  async navigateTo(url, waitUntil = 'networkidle2', retries = 3) {
    const fullUrl = url.startsWith('http') ? url : `${this.options.BASE_URL}${url}`;
    
    for (let attempt = 1; attempt <= retries; attempt++) {
      try {
        this.logger.step(`Navigating to ${fullUrl} (attempt ${attempt}/${retries})`);
        
        const loadTime = await this.performance.measurePageLoad(
          this.page, 
          fullUrl, 
          `navigation-${url.replace(/[^a-zA-Z0-9]/g, '-')}`
        );
        
        this.logger.success(`Navigation completed in ${loadTime}ms`);
        await this.screenshots.capture(this.page, `nav-${url.replace(/[^a-zA-Z0-9]/g, '-')}`);
        return;
        
      } catch (error) {
        this.logger.warning(`Navigation attempt ${attempt} failed: ${error.message}`);
        if (attempt === retries) {
          throw new Error(`Navigation failed after ${retries} attempts: ${error.message}`);
        }
        await this.delay(1000 * attempt); // Progressive backoff
      }
    }
  }
  
  /**
   * Enhanced element interaction methods
   */
  async waitForSelector(selector, options = {}) {
    try {
      this.logger.debug(`Waiting for selector: ${selector}`);
      const element = await this.page.waitForSelector(selector, {
        timeout: this.options.TIMEOUTS.element,
        ...options
      });
      return element;
    } catch (error) {
      await this.screenshots.capture(this.page, `selector-timeout-${selector.replace(/[^a-zA-Z0-9]/g, '-')}`);
      throw new Error(`Selector not found: ${selector} - ${error.message}`);
    }
  }
  
  async clickElement(selector, options = {}) {
    try {
      this.logger.debug(`Clicking element: ${selector}`);
      await this.waitForSelector(selector);
      await this.page.click(selector, options);
      await this.delay(100); // Small delay for UI updates
    } catch (error) {
      await this.screenshots.capture(this.page, `click-failed-${selector.replace(/[^a-zA-Z0-9]/g, '-')}`);
      throw new Error(`Click failed on ${selector}: ${error.message}`);
    }
  }
  
  async typeText(selector, text, options = {}) {
    try {
      this.logger.debug(`Typing text in ${selector}: "${text}"`);
      await this.waitForSelector(selector);
      await this.page.focus(selector);
      await this.page.keyboard.down('Control');
      await this.page.keyboard.press('KeyA');
      await this.page.keyboard.up('Control');
      await this.page.type(selector, text, { delay: 50, ...options });
    } catch (error) {
      await this.screenshots.capture(this.page, `type-failed-${selector.replace(/[^a-zA-Z0-9]/g, '-')}`);
      throw new Error(`Type failed on ${selector}: ${error.message}`);
    }
  }
  
  /**
   * HTMX-specific helpers
   */
  async waitForHTMXRequest() {
    this.logger.debug('Waiting for HTMX request to complete');
    await this.page.evaluate(() => {
      return new Promise((resolve) => {
        if (window.htmx) {
          const checkHTMX = () => {
            if (document.body.classList.contains('htmx-request')) {
              setTimeout(checkHTMX, 100);
            } else {
              resolve();
            }
          };
          checkHTMX();
        } else {
          resolve();
        }
      });
    });
  }
  
  /**
   * Authentication helpers
   */
  async isLoggedIn() {
    try {
      // Check for common authentication indicators
      const indicators = [
        'a[href="/logout"]',
        '[data-testid="user-menu"]',
        '.user-profile',
        'button:contains("Logout")'
      ];
      
      for (const indicator of indicators) {
        if (await this.page.$(indicator)) {
          return true;
        }
      }
      
      // Check for authentication in localStorage/sessionStorage
      const hasAuth = await this.page.evaluate(() => {
        return !!(localStorage.getItem('auth_token') || 
                 sessionStorage.getItem('auth_token') ||
                 document.cookie.includes('auth_token'));
      });
      
      return hasAuth;
    } catch (error) {
      this.logger.debug(`Auth check failed: ${error.message}`);
      return false;
    }
  }
  
  /**
   * Utility methods
   */
  async delay(ms) {
    await new Promise(resolve => setTimeout(resolve, ms));
  }
  
  async setViewport(viewport) {
    if (typeof viewport === 'string') {
      viewport = this.options.VIEWPORTS[viewport];
    }
    await this.page.setViewport(viewport);
    this.logger.info(`Viewport set to ${viewport.width}x${viewport.height}`);
  }
  
  /**
   * Test execution wrapper
   */
  async run(testFunction) {
    try {
      await this.setup();
      await testFunction.call(this);
      this.logger.success(`✅ Test completed: ${this.testName}`);
    } catch (error) {
      this.logger.error(`❌ Test failed: ${this.testName} - ${error.message}`);
      await this.screenshots.capture(this.page, 'test-failure');
      throw error;
    } finally {
      await this.teardown();
    }
  }
}

module.exports = {
  BaseTest,
  TestConfig,
  TestLogger,
  ScreenshotManager,
  PerformanceMonitor,
  colors
};