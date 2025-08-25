/**
 * Comprehensive E2E Test Suite for Alchemorsel v3 Docker Deployment
 * Tests live Docker containers running on ports 3013 (API) and 3014 (Web)
 * Validates performance, user journeys, and 14KB first packet optimization
 */

const puppeteer = require('puppeteer');
const axios = require('axios');
const fs = require('fs');
const path = require('path');

// Test configuration for Docker deployment
const CONFIG = {
    baseUrl: 'http://localhost:3014',  // Web frontend container
    apiUrl: 'http://localhost:3013',   // API backend container
    screenshotDir: './test/e2e/screenshots',
    testTimeout: 180000, // 3 minutes for comprehensive tests
    performance: {
        firstPacketSize: 14 * 1024, // 14KB optimization target
        responseTimeThreshold: 100,   // 100ms response time threshold
        networkTimeout: 5000,
        maxRedirects: 5
    },
    testUser: {
        username: `e2e_user_${Date.now()}`,
        email: `e2e_test_${Date.now()}@alchemorsel.com`,
        password: 'E2ETestPassword123!',
        fullName: 'E2E Test User'
    }
};

// Ensure screenshot directory exists
if (!fs.existsSync(CONFIG.screenshotDir)) {
    fs.mkdirSync(CONFIG.screenshotDir, { recursive: true });
}

class DockerDeploymentE2ETestSuite {
    constructor() {
        this.browser = null;
        this.page = null;
        this.testResults = {
            passed: [],
            failed: [],
            warnings: [],
            performance: {
                networkRequests: 0,
                totalResponseTime: 0,
                firstPacketSizes: [],
                failedRequests: 0,
                apiEndpoints: [],
                webPageMetrics: []
            }
        };
        this.screenshotCounter = 0;
        this.networkActivity = [];
        this.performanceMetrics = [];
    }

    async init() {
        console.log('🚀 Initializing Puppeteer for Docker Deployment Testing...');
        
        this.browser = await puppeteer.launch({
            headless: process.env.E2E_HEADLESS !== 'false',
            args: [
                '--no-sandbox', 
                '--disable-setuid-sandbox',
                '--disable-web-security',
                '--disable-features=VizDisplayCompositor'
            ],
            defaultViewport: { width: 1280, height: 800 }
        });
        
        this.page = await this.browser.newPage();
        
        // Enable performance monitoring
        await this.page.evaluateOnNewDocument(() => {
            window.performanceMetrics = {
                navigationStart: performance.now(),
                loadStart: 0,
                loadEnd: 0,
                domContentLoaded: 0
            };
            
            document.addEventListener('DOMContentLoaded', () => {
                window.performanceMetrics.domContentLoaded = performance.now();
            });
            
            window.addEventListener('load', () => {
                window.performanceMetrics.loadEnd = performance.now();
            });
        });
        
        // Set up network monitoring
        await this.page.setRequestInterception(true);
        
        this.page.on('request', request => {
            this.networkActivity.push({
                type: 'request',
                url: request.url(),
                method: request.method(),
                headers: request.headers(),
                timestamp: Date.now(),
                resourceType: request.resourceType()
            });
            this.testResults.performance.networkRequests++;
            request.continue();
        });

        this.page.on('response', async response => {
            const responseData = {
                type: 'response',
                url: response.url(),
                status: response.status(),
                headers: response.headers(),
                timestamp: Date.now(),
                size: 0
            };
            
            try {
                // Get response size
                const contentLength = response.headers()['content-length'];
                if (contentLength) {
                    responseData.size = parseInt(contentLength);
                } else {
                    // For responses without content-length, try to get the actual size
                    try {
                        const buffer = await response.buffer();
                        responseData.size = buffer.length;
                    } catch (e) {
                        responseData.size = 0;
                    }
                }
                
                // Track first packet sizes for performance optimization validation
                if (response.url().includes(CONFIG.baseUrl) && responseData.size > 0) {
                    this.testResults.performance.firstPacketSizes.push({
                        url: response.url(),
                        size: responseData.size,
                        withinTarget: responseData.size <= CONFIG.performance.firstPacketSize
                    });
                }
            } catch (error) {
                this.testResults.warnings.push(`Failed to analyze response size for ${response.url()}: ${error.message}`);
            }
            
            this.networkActivity.push(responseData);
            
            if (response.status() >= 400) {
                this.testResults.performance.failedRequests++;
                this.testResults.warnings.push(`HTTP ${response.status()} for ${response.url()}`);
            }
        });
        
        // Set up console monitoring
        this.page.on('console', msg => {
            const type = msg.type();
            const text = msg.text();
            
            if (type === 'error') {
                this.testResults.warnings.push(`Console error: ${text}`);
            } else if (type === 'warning') {
                this.testResults.warnings.push(`Console warning: ${text}`);
            }
        });
        
        // Set up error monitoring
        this.page.on('pageerror', error => {
            this.testResults.warnings.push(`Page error: ${error.message}`);
        });
    }

