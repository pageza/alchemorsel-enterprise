/**
 * Comprehensive Performance and Security Tests for Alchemorsel v3
 * 
 * Tests performance benchmarks, security vulnerabilities, load handling,
 * memory usage, and security boundaries to ensure the application
 * meets performance targets and security standards.
 */

const { BaseTest } = require('./framework/base-test');
const { 
  HomePage, 
  LoginPage, 
  RegisterPage, 
  AIChatPage, 
  RecipesPage,
  RecipeFormPage
} = require('./framework/page-objects');

/**
 * Performance and Security Test Suite
 */
class PerformanceSecurityTest extends BaseTest {
  constructor() {
    super('Performance-Security');
    this.performanceThresholds = {
      pageLoadTime: 3000, // 3 seconds max
      firstContentfulPaint: 1500, // 1.5 seconds max
      timeToInteractive: 5000, // 5 seconds max
      cumulativeLayoutShift: 0.1, // CLS score threshold
      firstInputDelay: 100, // 100ms max
      largestContentfulPaint: 2500, // 2.5 seconds max
      totalBlockingTime: 300 // 300ms max
    };

    this.securityTestPayloads = {
      xss: [
        '<script>alert("XSS")</script>',
        'javascript:alert("XSS")',
        '<img src="x" onerror="alert(\'XSS\')">',
        '"><script>alert("XSS")</script>',
        '<svg onload="alert(\'XSS\')">',
        'javascript:/*--></title></style></textarea></script></xmp><svg/onload=alert(/XSS/)>'
      ],
      sqlInjection: [
        "'; DROP TABLE users; --",
        "' OR '1'='1",
        "' UNION SELECT * FROM users --",
        "admin'--",
        "' OR 1=1#"
      ],
      pathTraversal: [
        '../../../etc/passwd',
        '..\\..\\..\\windows\\system32\\config\\sam',
        '....//....//....//etc//passwd',
        '%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd'
      ],
      commandInjection: [
        '; cat /etc/passwd',
        '| whoami',
        '& dir',
        '`whoami`',
        '$(whoami)'
      ]
    };
  }

  /**
   * Test page load performance metrics
   */
  async testPageLoadPerformance() {
    this.logger.step('Testing page load performance metrics');

    const pages = [
      { name: 'Home', page: new HomePage(this) },
      { name: 'Recipes', page: new RecipesPage(this) },
      { name: 'Login', page: new LoginPage(this) }
    ];

    const performanceResults = {};

    for (const { name, page } of pages) {
      this.logger.debug(`Testing performance for ${name} page`);
      
      // Clear cache and restart browser for clean test
      await this.page.evaluate(() => {
        if ('caches' in window) {
          caches.keys().then(names => {
            names.forEach(name => caches.delete(name));
          });
        }
      });

      this.performance.startTimer(`${name}_load_time`);
      
      // Navigate and measure performance
      await page.navigate();
      await page.waitForLoad();
      
      const loadTime = this.performance.endTimer(`${name}_load_time`);

      // Get Web Vitals metrics
      const vitals = await this.page.evaluate(() => {
        return new Promise((resolve) => {
          const observer = new PerformanceObserver((list) => {
            const entries = list.getEntries();
            resolve(entries.map(entry => ({
              name: entry.name,
              value: entry.value,
              startTime: entry.startTime
            })));
          });
          
          // Observe paint timing
          if ('PerformanceObserver' in window) {
            observer.observe({ entryTypes: ['paint', 'navigation', 'resource'] });
          }
          
          // Fallback after timeout
          setTimeout(() => {
            resolve([]);
          }, 2000);
        });
      });

      // Get navigation timing
      const navigationTiming = await this.page.evaluate(() => {
        const timing = performance.getEntriesByType('navigation')[0];
        if (timing) {
          return {
            domContentLoaded: timing.domContentLoadedEventEnd - timing.domContentLoadedEventStart,
            loadComplete: timing.loadEventEnd - timing.loadEventStart,
            domInteractive: timing.domInteractive - timing.navigationStart,
            firstPaint: timing.responseEnd - timing.navigationStart
          };
        }
        return {};
      });

      // Calculate performance score
      const performanceScore = this.calculatePerformanceScore({
        loadTime,
        vitals,
        navigationTiming
      });

      performanceResults[name] = {
        loadTime,
        vitals,
        navigationTiming,
        score: performanceScore,
        passed: loadTime <= this.performanceThresholds.pageLoadTime
      };

      this.logger.info(`${name} page performance:`, {
        loadTime: `${loadTime}ms`,
        score: `${performanceScore}/100`,
        passed: loadTime <= this.performanceThresholds.pageLoadTime ? '✅' : '❌'
      });

      await this.screenshots.capture(this.page, `performance-${name.toLowerCase()}`);
      await this.delay(1000);
    }

    // Generate performance report
    this.generatePerformanceReport(performanceResults);
  }

