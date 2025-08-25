/**
 * API Integration Test Suite for Alchemorsel v3 Docker Deployment
 * Tests API endpoints against live Docker deployment on port 3013
 * Validates all API functionality, response times, and data integrity
 */

const axios = require('axios');
const fs = require('fs');
const path = require('path');

// API Test Configuration
const API_CONFIG = {
    baseUrl: 'http://localhost:3013',
    timeout: 10000,
    performance: {
        responseThreshold: 100, // milliseconds
        healthCheckThreshold: 50,
        dataEndpointThreshold: 200
    }
};

class APIIntegrationTestSuite {
    constructor() {
        this.testResults = {
            passed: [],
            failed: [],
            warnings: [],
            performance: {
                endpoints: [],
                averageResponseTime: 0,
                slowestEndpoint: null,
                fastestEndpoint: null
            }
        };
        this.performanceMetrics = [];
    }

    async makeAPIRequest(method, endpoint, data = null, headers = {}) {
        const startTime = Date.now();
        const url = `${API_CONFIG.baseUrl}${endpoint}`;
        
        const config = {
            method,
            url,
            timeout: API_CONFIG.timeout,
            headers: {
                'Content-Type': 'application/json',
                'Accept': 'application/json',
                ...headers
            },
            validateStatus: () => true // Don't throw on any status code
        };
        
        if (data) {
            config.data = data;
        }
        
        try {
            const response = await axios(config);
            const responseTime = Date.now() - startTime;
            
            this.performanceMetrics.push({
                endpoint,
                method,
                responseTime,
                status: response.status,
                timestamp: new Date().toISOString()
            });
            
            return {
                success: true,
                status: response.status,
                data: response.data,
                headers: response.headers,
                responseTime
            };
        } catch (error) {
            const responseTime = Date.now() - startTime;
            
            this.performanceMetrics.push({
                endpoint,
                method,
                responseTime,
                status: 'error',
                error: error.message,
                timestamp: new Date().toISOString()
            });
            
            return {
                success: false,
                error: error.message,
                responseTime
            };
        }
    }