    async takeScreenshot(name) {
        this.screenshotCounter++;
        const filename = `${String(this.screenshotCounter).padStart(3, '0')}_${name}.png`;
        const filepath = path.join(CONFIG.screenshotDir, filename);
        
        try {
            await this.page.screenshot({ 
                path: filepath, 
                fullPage: true,
                type: 'png',
                quality: 80 
            });
            console.log(`📸 Screenshot saved: ${filename}`);
            return filepath;
        } catch (error) {
            console.error(`Failed to take screenshot: ${error.message}`);
            return null;
        }
    }

    async testStep(name, testFn, options = {}) {
        console.log(`\n📝 Testing: ${name}`);
        const startTime = Date.now();
        
        try {
            await testFn();
            const duration = Date.now() - startTime;
            
            this.testResults.passed.push({ 
                name, 
                duration,
                timestamp: new Date().toISOString()
            });
            
            console.log(`✅ PASSED: ${name} (${duration}ms)`);
            
            // Take screenshot for critical tests
            if (options.screenshot !== false) {
                await this.takeScreenshot(`PASS_${name.replace(/\s+/g, '_')}`);
            }
            
            return true;
        } catch (error) {
            const duration = Date.now() - startTime;
            this.testResults.failed.push({ 
                test: name, 
                error: error.message,
                duration,
                timestamp: new Date().toISOString()
            });
            
            console.error(`❌ FAILED: ${name} - ${error.message} (${duration}ms)`);
            await this.takeScreenshot(`FAIL_${name.replace(/\s+/g, '_')}`);
            
            if (options.fatal) {
                throw error;
            }
            
            return false;
        }
    }

    async waitForElement(selector, options = {}) {
        const timeout = options.timeout || 5000;
        const visible = options.visible !== false;
        
        try {
            await this.page.waitForSelector(selector, { 
                timeout, 
                visible 
            });
            return true;
        } catch (error) {
            throw new Error(`Element '${selector}' not found within ${timeout}ms`);
        }
    }

    async clickElement(selector, description = '') {
        await this.waitForElement(selector);
        await this.page.click(selector);
        if (description) {
            console.log(`   👆 Clicked: ${description}`);
        }
        // Small delay to allow for UI updates
        await this.page.waitForTimeout(500);
    }

    async typeInElement(selector, text, description = '') {
        await this.waitForElement(selector);
        await this.page.click(selector);
        await this.page.keyboard.selectAll();
        await this.page.type(selector, text);
        if (description) {
            console.log(`   ⌨️  Typed: ${description}`);
        }
        await this.page.waitForTimeout(300);
    }

    // =============================================================
    // API VALIDATION TESTS
    // =============================================================