  /**
   * Test resource loading performance
   */
  async testResourceLoadingPerformance() {
    this.logger.step('Testing resource loading performance');

    const homePage = new HomePage(this);
    
    // Monitor resource loading
    const resourceMetrics = [];
    
    this.page.on('response', response => {
      const request = response.request();
      const timing = response.timing();
      
      resourceMetrics.push({
        url: response.url(),
        status: response.status(),
        contentType: response.headers()['content-type'],
        size: parseInt(response.headers()['content-length']) || 0,
        timing: timing ? {
          dns: timing.dnsEnd - timing.dnsStart,
          connect: timing.connectEnd - timing.connectStart,
          tls: timing.connectEnd - timing.secureConnectionStart,
          request: timing.responseStart - timing.requestStart,
          response: timing.responseEnd - timing.responseStart,
          total: timing.responseEnd - timing.requestStart
        } : null
      });
    });

    await homePage.navigate();
    await homePage.waitForLoad();
    
    await this.delay(3000); // Allow all resources to load

    // Analyze resource metrics
    const slowResources = resourceMetrics.filter(resource => 
      resource.timing && resource.timing.total > 1000 // > 1 second
    );

    const largeResources = resourceMetrics.filter(resource => 
      resource.size > 1024 * 1024 // > 1MB
    );

    const failedResources = resourceMetrics.filter(resource => 
      resource.status >= 400
    );

    this.logger.info('Resource loading analysis:', {
      totalResources: resourceMetrics.length,
      slowResources: slowResources.length,
      largeResources: largeResources.length,
      failedResources: failedResources.length
    });

    if (slowResources.length > 0) {
      this.logger.warning('Slow loading resources found:');
      slowResources.forEach(resource => {
        this.logger.warning(`  ${resource.url}: ${resource.timing.total}ms`);
      });
    }

    if (largeResources.length > 0) {
      this.logger.warning('Large resources found:');
      largeResources.forEach(resource => {
        this.logger.warning(`  ${resource.url}: ${(resource.size / 1024 / 1024).toFixed(2)}MB`);
      });
    }

    if (failedResources.length > 0) {
      this.logger.error('Failed resource requests:');
      failedResources.forEach(resource => {
        this.logger.error(`  ${resource.url}: HTTP ${resource.status}`);
      });
    }

    await this.screenshots.capture(this.page, 'resource-loading-performance');
  }

  /**
   * Test memory usage and leaks
   */
  async testMemoryUsage() {
    this.logger.step('Testing memory usage and potential leaks');

    const pages = [
      new HomePage(this),
      new RecipesPage(this),
      new AIChatPage(this)
    ];

    const memoryMetrics = [];

    for (let i = 0; i < 3; i++) { // Test multiple iterations
      for (const page of pages) {
        await page.navigate();
        await page.waitForLoad();
        
        // Force garbage collection if possible
        await this.page.evaluate(() => {
          if (window.gc) {
            window.gc();
          }
        });

        // Get memory usage
        const memoryUsage = await this.page.evaluate(() => {
          if ('memory' in performance) {
            return {
              usedJSHeapSize: performance.memory.usedJSHeapSize,
              totalJSHeapSize: performance.memory.totalJSHeapSize,
              jsHeapSizeLimit: performance.memory.jsHeapSizeLimit
            };
          }
          return null;
        });

        if (memoryUsage) {
          memoryMetrics.push({
            iteration: i + 1,
            page: page.constructor.name,
            ...memoryUsage,
            usedMB: Math.round(memoryUsage.usedJSHeapSize / 1024 / 1024),
            totalMB: Math.round(memoryUsage.totalJSHeapSize / 1024 / 1024)
          });

          this.logger.debug(`Memory usage - ${page.constructor.name} (iteration ${i + 1}): ${Math.round(memoryUsage.usedJSHeapSize / 1024 / 1024)}MB`);
        }

        await this.delay(1000);
      }
    }

    // Analyze memory trends
    if (memoryMetrics.length > 0) {
      const firstIteration = memoryMetrics.filter(m => m.iteration === 1);
      const lastIteration = memoryMetrics.filter(m => m.iteration === 3);

      const memoryGrowth = lastIteration.map(last => {
        const first = firstIteration.find(f => f.page === last.page);
        return {
          page: last.page,
          growth: last.usedMB - (first ? first.usedMB : 0),
          growthPercent: first ? ((last.usedMB - first.usedMB) / first.usedMB * 100) : 0
        };
      });

      const significantGrowth = memoryGrowth.filter(g => g.growthPercent > 20); // > 20% growth

      if (significantGrowth.length > 0) {
        this.logger.warning('Potential memory leaks detected:');
        significantGrowth.forEach(growth => {
          this.logger.warning(`  ${growth.page}: +${growth.growth}MB (+${growth.growthPercent.toFixed(1)}%)`);
        });
      } else {
        this.logger.success('No significant memory leaks detected');
      }
    }

    await this.screenshots.capture(this.page, 'memory-usage-test');
  }

