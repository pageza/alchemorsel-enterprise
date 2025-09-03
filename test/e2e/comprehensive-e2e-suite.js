/**
 * Comprehensive E2E Test Suite for Alchemorsel v3
 * 
 * Master test runner that orchestrates all E2E test categories:
 * - User Journey Tests (complete workflows)
 * - Navigation Matrix Tests (all possible navigation paths)
 * - AI/LLM Integration Tests (AI functionality and context retention)
 * - HTMX Interaction Tests (dynamic functionality)
 * - Performance and Security Tests (benchmarks and vulnerability testing)
 * 
 * This is the comprehensive testing framework that covers "all the possible User Journeys" 
 * and "every conceivable combination of links/navigation" as requested.
 */

const { BaseTest } = require('./framework/base-test');

// Import all test suites
const { runUserJourneyTests } = require('./user-journeys');
const { runNavigationMatrixTests } = require('./navigation-matrix');
const { runAILLMIntegrationTests } = require('./ai-llm-integration');
const { runHTMXInteractionTests } = require('./htmx-interactions');
const { runPerformanceSecurityTests } = require('./performance-security');

/**
 * Comprehensive E2E Test Suite Runner
 */
class ComprehensiveE2ETestSuite {
  constructor(options = {}) {
    this.options = {
      // Test suite selection (all enabled by default)
      runUserJourneys: options.runUserJourneys !== false,
      runNavigation: options.runNavigation !== false,
      runAIIntegration: options.runAIIntegration !== false,
      runHTMXInteractions: options.runHTMXInteractions !== false,
      runPerformanceSecurity: options.runPerformanceSecurity !== false,
      
      // Test execution options
      parallel: options.parallel || false,
      continueOnFailure: options.continueOnFailure !== false,
      screenshot: options.screenshot !== false,
      verbose: options.verbose || false,
      
      // Environment options
      baseUrl: options.baseUrl || process.env.BASE_URL || 'http://web:8080',
      timeout: options.timeout || 300000, // 5 minutes per test suite
      
      // Reporting options
      generateReport: options.generateReport !== false,
      reportFormat: options.reportFormat || 'json',
      
      ...options
    };

    this.results = {
      suites: {},
      summary: {
        total: 0,
        passed: 0,
        failed: 0,
        skipped: 0,
        duration: 0
      },
      startTime: null,
      endTime: null
    };

    this.logger = this.createLogger();
  }

  createLogger() {
    const colors = {
      reset: '\x1b[0m',
      green: '\x1b[32m',
      red: '\x1b[31m',
      yellow: '\x1b[33m',
      blue: '\x1b[34m',
      magenta: '\x1b[35m',
      cyan: '\x1b[36m',
      bold: '\x1b[1m'
    };

    return {
      info: (msg, data) => console.log(`${colors.cyan}[INFO]${colors.reset} ${msg}`, data || ''),
      success: (msg) => console.log(`${colors.green}[SUCCESS]${colors.reset} ${msg}`),
      error: (msg) => console.log(`${colors.red}[ERROR]${colors.reset} ${msg}`),
      warning: (msg) => console.log(`${colors.yellow}[WARNING]${colors.reset} ${msg}`),
      debug: (msg) => this.options.verbose && console.log(`${colors.blue}[DEBUG]${colors.reset} ${msg}`),
      suite: (msg) => console.log(`\n${colors.bold}${colors.magenta}=== ${msg} ===${colors.reset}\n`)
    };
  }

  /**
   * Run all comprehensive E2E tests
   */
  async runAllTests() {
    this.logger.suite('ALCHEMORSEL V3 - COMPREHENSIVE E2E TEST SUITE');
    this.logger.info('Starting comprehensive end-to-end testing...');
    this.logger.info('Test configuration:', {
      baseUrl: this.options.baseUrl,
      parallel: this.options.parallel,
      continueOnFailure: this.options.continueOnFailure,
      suites: this.getEnabledSuites()
    });

    this.results.startTime = Date.now();

    try {
      // Pre-flight checks
      await this.preflightChecks();

      // Run test suites
      if (this.options.parallel) {
        await this.runTestSuitesInParallel();
      } else {
        await this.runTestSuitesSequentially();
      }

      // Generate final report
      await this.generateFinalReport();

      this.logger.suite('COMPREHENSIVE E2E TEST SUITE COMPLETED');
      this.printSummary();

      return this.results;

    } catch (error) {
      this.logger.error(`Test suite execution failed: ${error.message}`);
      this.results.summary.failed++;
      throw error;
    } finally {
      this.results.endTime = Date.now();
      this.results.summary.duration = this.results.endTime - this.results.startTime;
    }
  }