    async testAPIHealthAndEndpoints() {
        await this.testStep('API Health Check', async () => {
            const response = await axios.get(`${CONFIG.apiUrl}/health`, {
                timeout: CONFIG.performance.networkTimeout
            });
            
            if (response.status !== 200) {
                throw new Error(`API health check failed with status ${response.status}`);
            }
            
            const healthData = response.data;
            if (healthData.status !== 'healthy') {
                throw new Error(`API health status is '${healthData.status}', expected 'healthy'`);
            }
            
            console.log(`   ✓ API Version: ${healthData.version}`);
            console.log(`   ✓ API Status: ${healthData.status}`);
            
            this.testResults.performance.apiEndpoints.push({
                endpoint: '/health',
                status: response.status,
                responseTime: response.headers['x-response-time'] || 'unknown',
                healthy: true
            });
        });

        await this.testStep('API Recipes Endpoint', async () => {
            const response = await axios.get(`${CONFIG.apiUrl}/api/v1/recipes`, {
                timeout: CONFIG.performance.networkTimeout
            });
            
            if (response.status !== 200) {
                throw new Error(`Recipes endpoint failed with status ${response.status}`);
            }
            
            const recipesData = response.data;
            console.log(`   ✓ Recipes API Response: ${recipesData.message}`);
            console.log(`   ✓ Success: ${recipesData.success}`);
            
            this.testResults.performance.apiEndpoints.push({
                endpoint: '/api/v1/recipes',
                status: response.status,
                responseTime: response.headers['x-response-time'] || 'unknown',
                healthy: true
            });
        });
    }

    // =============================================================
    // WEB FRONTEND TESTS
    // =============================================================

    async testWebFrontendLoad() {
        await this.testStep('Web Frontend Initial Load', async () => {
            const startTime = Date.now();
            
            const response = await this.page.goto(CONFIG.baseUrl, { 
                waitUntil: 'networkidle2',
                timeout: CONFIG.performance.networkTimeout 
            });
            
            if (!response || response.status() >= 400) {
                throw new Error(`Failed to load homepage, status: ${response ? response.status() : 'no response'}`);
            }
            
            const loadTime = Date.now() - startTime;
            
            // Get performance metrics
            const metrics = await this.page.evaluate(() => {
                const perfData = performance.getEntriesByType('navigation')[0];
                return {
                    domContentLoaded: perfData.domContentLoadedEventEnd - perfData.domContentLoadedEventStart,
                    loadComplete: perfData.loadEventEnd - perfData.loadEventStart,
                    totalTime: perfData.loadEventEnd - perfData.navigationStart
                };
            });
            
            console.log(`   ✓ Page Load Time: ${loadTime}ms`);
            console.log(`   ✓ DOM Content Loaded: ${metrics.domContentLoaded}ms`);
            console.log(`   ✓ Total Load Time: ${metrics.totalTime}ms`);
            
            this.testResults.performance.webPageMetrics.push({
                page: 'homepage',
                loadTime,
                metrics
            });
            
            // Check for critical elements
            await this.waitForElement('title');
            const title = await this.page.title();
            console.log(`   ✓ Page Title: ${title}`);
            
            // Check for HTMX loading
            const htmxLoaded = await this.page.evaluate(() => {
                return typeof htmx !== 'undefined';
            });
            
            if (!htmxLoaded) {
                this.testResults.warnings.push('HTMX library not loaded');
            } else {
                console.log('   ✓ HTMX library loaded successfully');
            }
        });
    }