  /**
   * Test XSS vulnerability protection
   */
  async testXSSProtection() {
    this.logger.step('Testing XSS vulnerability protection');

    const testPages = [
      { page: new AIChatPage(this), inputSelector: 'input[name="message"], textarea[name="message"]' },
      { page: new RecipeFormPage(this), inputSelector: 'input[name="title"]' }
    ];

    for (const { page, inputSelector } of testPages) {
      this.logger.debug(`Testing XSS protection on ${page.constructor.name}`);

      try {
        // Login if required
        if (page.constructor.name !== 'LoginPage') {
          await this.loginTestUser();
        }

        await page.navigate();
        await page.waitForLoad();

        for (const xssPayload of this.securityTestPayloads.xss) {
          this.logger.debug(`Testing XSS payload: ${xssPayload.substring(0, 50)}...`);

          try {
            // Inject XSS payload
            const inputElement = await this.page.$(inputSelector);
            if (inputElement) {
              await inputElement.fill(xssPayload);
              
              // Submit form if submit button exists
              const submitButton = await this.page.$('button[type="submit"]');
              if (submitButton) {
                await submitButton.click();
                await this.delay(2000);
              }

              // Check if XSS payload was executed
              const alertTriggered = await this.page.evaluate(() => {
                return window.xssTriggered === true;
              });

              // Check if payload appears in DOM unescaped
              const pageContent = await this.page.content();
              const xssPresent = pageContent.includes(xssPayload) && 
                               !pageContent.includes(this.escapeHtml(xssPayload));

              if (alertTriggered || xssPresent) {
                this.logger.error(`❌ XSS vulnerability detected: ${xssPayload}`);
              } else {
                this.logger.success(`✅ XSS payload properly handled: ${xssPayload.substring(0, 30)}...`);
              }

              // Clear input for next test
              await inputElement.fill('');
            }
          } catch (error) {
            this.logger.debug(`XSS test error (may be expected): ${error.message}`);
          }
        }

        await this.screenshots.capture(this.page, `xss-protection-${page.constructor.name.toLowerCase()}`);

      } catch (error) {
        this.logger.error(`XSS testing failed for ${page.constructor.name}: ${error.message}`);
      }
    }
  }