    async testStep(name, testFn) {
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
            return false;
        }
    }

    async testHealthEndpoint() {
        await this.testStep('Health Endpoint - Structure and Response', async () => {
            const result = await this.makeAPIRequest('GET', '/health');
            
            if (!result.success) {
                throw new Error(`Health endpoint failed: ${result.error}`);
            }
            
            if (result.status !== 200) {
                throw new Error(`Health endpoint returned status ${result.status}`);
            }
            
            const health = result.data;
            const requiredFields = ['status', 'version', 'timestamp'];
            
            for (const field of requiredFields) {
                if (!(field in health)) {
                    throw new Error(`Health response missing required field: ${field}`);
                }
            }
            
            if (health.status !== 'healthy') {
                throw new Error(`Service status is '${health.status}', expected 'healthy'`);
            }
            
            console.log(`   ✓ Service Status: ${health.status}`);
            console.log(`   ✓ Version: ${health.version}`);
            console.log(`   ✓ Response Time: ${result.responseTime}ms`);
            
            if (result.responseTime > API_CONFIG.performance.healthCheckThreshold) {
                this.testResults.warnings.push(`Health endpoint slow: ${result.responseTime}ms`);
            }
        });
    }

    async testRecipesEndpoints() {
        await this.testStep('Recipes Endpoint - Get All Recipes', async () => {
            const result = await this.makeAPIRequest('GET', '/api/v1/recipes');
            
            if (!result.success) {
                throw new Error(`Recipes endpoint failed: ${result.error}`);
            }
            
            if (result.status !== 200) {
                throw new Error(`Recipes endpoint returned status ${result.status}`);
            }
            
            const recipes = result.data;
            const requiredFields = ['success', 'data', 'message'];
            
            for (const field of requiredFields) {
                if (!(field in recipes)) {
                    throw new Error(`Recipes response missing required field: ${field}`);
                }
            }
            
            console.log(`   ✓ Success: ${recipes.success}`);
            console.log(`   ✓ Message: ${recipes.message}`);
            console.log(`   ✓ Response Time: ${result.responseTime}ms`);
        });
    }

    async testAPIPerformance() {
        await this.testStep('API Performance Analysis', async () => {
            if (this.performanceMetrics.length === 0) {
                throw new Error('No performance metrics collected');
            }
            
            const successfulRequests = this.performanceMetrics.filter(m => 
                typeof m.status === 'number' && m.status < 400
            );
            
            if (successfulRequests.length === 0) {
                throw new Error('No successful requests for performance analysis');
            }
            
            const responseTimes = successfulRequests.map(m => m.responseTime);
            const averageResponseTime = responseTimes.reduce((a, b) => a + b, 0) / responseTimes.length;
            
            console.log(`   ✓ Total Requests: ${this.performanceMetrics.length}`);
            console.log(`   ✓ Successful Requests: ${successfulRequests.length}`);
            console.log(`   ✓ Average Response Time: ${averageResponseTime.toFixed(2)}ms`);
            
            this.testResults.performance.averageResponseTime = parseFloat(averageResponseTime.toFixed(2));
        });
    }

    async generateAPITestReport() {
        console.log('\n' + '='.repeat(80));
        console.log('📊 API INTEGRATION TEST RESULTS');
        console.log('='.repeat(80));
        
        const totalTests = this.testResults.passed.length + this.testResults.failed.length;
        const successRate = totalTests > 0 ? ((this.testResults.passed.length / totalTests) * 100).toFixed(1) : '0';
        
        console.log(`\n✅ PASSED TESTS: ${this.testResults.passed.length}`);
        this.testResults.passed.forEach(test => {
            console.log(`   ✓ ${test.name} (${test.duration}ms)`);
        });
        
        console.log(`\n❌ FAILED TESTS: ${this.testResults.failed.length}`);
        this.testResults.failed.forEach(test => {
            console.log(`   ✗ ${test.test} (${test.duration}ms)`);
            console.log(`     Error: ${test.error}`);
        });
        
        console.log(`\n⚠️  WARNINGS: ${this.testResults.warnings.length}`);
        this.testResults.warnings.forEach(warning => {
            console.log(`   - ${warning}`);
        });
        
        console.log(`\n📈 PERFORMANCE SUMMARY:`);
        console.log(`   Average Response Time: ${this.testResults.performance.averageResponseTime}ms`);
        console.log(`   Total API Requests: ${this.performanceMetrics.length}`);
        
        console.log('\n' + '='.repeat(80));
        console.log(`API TEST SUMMARY:`);
        console.log(`Total Tests: ${totalTests}`);
        console.log(`Success Rate: ${successRate}%`);
        console.log(`API Base URL: ${API_CONFIG.baseUrl}`);
        console.log('='.repeat(80));
        
        const report = {
            timestamp: new Date().toISOString(),
            apiUrl: API_CONFIG.baseUrl,
            summary: {
                total: totalTests,
                passed: this.testResults.passed.length,
                failed: this.testResults.failed.length,
                warnings: this.testResults.warnings.length,
                successRate: successRate + '%'
            },
            performance: this.testResults.performance,
            allMetrics: this.performanceMetrics,
            details: this.testResults
        };
        
        const reportDir = './test/e2e/screenshots';
        if (!fs.existsSync(reportDir)) {
            fs.mkdirSync(reportDir, { recursive: true });
        }
        
        const reportPath = path.join(reportDir, 'api_integration_test_report.json');
        fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
        
        console.log(`\n📁 API test report saved to: ${reportPath}`);
        return report;
    }

    async run() {
        const testStartTime = Date.now();
        
        try {
            console.log('\n🧪 Starting API Integration Test Suite');
            console.log('Testing Alchemorsel v3 API - Live Docker Deployment');
            console.log(`API Base URL: ${API_CONFIG.baseUrl}`);
            console.log('='.repeat(80));
            
            await this.testHealthEndpoint();
            await this.testRecipesEndpoints();
            await this.testAPIPerformance();
            
            const report = await this.generateAPITestReport();
            
            const testDuration = Date.now() - testStartTime;
            console.log(`\nTest suite completed in ${(testDuration / 1000).toFixed(1)} seconds`);
            
            return this.testResults.failed.length === 0 ? 0 : 1;
            
        } catch (error) {
            console.error('💥 Critical API test failure:', error);
            this.testResults.failed.push({
                test: 'API Test Suite Execution',
                error: error.message,
                timestamp: new Date().toISOString()
            });
            
            await this.generateAPITestReport();
            return 1;
        }
    }
}

if (require.main === module) {
    (async () => {
        const testSuite = new APIIntegrationTestSuite();
        const exitCode = await testSuite.run();
        process.exit(exitCode);
    })();
}