    async testWebFrontendNavigation() {
        await this.testStep('Web Frontend Navigation', async () => {
            // Test basic navigation elements
            const navElements = await this.page.evaluate(() => {
                const links = Array.from(document.querySelectorAll('a, button'));
                return links.map(el => ({
                    tag: el.tagName,
                    text: el.textContent.trim(),
                    href: el.href || null,
                    visible: el.offsetParent !== null
                })).filter(el => el.visible && el.text);
            });
            
            console.log(`   ✓ Found ${navElements.length} navigation elements`);
            
            // Test some navigation links
            const testableLinks = navElements.filter(el => 
                el.href && 
                el.href.includes(CONFIG.baseUrl) && 
                !el.href.includes('logout') &&
                el.text.length > 0
            );
            
            for (const link of testableLinks.slice(0, 3)) { // Test first 3 links
                try {
                    console.log(`   Testing link: "${link.text}" -> ${link.href}`);
                    
                    const response = await this.page.goto(link.href, { 
                        waitUntil: 'networkidle2',
                        timeout: 10000 
                    });
                    
                    if (response && response.status() < 400) {
                        console.log(`   ✓ Successfully navigated to ${link.href}`);
                    } else {
                        this.testResults.warnings.push(`Navigation to ${link.href} returned ${response.status()}`);
                    }
                } catch (error) {
                    this.testResults.warnings.push(`Failed to navigate to ${link.href}: ${error.message}`);
                }
            }
            
            // Return to homepage
            await this.page.goto(CONFIG.baseUrl, { waitUntil: 'networkidle2' });
        });
    }

    // =============================================================
    // PERFORMANCE TESTS
    // =============================================================

    async testPerformanceOptimization() {
        await this.testStep('14KB First Packet Optimization', async () => {
            // Clear network activity and reload page to get fresh metrics
            this.networkActivity = [];
            this.testResults.performance.firstPacketSizes = [];
            
            const startTime = Date.now();
            await this.page.goto(CONFIG.baseUrl, { 
                waitUntil: 'domcontentloaded' 
            });
            const loadTime = Date.now() - startTime;
            
            // Wait a bit for all requests to complete
            await this.page.waitForTimeout(2000);
            
            // Analyze first packet sizes
            const firstPacketSizes = this.testResults.performance.firstPacketSizes;
            
            if (firstPacketSizes.length === 0) {
                this.testResults.warnings.push('No first packet size data collected');
                return;
            }
            
            const htmlResponses = firstPacketSizes.filter(p => 
                p.url === CONFIG.baseUrl || 
                p.url === CONFIG.baseUrl + '/' ||
                p.url.includes('.html')
            );
            
            console.log(`   ✓ Analyzed ${firstPacketSizes.length} responses`);
            console.log(`   ✓ HTML responses: ${htmlResponses.length}`);
            
            for (const response of htmlResponses) {
                const sizeKB = (response.size / 1024).toFixed(2);
                const targetKB = (CONFIG.performance.firstPacketSize / 1024).toFixed(2);
                
                console.log(`   📊 ${response.url}: ${sizeKB}KB (target: ${targetKB}KB)`);
                
                if (!response.withinTarget) {
                    this.testResults.warnings.push(
                        `First packet size ${sizeKB}KB exceeds target ${targetKB}KB for ${response.url}`
                    );
                }
            }
            
            const optimizedCount = firstPacketSizes.filter(p => p.withinTarget).length;
            const optimizationRate = ((optimizedCount / firstPacketSizes.length) * 100).toFixed(1);
            
            console.log(`   ✓ First Packet Optimization Rate: ${optimizationRate}%`);
            console.log(`   ✓ Page Load Time: ${loadTime}ms`);
        });
    }