  /**
   * Pre-flight checks to ensure environment is ready
   */
  async preflightChecks() {
    this.logger.info('Running pre-flight checks...');

    // Check if services are accessible
    const healthCheckUrl = `${this.options.baseUrl}/health`;
    
    try {
      const fetch = (await import('node-fetch')).default;
      const response = await fetch(healthCheckUrl, { 
        timeout: 10000,
        headers: { 'User-Agent': 'E2E-Test-Suite/1.0' }
      });
      
      if (response.ok) {
        this.logger.success('✅ Application health check passed');
      } else {
        throw new Error(`Health check failed: HTTP ${response.status}`);
      }
    } catch (error) {
      this.logger.error(`❌ Application not accessible: ${error.message}`);
      throw new Error(`Pre-flight check failed: ${error.message}`);
    }

    // Check API health if available
    try {
      const apiHealthUrl = `${this.options.baseUrl.replace('web:', 'api:')}/health`;
      const fetch = (await import('node-fetch')).default;
      const apiResponse = await fetch(apiHealthUrl, { timeout: 5000 });
      
      if (apiResponse.ok) {
        this.logger.success('✅ API health check passed');
      }
    } catch (error) {
      this.logger.warning('⚠️ API health check failed (may not be critical)');
    }

    this.logger.success('Pre-flight checks completed');
  }

  /**
   * Run test suites sequentially (safer, more stable)
   */
  async runTestSuitesSequentially() {
    this.logger.info('Running test suites sequentially...');

    const testSuites = [
      { 
        name: 'User Journey Tests', 
        runner: runUserJourneyTests, 
        enabled: this.options.runUserJourneys,
        description: 'Complete user workflows and business-critical paths'
      },
      { 
        name: 'Navigation Matrix Tests', 
        runner: runNavigationMatrixTests, 
        enabled: this.options.runNavigation,
        description: 'All possible navigation paths and authentication states'
      },
      { 
        name: 'AI/LLM Integration Tests', 
        runner: runAILLMIntegrationTests, 
        enabled: this.options.runAIIntegration,
        description: 'AI functionality, context retention, and LLM responses'
      },
      { 
        name: 'HTMX Interaction Tests', 
        runner: runHTMXInteractionTests, 
        enabled: this.options.runHTMXInteractions,
        description: 'Dynamic HTMX functionality and real-time interactions'
      },
      { 
        name: 'Performance & Security Tests', 
        runner: runPerformanceSecurityTests, 
        enabled: this.options.runPerformanceSecurity,
        description: 'Performance benchmarks and security vulnerability testing'
      }
    ];

    for (const suite of testSuites) {
      if (!suite.enabled) {
        this.logger.info(`⏭️ Skipping ${suite.name} (disabled)`);
        this.results.suites[suite.name] = { status: 'skipped', reason: 'disabled' };
        this.results.summary.skipped++;
        continue;
      }

      this.logger.suite(`${suite.name.toUpperCase()}`);
      this.logger.info(`Description: ${suite.description}`);
      
      const suiteStartTime = Date.now();
      
      try {
        await this.runTestSuiteWithTimeout(suite);
        
        const duration = Date.now() - suiteStartTime;
        this.results.suites[suite.name] = {
          status: 'passed',
          duration,
          description: suite.description
        };
        
        this.results.summary.passed++;
        this.logger.success(`✅ ${suite.name} completed successfully in ${duration}ms`);
        
      } catch (error) {
        const duration = Date.now() - suiteStartTime;
        this.results.suites[suite.name] = {
          status: 'failed',
          duration,
          error: error.message,
          description: suite.description
        };
        
        this.results.summary.failed++;
        this.logger.error(`❌ ${suite.name} failed: ${error.message}`);
        
        if (!this.options.continueOnFailure) {
          throw new Error(`Test suite failed: ${suite.name}`);
        }
      }
      
      this.results.summary.total++;
      
      // Brief pause between suites for cleanup
      await this.delay(2000);
    }
  }