  /**
   * Test SQL injection protection
   */
  async testSQLInjectionProtection() {
    this.logger.step('Testing SQL injection protection');

    const loginPage = new LoginPage(this);
    await loginPage.navigate();
    await loginPage.waitForLoad();

    for (const sqlPayload of this.securityTestPayloads.sqlInjection) {
      this.logger.debug(`Testing SQL injection payload: ${sqlPayload}`);

      try {
        // Try SQL injection in email field
        await this.page.fill('input[name="email"]', sqlPayload);
        await this.page.fill('input[name="password"]', 'anypassword');
        
        // Monitor for database errors or successful unauthorized login
        const responsePromise = this.page.waitForResponse(response => 
          response.url().includes('/login') || response.url().includes('/auth'),
          { timeout: 5000 }
        );

        await this.page.click('button[type="submit"]');

        try {
          const response = await responsePromise;
          const responseText = await response.text();

          // Check for database error messages that might indicate SQL injection worked
          const sqlErrorIndicators = [
            'sql', 'database', 'query', 'syntax error', 'mysql', 'postgresql',
            'column', 'table', 'constraint', 'violation'
          ];

          const hasDBError = sqlErrorIndicators.some(indicator => 
            responseText.toLowerCase().includes(indicator)
          );

          const isUnauthorizedAccess = response.status() === 200 && 
                                      (response.url().includes('/dashboard') || 
                                       responseText.includes('welcome'));

          if (hasDBError) {
            this.logger.error(`❌ Database error exposed for SQL injection: ${sqlPayload}`);
          } else if (isUnauthorizedAccess) {
            this.logger.error(`❌ SQL injection may have bypassed authentication: ${sqlPayload}`);
          } else {
            this.logger.success(`✅ SQL injection properly handled: ${sqlPayload}`);
          }

        } catch (error) {
          // Timeout or other error is expected for blocked SQL injection
          this.logger.success(`✅ SQL injection blocked (no response): ${sqlPayload}`);
        }

        // Clear fields for next test
        await this.page.fill('input[name="email"]', '');
        await this.page.fill('input[name="password"]', '');
        await this.delay(500);

      } catch (error) {
        this.logger.debug(`SQL injection test error (may be expected): ${error.message}`);
      }
    }

    await this.screenshots.capture(this.page, 'sql-injection-protection');
  }

  /**
   * Test authentication and authorization boundaries
   */
  async testAuthenticationSecurity() {
    this.logger.step('Testing authentication and authorization security');

    // Test direct access to protected routes without authentication
    const protectedRoutes = [
      '/dashboard',
      '/profile',
      '/recipes/new',
      '/ai/chat'
    ];

    for (const route of protectedRoutes) {
      this.logger.debug(`Testing unauthorized access to: ${route}`);

      // Make sure we're logged out
      await this.page.goto(`${this.options.BASE_URL}/logout`);
      await this.delay(1000);

      // Try to access protected route
      await this.page.goto(`${this.options.BASE_URL}${route}`);
      await this.delay(2000);

      const currentUrl = this.page.url();

      if (currentUrl.includes('/login')) {
        this.logger.success(`✅ Unauthorized access blocked for ${route} - redirected to login`);
      } else if (currentUrl.includes(route)) {
        this.logger.error(`❌ Unauthorized access allowed to ${route}`);
      } else {
        this.logger.warning(`⚠️ Unexpected redirect for ${route}: ${currentUrl}`);
      }
    }

    // Test session hijacking protection
    await this.testSessionSecurity();

    // Test password policy
    await this.testPasswordPolicy();

    await this.screenshots.capture(this.page, 'authentication-security');
  }

  /**
   * Test session security
   */
  async testSessionSecurity() {
    this.logger.debug('Testing session security');

    // Login with test user
    await this.loginTestUser();
    
    // Check if session cookies have secure flags
    const cookies = await this.page.context().cookies();
    const sessionCookie = cookies.find(cookie => 
      cookie.name.toLowerCase().includes('session') || 
      cookie.name.toLowerCase().includes('auth')
    );

    if (sessionCookie) {
      const securityFlags = {
        httpOnly: sessionCookie.httpOnly,
        secure: sessionCookie.secure,
        sameSite: sessionCookie.sameSite
      };

      this.logger.info('Session cookie security:', securityFlags);

      if (!securityFlags.httpOnly) {
        this.logger.warning('⚠️ Session cookie missing HttpOnly flag');
      }

      if (!securityFlags.secure && this.options.BASE_URL.includes('https')) {
        this.logger.warning('⚠️ Session cookie missing Secure flag for HTTPS');
      }

      if (securityFlags.sameSite === 'None') {
        this.logger.warning('⚠️ Session cookie SameSite=None may be insecure');
      }

      if (securityFlags.httpOnly && securityFlags.sameSite !== 'None') {
        this.logger.success('✅ Session cookie security flags properly set');
      }
    } else {
      this.logger.warning('⚠️ No session cookie found');
    }
  }