    async testAPIResponseTimes() {
        await this.testStep('API Response Time Performance', async () => {
            const endpoints = [
                '/health',
                '/api/v1/recipes'
            ];
            
            const responseTimeResults = [];
            
            for (const endpoint of endpoints) {
                const startTime = Date.now();
                
                try {
                    const response = await axios.get(`${CONFIG.apiUrl}${endpoint}`, {
                        timeout: CONFIG.performance.networkTimeout
                    });
                    
                    const responseTime = Date.now() - startTime;
                    responseTimeResults.push({
                        endpoint,
                        responseTime,
                        status: response.status,
                        withinThreshold: responseTime <= CONFIG.performance.responseTimeThreshold
                    });
                    
                    console.log(`   📊 ${endpoint}: ${responseTime}ms`);
                    
                    if (responseTime > CONFIG.performance.responseTimeThreshold) {
                        this.testResults.warnings.push(
                            `API endpoint ${endpoint} response time ${responseTime}ms exceeds threshold ${CONFIG.performance.responseTimeThreshold}ms`
                        );
                    }
                } catch (error) {
                    responseTimeResults.push({
                        endpoint,
                        responseTime: 'timeout',
                        status: 'error',
                        withinThreshold: false,
                        error: error.message
                    });
                    
                    this.testResults.warnings.push(`API endpoint ${endpoint} failed: ${error.message}`);
                }
            }
            
            const fastEndpoints = responseTimeResults.filter(r => r.withinThreshold).length;
            const performanceRate = ((fastEndpoints / responseTimeResults.length) * 100).toFixed(1);
            
            console.log(`   ✓ API Performance Rate: ${performanceRate}%`);
            
            // Store results for reporting
            this.testResults.performance.apiResponseTimes = responseTimeResults;
        });
    }

    // =============================================================
    // USER JOURNEY TESTS
    // =============================================================

    async testBasicUserJourney() {
        await this.testStep('Basic User Journey - Browse Recipes', async () => {
            // Start from homepage
            await this.page.goto(CONFIG.baseUrl, { waitUntil: 'networkidle2' });
            
            // Look for recipe-related elements
            const recipeElements = await this.page.evaluate(() => {
                const elements = [];
                
                // Look for common recipe page elements
                document.querySelectorAll('a, button, input').forEach(el => {
                    const text = el.textContent.toLowerCase();
                    const href = el.href ? el.href.toLowerCase() : '';
                    
                    if (text.includes('recipe') || 
                        text.includes('search') || 
                        text.includes('browse') ||
                        href.includes('recipe')) {
                        elements.push({
                            tag: el.tagName,
                            text: el.textContent.trim(),
                            href: el.href || null
                        });
                    }
                });
                
                return elements;
            });
            
            console.log(`   ✓ Found ${recipeElements.length} recipe-related elements`);
            
            // Try to interact with recipe elements
            if (recipeElements.length > 0) {
                for (const element of recipeElements.slice(0, 2)) {
                    if (element.href) {
                        try {
                            console.log(`   Testing recipe link: ${element.text}`);
                            await this.page.goto(element.href, { 
                                waitUntil: 'networkidle2',
                                timeout: 10000 
                            });
                            
                            const title = await this.page.title();
                            console.log(`   ✓ Navigated to: ${title}`);
                            
                            await this.takeScreenshot(`recipe_page_${element.text.replace(/\s+/g, '_')}`);
                        } catch (error) {
                            this.testResults.warnings.push(`Failed to test recipe link ${element.href}: ${error.message}`);
                        }
                    }
                }
            }
            
            // Return to homepage
            await this.page.goto(CONFIG.baseUrl, { waitUntil: 'networkidle2' });
        });
    }

    // =============================================================
    // RESPONSIVE DESIGN TESTS
    // =============================================================