  /**
   * Run test suites in parallel (faster but potentially less stable)
   */
  async runTestSuitesInParallel() {
    this.logger.info('Running test suites in parallel...');
    this.logger.warning('⚠️ Parallel execution may cause resource conflicts');

    const enabledSuites = [
      { name: 'User Journey Tests', runner: runUserJourneyTests, enabled: this.options.runUserJourneys },
      { name: 'Navigation Matrix Tests', runner: runNavigationMatrixTests, enabled: this.options.runNavigation },
      { name: 'AI/LLM Integration Tests', runner: runAILLMIntegrationTests, enabled: this.options.runAIIntegration },
      { name: 'HTMX Interaction Tests', runner: runHTMXInteractionTests, enabled: this.options.runHTMXInteractions },
      { name: 'Performance & Security Tests', runner: runPerformanceSecurityTests, enabled: this.options.runPerformanceSecurity }
    ].filter(suite => suite.enabled);

    const suitePromises = enabledSuites.map(async (suite) => {
      const suiteStartTime = Date.now();
      
      try {
        this.logger.info(`🚀 Starting ${suite.name}...`);
        await this.runTestSuiteWithTimeout(suite);
        
        const duration = Date.now() - suiteStartTime;
        this.results.suites[suite.name] = {
          status: 'passed',
          duration
        };
        
        this.logger.success(`✅ ${suite.name} completed`);
        return { name: suite.name, status: 'passed' };
        
      } catch (error) {
        const duration = Date.now() - suiteStartTime;
        this.results.suites[suite.name] = {
          status: 'failed',
          duration,
          error: error.message
        };
        
        this.logger.error(`❌ ${suite.name} failed: ${error.message}`);
        return { name: suite.name, status: 'failed', error: error.message };
      }
    });

    const results = await Promise.allSettled(suitePromises);
    
    // Process results
    results.forEach((result, index) => {
      this.results.summary.total++;
      
      if (result.status === 'fulfilled') {
        if (result.value.status === 'passed') {
          this.results.summary.passed++;
        } else {
          this.results.summary.failed++;
        }
      } else {
        this.results.summary.failed++;
        this.logger.error(`Suite execution failed: ${result.reason}`);
      }
    });

    if (this.results.summary.failed > 0 && !this.options.continueOnFailure) {
      throw new Error(`${this.results.summary.failed} test suites failed`);
    }
  }