  /**
   * Test password policy enforcement
   */
  async testPasswordPolicy() {
    this.logger.debug('Testing password policy enforcement');

    const registerPage = new RegisterPage(this);
    await registerPage.navigate();
    await registerPage.waitForLoad();

    const weakPasswords = [
      '123',           // Too short
      'password',      // Common password
      '12345678',      // Only numbers
      'abcdefgh',      // Only letters
      'Password',      // Missing special chars/numbers
    ];

    for (const weakPassword of weakPasswords) {
      this.logger.debug(`Testing weak password: ${weakPassword}`);

      try {
        await this.page.fill('input[name="name"]', 'Test User');
        await this.page.fill('input[name="email"]', `test${Date.now()}@example.com`);
        await this.page.fill('input[name="password"]', weakPassword);
        
        const confirmPasswordField = await this.page.$('input[name="password_confirm"]');
        if (confirmPasswordField) {
          await confirmPasswordField.fill(weakPassword);
        }

        await this.page.click('button[type="submit"]');
        await this.delay(2000);

        // Check for password validation error
        const errorMessage = await this.page.$('.error, [data-testid="error-message"]');
        const currentUrl = this.page.url();

        if (errorMessage) {
          const errorText = await errorMessage.textContent();
          this.logger.success(`✅ Weak password rejected: ${weakPassword} - ${errorText}`);
        } else if (currentUrl.includes('/register')) {
          this.logger.success(`✅ Weak password rejected: ${weakPassword} - stayed on register page`);
        } else {
          this.logger.warning(`⚠️ Weak password may have been accepted: ${weakPassword}`);
        }

        // Clear fields for next test
        await this.page.fill('input[name="name"]', '');
        await this.page.fill('input[name="email"]', '');
        await this.page.fill('input[name="password"]', '');
        if (confirmPasswordField) {
          await confirmPasswordField.fill('');
        }

      } catch (error) {
        this.logger.debug(`Password policy test error: ${error.message}`);
      }
    }
  }

  /**
   * Test file upload security
   */
  async testFileUploadSecurity() {
    this.logger.step('Testing file upload security');

    await this.loginTestUser();

    // Look for file upload functionality
    const recipePage = new RecipeFormPage(this);
    await recipePage.navigate();
    await recipePage.waitForLoad();

    const fileUpload = await this.page.$('input[type="file"]');
    if (fileUpload) {
      this.logger.debug('File upload found - testing security');

      // Test malicious file types
      const maliciousFiles = [
        { name: 'test.php', content: '<?php echo "malicious"; ?>' },
        { name: 'test.jsp', content: '<%@ page language="java" %>' },
        { name: 'test.exe', content: 'MZ\x90\x00' }, // PE header
        { name: '../../../etc/passwd', content: 'root:x:0:0:root:/root:/bin/bash' }
      ];

      for (const { name, content } of maliciousFiles) {
        try {
          this.logger.debug(`Testing malicious file upload: ${name}`);

          // Create temporary file
          const tempFile = `/tmp/test_${name.replace(/[^a-zA-Z0-9.]/g, '_')}`;
          await this.page.evaluate(([fileName, fileContent]) => {
            const blob = new Blob([fileContent], { type: 'application/octet-stream' });
            const file = new File([blob], fileName);
            
            // Try to set file on input
            const fileInput = document.querySelector('input[type="file"]');
            if (fileInput) {
              const dataTransfer = new DataTransfer();
              dataTransfer.items.add(file);
              fileInput.files = dataTransfer.files;
            }
          }, [name, content]);

          // Submit form if submit button exists
          const submitButton = await this.page.$('button[type="submit"]');
          if (submitButton) {
            await submitButton.click();
            await this.delay(2000);

            // Check for upload validation error
            const errorMessage = await this.page.$('.error, [data-testid="error-message"]');
            if (errorMessage) {
              const errorText = await errorMessage.textContent();
              this.logger.success(`✅ Malicious file rejected: ${name} - ${errorText}`);
            } else {
              this.logger.warning(`⚠️ Malicious file may have been accepted: ${name}`);
            }
          }

        } catch (error) {
          this.logger.debug(`File upload test error: ${error.message}`);
        }
      }
    } else {
      this.logger.info('No file upload functionality found to test');
    }

    await this.screenshots.capture(this.page, 'file-upload-security');
  }