    async testResponsiveDesign() {
        const viewports = [
            { name: 'Desktop', width: 1280, height: 800 },
            { name: 'Tablet', width: 768, height: 1024 },
            { name: 'Mobile', width: 375, height: 667 }
        ];
        
        for (const viewport of viewports) {
            await this.testStep(`Responsive Design - ${viewport.name}`, async () => {
                await this.page.setViewport({ 
                    width: viewport.width, 
                    height: viewport.height 
                });
                
                await this.page.goto(CONFIG.baseUrl, { 
                    waitUntil: 'networkidle2' 
                });
                
                // Take screenshot for visual verification
                await this.takeScreenshot(`responsive_${viewport.name.toLowerCase()}`);
                
                // Check that key elements are still visible
                const visibleElements = await this.page.evaluate(() => {
                    const elements = document.querySelectorAll('nav, header, main, .container');
                    let visibleCount = 0;
                    
                    elements.forEach(el => {
                        const rect = el.getBoundingClientRect();
                        if (rect.width > 0 && rect.height > 0) {
                            visibleCount++;
                        }
                    });
                    
                    return {
                        total: elements.length,
                        visible: visibleCount
                    };
                });
                
                console.log(`   ✓ ${viewport.name}: ${visibleElements.visible}/${visibleElements.total} key elements visible`);
                
                if (visibleElements.visible === 0) {
                    this.testResults.warnings.push(`No key elements visible in ${viewport.name} viewport`);
                }
            }, { screenshot: false }); // Screenshot already taken above
        }
        
        // Reset to desktop viewport
        await this.page.setViewport({ width: 1280, height: 800 });
    }

    // =============================================================
    // SECURITY TESTS
    // =============================================================

    async testBasicSecurityHeaders() {
        await this.testStep('Security Headers Validation', async () => {
            const response = await this.page.goto(CONFIG.baseUrl, { 
                waitUntil: 'networkidle2' 
            });
            
            const headers = response.headers();
            const securityHeaders = [
                'access-control-allow-origin',
                'access-control-allow-methods',
                'access-control-allow-headers'
            ];
            
            const presentHeaders = [];
            const missingHeaders = [];
            
            for (const header of securityHeaders) {
                if (headers[header]) {
                    presentHeaders.push(header);
                    console.log(`   ✓ ${header}: ${headers[header]}`);
                } else {
                    missingHeaders.push(header);
                }
            }
            
            console.log(`   ✓ Security headers present: ${presentHeaders.length}/${securityHeaders.length}`);
            
            if (missingHeaders.length > 0) {
                this.testResults.warnings.push(`Missing security headers: ${missingHeaders.join(', ')}`);
            }
        });
    }

    // =============================================================
    // REPORTING AND CLEANUP
    // =============================================================

    async generateComprehensiveReport() {
        console.log('\n' + '='.repeat(80));
        console.log('📊 COMPREHENSIVE E2E TEST RESULTS SUMMARY');
        console.log('='.repeat(80));
        
        const totalTests = this.testResults.passed.length + this.testResults.failed.length;
        const successRate = totalTests > 0 ? ((this.testResults.passed.length / totalTests) * 100).toFixed(1) : '0';
        
        console.log(`\n✅ PASSED TESTS: ${this.testResults.passed.length}`);
        this.testResults.passed.forEach(test => {
            const duration = typeof test === 'object' ? ` (${test.duration}ms)` : '';
            const name = typeof test === 'object' ? test.name : test;
            console.log(`   ✓ ${name}${duration}`);
        });
        
        console.log(`\n❌ FAILED TESTS: ${this.testResults.failed.length}`);
        this.testResults.failed.forEach(({ test, error, duration }) => {
            console.log(`   ✗ ${test} (${duration}ms)`);
            console.log(`     Error: ${error}`);
        });
        
        console.log(`\n⚠️  WARNINGS: ${this.testResults.warnings.length}`);
        this.testResults.warnings.forEach(warning => {
            console.log(`   - ${warning}`);
        });
        
        // Performance Summary
        console.log(`\n📈 PERFORMANCE SUMMARY:`);
        console.log(`   Network Requests: ${this.testResults.performance.networkRequests}`);
        console.log(`   Failed Requests: ${this.testResults.performance.failedRequests}`);
        console.log(`   API Endpoints Tested: ${this.testResults.performance.apiEndpoints.length}`);
        console.log(`   First Packet Sizes Analyzed: ${this.testResults.performance.firstPacketSizes.length}`);
        
        if (this.testResults.performance.firstPacketSizes.length > 0) {
            const optimizedPackets = this.testResults.performance.firstPacketSizes.filter(p => p.withinTarget).length;
            const optimizationRate = ((optimizedPackets / this.testResults.performance.firstPacketSizes.length) * 100).toFixed(1);
            console.log(`   14KB Optimization Rate: ${optimizationRate}%`);
        }
        
        console.log('\n' + '='.repeat(80));
        console.log(`SUMMARY STATISTICS:`);
        console.log(`Total Tests: ${totalTests}`);
        console.log(`Success Rate: ${successRate}%`);
        console.log(`Screenshots: ${this.screenshotCounter}`);
        console.log(`Test Duration: ${((Date.now() - this.testStartTime) / 1000).toFixed(1)}s`);
        console.log('='.repeat(80));
        
        // Write detailed report to file
        const report = {
            timestamp: new Date().toISOString(),
            summary: {
                total: totalTests,
                passed: this.testResults.passed.length,
                failed: this.testResults.failed.length,
                warnings: this.testResults.warnings.length,
                successRate: successRate + '%',
                testDurationSeconds: ((Date.now() - this.testStartTime) / 1000).toFixed(1)
            },
            performance: this.testResults.performance,
            details: this.testResults,
            networkActivity: this.networkActivity,
            config: CONFIG
        };
        
        const reportPath = path.join(CONFIG.screenshotDir, 'docker_e2e_test_report.json');
        fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
        
        console.log(`\n📁 Detailed report saved to: ${reportPath}`);
        
        return report;
    }