  /**
   * Run a test suite with timeout protection
   */
  async runTestSuiteWithTimeout(suite) {
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        reject(new Error(`Test suite timeout after ${this.options.timeout}ms`));
      }, this.options.timeout);

      suite.runner()
        .then(() => {
          clearTimeout(timeout);
          resolve();
        })
        .catch((error) => {
          clearTimeout(timeout);
          reject(error);
        });
    });
  }

  /**
   * Generate comprehensive test report
   */
  async generateFinalReport() {
    if (!this.options.generateReport) {
      return;
    }

    this.logger.info('Generating comprehensive test report...');

    const report = {
      metadata: {
        testSuite: 'Alchemorsel v3 - Comprehensive E2E Tests',
        timestamp: new Date().toISOString(),
        environment: {
          baseUrl: this.options.baseUrl,
          userAgent: 'E2E-Test-Suite/1.0',
          nodeVersion: process.version,
          platform: process.platform
        },
        configuration: this.options
      },
      summary: this.results.summary,
      suites: this.results.suites,
      recommendations: this.generateRecommendations()
    };

    const reportPath = `/tmp/e2e-reports/comprehensive-report-${Date.now()}.${this.options.reportFormat}`;
    
    try {
      const fs = require('fs');
      const path = require('path');
      
      // Ensure directory exists
      const dir = path.dirname(reportPath);
      if (!fs.existsSync(dir)) {
        fs.mkdirSync(dir, { recursive: true });
      }

      if (this.options.reportFormat === 'json') {
        fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
      } else {
        // Generate HTML report
        const htmlReport = this.generateHTMLReport(report);
        fs.writeFileSync(reportPath.replace('.json', '.html'), htmlReport);
      }

      this.logger.success(`📊 Test report saved: ${reportPath}`);
      
    } catch (error) {
      this.logger.error(`Failed to generate report: ${error.message}`);
    }
  }

  /**
   * Generate recommendations based on test results
   */
  generateRecommendations() {
    const recommendations = [];

    if (this.results.summary.failed > 0) {
      recommendations.push({
        priority: 'HIGH',
        category: 'Test Failures',
        message: `${this.results.summary.failed} test suite(s) failed. Review and fix failing tests before deployment.`
      });
    }

    const duration = this.results.summary.duration;
    if (duration > 600000) { // > 10 minutes
      recommendations.push({
        priority: 'MEDIUM',
        category: 'Performance',
        message: `Total test execution time (${Math.round(duration / 1000)}s) is high. Consider optimizing test performance or running in parallel.`
      });
    }

    if (this.results.summary.skipped > 0) {
      recommendations.push({
        priority: 'LOW',
        category: 'Test Coverage',
        message: `${this.results.summary.skipped} test suite(s) were skipped. Consider enabling all test suites for comprehensive coverage.`
      });
    }

    // Add suite-specific recommendations
    Object.entries(this.results.suites).forEach(([suiteName, result]) => {
      if (result.status === 'failed' && result.error) {
        recommendations.push({
          priority: 'HIGH',
          category: suiteName,
          message: `${suiteName} failed with error: ${result.error}`
        });
      }
    });

    return recommendations;
  }

  /**
   * Generate HTML report
   */
  generateHTMLReport(report) {
    const passedCount = report.summary.passed;
    const failedCount = report.summary.failed;
    const totalCount = report.summary.total;
    const successRate = totalCount > 0 ? Math.round((passedCount / totalCount) * 100) : 0;

    return `
<!DOCTYPE html>
<html>
<head>
    <title>Alchemorsel v3 - E2E Test Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background-color: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; }
        .header { text-align: center; color: #333; border-bottom: 2px solid #ddd; padding-bottom: 20px; }
        .summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin: 20px 0; }
        .metric { background: #f8f9fa; padding: 20px; border-radius: 6px; text-align: center; }
        .metric.success { border-left: 4px solid #28a745; }
        .metric.danger { border-left: 4px solid #dc3545; }
        .metric.warning { border-left: 4px solid #ffc107; }
        .suites { margin: 20px 0; }
        .suite { margin: 10px 0; padding: 15px; border-radius: 6px; }
        .suite.passed { background: #d4edda; border: 1px solid #c3e6cb; }
        .suite.failed { background: #f8d7da; border: 1px solid #f5c6cb; }
        .suite.skipped { background: #fff3cd; border: 1px solid #ffeeba; }
        .recommendations { margin: 20px 0; }
        .recommendation { margin: 10px 0; padding: 10px; border-radius: 4px; }
        .recommendation.HIGH { background: #f8d7da; border-left: 4px solid #dc3545; }
        .recommendation.MEDIUM { background: #fff3cd; border-left: 4px solid #ffc107; }
        .recommendation.LOW { background: #d1ecf1; border-left: 4px solid #bee5eb; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🧪 Alchemorsel v3 - Comprehensive E2E Test Report</h1>
            <p>Generated: ${report.metadata.timestamp}</p>
            <p>Environment: ${report.metadata.environment.baseUrl}</p>
        </div>

        <div class="summary">
            <div class="metric success">
                <h2>${successRate}%</h2>
                <p>Success Rate</p>
            </div>
            <div class="metric ${failedCount > 0 ? 'danger' : 'success'}">
                <h2>${passedCount}/${totalCount}</h2>
                <p>Tests Passed</p>
            </div>
            <div class="metric">
                <h2>${Math.round(report.summary.duration / 1000)}s</h2>
                <p>Total Duration</p>
            </div>
        </div>

        <div class="suites">
            <h2>📋 Test Suite Results</h2>
            ${Object.entries(report.suites).map(([name, result]) => `
                <div class="suite ${result.status}">
                    <h3>${result.status === 'passed' ? '✅' : result.status === 'failed' ? '❌' : '⏭️'} ${name}</h3>
                    <p><strong>Status:</strong> ${result.status.toUpperCase()}</p>
                    <p><strong>Duration:</strong> ${result.duration ? Math.round(result.duration / 1000) : 0}s</p>
                    ${result.description ? `<p><strong>Description:</strong> ${result.description}</p>` : ''}
                    ${result.error ? `<p><strong>Error:</strong> ${result.error}</p>` : ''}
                </div>
            `).join('')}
        </div>

        ${report.recommendations.length > 0 ? `
            <div class="recommendations">
                <h2>💡 Recommendations</h2>
                ${report.recommendations.map(rec => `
                    <div class="recommendation ${rec.priority}">
                        <strong>[${rec.priority}] ${rec.category}:</strong> ${rec.message}
                    </div>
                `).join('')}
            </div>
        ` : ''}

        <div style="margin-top: 40px; text-align: center; color: #666; font-size: 14px;">
            <p>🤖 Generated with Claude Code - Alchemorsel v3 E2E Test Suite</p>
        </div>
    </div>
</body>
</html>`;
  }

  /**
   * Print final summary to console
   */
  printSummary() {
    const { passed, failed, skipped, total, duration } = this.results.summary;
    const successRate = total > 0 ? Math.round((passed / total) * 100) : 0;

    this.logger.info('\n' + '='.repeat(60));
    this.logger.info('📊 COMPREHENSIVE E2E TEST SUITE SUMMARY');
    this.logger.info('='.repeat(60));
    this.logger.info(`Total Test Suites: ${total}`);
    this.logger.info(`✅ Passed: ${passed}`);
    this.logger.info(`❌ Failed: ${failed}`);
    this.logger.info(`⏭️ Skipped: ${skipped}`);
    this.logger.info(`🎯 Success Rate: ${successRate}%`);
    this.logger.info(`⏱️ Total Duration: ${Math.round(duration / 1000)}s`);
    this.logger.info('='.repeat(60));

    if (failed === 0) {
      this.logger.success('🎉 ALL TESTS PASSED! Application is ready for deployment.');
    } else {
      this.logger.error(`💥 ${failed} test suite(s) failed. Review issues before deployment.`);
    }

    // Display suite breakdown
    this.logger.info('\n📋 Suite Breakdown:');
    Object.entries(this.results.suites).forEach(([name, result]) => {
      const status = result.status === 'passed' ? '✅' : 
                    result.status === 'failed' ? '❌' : '⏭️';
      const duration = result.duration ? `(${Math.round(result.duration / 1000)}s)` : '';
      this.logger.info(`  ${status} ${name} ${duration}`);
    });
  }

  /**
   * Get list of enabled test suites
   */
  getEnabledSuites() {
    return [
      this.options.runUserJourneys && 'User Journeys',
      this.options.runNavigation && 'Navigation Matrix',
      this.options.runAIIntegration && 'AI/LLM Integration',
      this.options.runHTMXInteractions && 'HTMX Interactions',
      this.options.runPerformanceSecurity && 'Performance & Security'
    ].filter(Boolean);
  }

  /**
   * Helper delay function
   */
  delay(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
  }
}

/**
 * CLI execution
 */
async function main() {
  const args = process.argv.slice(2);
  
  const options = {
    parallel: args.includes('--parallel'),
    continueOnFailure: !args.includes('--fail-fast'),
    verbose: args.includes('--verbose'),
    runUserJourneys: !args.includes('--skip-journeys'),
    runNavigation: !args.includes('--skip-navigation'),
    runAIIntegration: !args.includes('--skip-ai'),
    runHTMXInteractions: !args.includes('--skip-htmx'),
    runPerformanceSecurity: !args.includes('--skip-security'),
    baseUrl: process.env.BASE_URL || 'http://web:8080'
  };

  const suite = new ComprehensiveE2ETestSuite(options);
  
  try {
    const results = await suite.runAllTests();
    
    if (results.summary.failed > 0) {
      process.exit(1);
    } else {
      process.exit(0);
    }
  } catch (error) {
    console.error('❌ Comprehensive E2E test suite failed:', error.message);
    process.exit(1);
  }
}

// Export for use as module or run as CLI
module.exports = { ComprehensiveE2ETestSuite };

if (require.main === module) {
  main();
}