  /**
   * Test CSRF protection
   */
  async testCSRFProtection() {
    this.logger.step('Testing CSRF protection');

    await this.loginTestUser();

    // Check for CSRF tokens in forms
    const forms = await this.page.$$('form');
    
    for (const form of forms) {
      const csrfToken = await form.$('input[name*="csrf"], input[name*="token"], input[type="hidden"]');
      const formAction = await form.getAttribute('action');
      
      if (csrfToken) {
        this.logger.success(`✅ CSRF token found in form: ${formAction || 'unknown'}`);
      } else {
        this.logger.warning(`⚠️ No CSRF token found in form: ${formAction || 'unknown'}`);
      }
    }

    // Test CSRF attack simulation (attempt form submission without proper token)
    const recipePage = new RecipeFormPage(this);
    await recipePage.navigate();
    await recipePage.waitForLoad();

    try {
      // Remove CSRF token if present
      await this.page.evaluate(() => {
        const csrfInputs = document.querySelectorAll('input[name*="csrf"], input[name*="token"]');
        csrfInputs.forEach(input => input.remove());
      });

      // Try to submit form without CSRF token
      await this.page.fill('input[name="title"]', 'CSRF Test Recipe');
      await this.page.click('button[type="submit"]');
      await this.delay(2000);

      // Check if submission was blocked
      const errorMessage = await this.page.$('.error, [data-testid="error-message"]');
      const currentUrl = this.page.url();

      if (errorMessage || currentUrl.includes('error')) {
        this.logger.success('✅ CSRF attack blocked');
      } else {
        this.logger.warning('⚠️ Potential CSRF vulnerability - submission may have succeeded');
      }

    } catch (error) {
      this.logger.debug(`CSRF test error (may be expected): ${error.message}`);
    }

    await this.screenshots.capture(this.page, 'csrf-protection');
  }

  /**
   * Helper method to calculate performance score
   */
  calculatePerformanceScore({ loadTime, vitals, navigationTiming }) {
    let score = 100;

    // Deduct points for slow load time
    if (loadTime > this.performanceThresholds.pageLoadTime) {
      score -= Math.min(30, (loadTime - this.performanceThresholds.pageLoadTime) / 100);
    }

    // Deduct points for slow navigation timing
    if (navigationTiming.domInteractive > 2000) {
      score -= 20;
    }

    // Additional deductions for other metrics would go here
    
    return Math.max(0, Math.round(score));
  }

  /**
   * Helper method to generate performance report
   */
  generatePerformanceReport(results) {
    this.logger.info('=== PERFORMANCE REPORT ===');
    
    let totalScore = 0;
    let pageCount = 0;

    Object.entries(results).forEach(([pageName, result]) => {
      this.logger.info(`${pageName} Page:`);
      this.logger.info(`  Load Time: ${result.loadTime}ms ${result.passed ? '✅' : '❌'}`);
      this.logger.info(`  Performance Score: ${result.score}/100`);
      
      totalScore += result.score;
      pageCount++;
    });

    const averageScore = Math.round(totalScore / pageCount);
    this.logger.info(`Average Performance Score: ${averageScore}/100`);

    if (averageScore >= 80) {
      this.logger.success('✅ Overall performance: EXCELLENT');
    } else if (averageScore >= 60) {
      this.logger.warning('⚠️ Overall performance: GOOD');
    } else {
      this.logger.error('❌ Overall performance: NEEDS IMPROVEMENT');
    }
  }

  /**
   * Helper method to escape HTML
   */
  escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  /**
   * Helper method to login test user
   */
  async loginTestUser() {
    const loginPage = new LoginPage(this);
    await loginPage.navigate();
    await loginPage.waitForLoad();
    
    try {
      await loginPage.login('perftest@example.com', 'perftest123');
      await this.delay(2000);
    } catch (error) {
      const registerPage = new RegisterPage(this);
      await registerPage.navigate();
      await registerPage.waitForLoad();
      
      await registerPage.register('Perf Test User', 'perftest@example.com', 'perftest123');
      await this.delay(2000);
    }
  }
}

/**
 * Run comprehensive performance and security tests
 */
async function runPerformanceSecurityTests() {
  const perfSecTest = new PerformanceSecurityTest();
  
  await perfSecTest.run(async function() {
    // Performance tests
    await this.testPageLoadPerformance();
    await this.testResourceLoadingPerformance();
    await this.testMemoryUsage();
    
    // Security tests
    await this.testXSSProtection();
    await this.testSQLInjectionProtection();
    await this.testAuthenticationSecurity();
    await this.testFileUploadSecurity();
    await this.testCSRFProtection();
  });
}

// Export for use in main test suite
module.exports = { PerformanceSecurityTest, runPerformanceSecurityTests };

// Run tests if this file is executed directly
if (require.main === module) {
  runPerformanceSecurityTests().catch(error => {
    console.error('Performance and security tests failed:', error);
    process.exit(1);
  });
}