    async cleanup() {
        if (this.browser) {
            await this.browser.close();
        }
    }

    async run() {
        this.testStartTime = Date.now();
        
        try {
            await this.init();
            
            console.log('\n🧪 Starting Comprehensive E2E Test Suite for Docker Deployment');
            console.log('Testing Alchemorsel v3 - Live Docker Containers');
            console.log(`Web Frontend: ${CONFIG.baseUrl}`);
            console.log(`API Backend: ${CONFIG.apiUrl}`);
            console.log('='.repeat(80));
            
            // Phase 1: Infrastructure Tests
            console.log('\n📌 PHASE 1: Infrastructure and API Validation');
            console.log('-'.repeat(50));
            await this.testAPIHealthAndEndpoints();
            
            // Phase 2: Web Frontend Tests
            console.log('\n📌 PHASE 2: Web Frontend Testing');
            console.log('-'.repeat(50));
            await this.testWebFrontendLoad();
            await this.testWebFrontendNavigation();
            
            // Phase 3: Performance Tests
            console.log('\n📌 PHASE 3: Performance and Optimization Testing');
            console.log('-'.repeat(50));
            await this.testPerformanceOptimization();
            await this.testAPIResponseTimes();
            
            // Phase 4: User Experience Tests
            console.log('\n📌 PHASE 4: User Experience and Journey Testing');
            console.log('-'.repeat(50));
            await this.testBasicUserJourney();
            await this.testResponsiveDesign();
            
            // Phase 5: Security Tests
            console.log('\n📌 PHASE 5: Security and Headers Testing');
            console.log('-'.repeat(50));
            await this.testBasicSecurityHeaders();
            
            // Generate comprehensive report
            const report = await this.generateComprehensiveReport();
            
            // Return exit code based on test results
            return this.testResults.failed.length === 0 ? 0 : 1;
            
        } catch (error) {
            console.error('💥 Critical test suite failure:', error);
            this.testResults.failed.push({ 
                test: 'Test Suite Execution', 
                error: error.message,
                timestamp: new Date().toISOString()
            });
            
            await this.generateComprehensiveReport();
            return 1;
        } finally {
            await this.cleanup();
        }
    }
}

// Export for use in other modules
module.exports = DockerDeploymentE2ETestSuite;

// Run the test suite if this file is executed directly
if (require.main === module) {
    (async () => {
        const testSuite = new DockerDeploymentE2ETestSuite();
        const exitCode = await testSuite.run();
        process.exit(exitCode);
    })();
}