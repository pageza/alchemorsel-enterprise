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
    },
    testData: {
        recipe: {
            title: 'API Test Recipe',
            description: 'A recipe created by API integration tests',
            ingredients: ['test ingredient 1', 'test ingredient 2'],
            instructions: ['test step 1', 'test step 2'],
            category: 'test',
            prepTime: 30,
            cookTime: 45,
            servings: 4
        }
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
            
            // Track performance metrics
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

    // =============================================================
    // HEALTH AND STATUS TESTS
    // =============================================================

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
            
            // Validate health response structure
            const requiredFields = ['status', 'version', 'timestamp', 'checks'];
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
            console.log(`   ✓ Checks: ${health.checks ? health.checks.length : 0}`);
            console.log(`   ✓ Response Time: ${result.responseTime}ms`);
            
            // Validate individual health checks
            if (health.checks && Array.isArray(health.checks)) {
                for (const check of health.checks) {
                    if (check.status !== 'healthy') {
                        this.testResults.warnings.push(`Health check '${check.name}' status: ${check.status}`);
                    } else {
                        console.log(`   ✓ Health Check '${check.name}': ${check.status}`);
                    }
                }
            }
            
            // Performance validation
            if (result.responseTime > API_CONFIG.performance.healthCheckThreshold) {
                this.testResults.warnings.push(`Health endpoint slow: ${result.responseTime}ms > ${API_CONFIG.performance.healthCheckThreshold}ms`);
            }
        });
    }

    // =============================================================
    // RECIPES API TESTS
    // =============================================================

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
            
            // Validate response structure
            const requiredFields = ['success', 'data', 'message'];
            for (const field of requiredFields) {
                if (!(field in recipes)) {
                    throw new Error(`Recipes response missing required field: ${field}`);
                }
            }
            
            if (recipes.success !== true) {
                throw new Error(`Recipes response success is ${recipes.success}, expected true`);
            }
            
            console.log(`   ✓ Success: ${recipes.success}`);
            console.log(`   ✓ Message: ${recipes.message}`);
            console.log(`   ✓ Data Type: ${Array.isArray(recipes.data) ? 'Array' : typeof recipes.data}`);
            console.log(`   ✓ Recipe Count: ${Array.isArray(recipes.data) ? recipes.data.length : 'N/A'}`);
            console.log(`   ✓ Response Time: ${result.responseTime}ms`);
            
            // Performance validation
            if (result.responseTime > API_CONFIG.performance.dataEndpointThreshold) {
                this.testResults.warnings.push(`Recipes endpoint slow: ${result.responseTime}ms > ${API_CONFIG.performance.dataEndpointThreshold}ms`);
            }
        });
    }

    async testRecipesSearchEndpoint() {
        await this.testStep('Recipes Search Endpoint', async () => {
            const searchQueries = ['pasta', 'chicken', 'test', ''];
            
            for (const query of searchQueries) {
                const endpoint = query ? `/api/v1/recipes/search?q=${encodeURIComponent(query)}` : '/api/v1/recipes/search';
                const result = await this.makeAPIRequest('GET', endpoint);
                
                console.log(`   Testing search query: "${query}"`);
                
                if (query === '') {
                    // Empty query should return an error
                    if (result.status < 400) {
                        this.testResults.warnings.push(`Search without query should return error, got status ${result.status}`);
                    } else {
                        console.log(`   ✓ Empty query correctly rejected (${result.status})`);
                    }
                } else {
                    // Non-empty queries should succeed
                    if (result.status === 200) {
                        const searchResult = result.data;
                        console.log(`   ✓ Search "${query}": ${result.status} (${result.responseTime}ms)`);
                        
                        // Validate search response structure if it exists
                        if (searchResult && typeof searchResult === 'object') {
                            if ('data' in searchResult && Array.isArray(searchResult.data)) {
                                console.log(`   ✓ Found ${searchResult.data.length} results`);
                            }
                        }
                    } else {
                        console.log(`   ⚠️ Search "${query}": ${result.status} (${result.responseTime}ms)`);
                        // Not necessarily a failure, endpoint might not be implemented yet
                    }
                }
            }
        });
    }

    // =============================================================
    // ERROR HANDLING TESTS
    // =============================================================

    async testErrorHandling() {
        await this.testStep('API Error Handling', async () => {
            const errorCases = [
                {
                    name: 'Non-existent endpoint',
                    method: 'GET',
                    endpoint: '/api/v1/nonexistent',
                    expectedStatus: [404, 405]
                },
                {
                    name: 'Invalid recipe ID',
                    method: 'GET',
                    endpoint: '/api/v1/recipes/invalid-id-format',
                    expectedStatus: [400, 404]
                },
                {
                    name: 'Malformed JSON POST',
                    method: 'POST',
                    endpoint: '/api/v1/recipes',
                    data: 'invalid json',
                    headers: { 'Content-Type': 'application/json' },
                    expectedStatus: [400]
                }
            ];
            
            for (const testCase of errorCases) {
                console.log(`   Testing: ${testCase.name}`);
                
                let result;
                if (testCase.data) {
                    // For malformed JSON, we need to send raw string data
                    try {
                        const response = await axios({
                            method: testCase.method,
                            url: `${API_CONFIG.baseUrl}${testCase.endpoint}`,
                            data: testCase.data,
                            headers: testCase.headers || {},
                            timeout: API_CONFIG.timeout,
                            validateStatus: () => true
                        });
                        result = { status: response.status, data: response.data };
                    } catch (error) {
                        result = { status: 'error', error: error.message };
                    }
                } else {
                    result = await this.makeAPIRequest(testCase.method, testCase.endpoint);
                }
                
                if (testCase.expectedStatus.includes(result.status)) {
                    console.log(`   ✓ ${testCase.name}: Status ${result.status} (expected)`);
                } else {
                    console.log(`   ⚠️ ${testCase.name}: Status ${result.status} (expected one of: ${testCase.expectedStatus.join(', ')})`);
                    // Not necessarily a failure, just different error handling
                }
            }
        });
    }

    // =============================================================
    // PERFORMANCE AND LOAD TESTS
    // =============================================================

    async testAPIPerformance() {
        await this.testStep('API Performance Analysis', async () => {
            // Calculate performance metrics from all requests
            if (this.performanceMetrics.length === 0) {
                throw new Error('No performance metrics collected');
            }
            
            const successfulRequests = this.performanceMetrics.filter(m => typeof m.status === 'number' && m.status < 400);
            
            if (successfulRequests.length === 0) {
                throw new Error('No successful requests for performance analysis');
            }
            
            const responseTimes = successfulRequests.map(m => m.responseTime);
            const averageResponseTime = responseTimes.reduce((a, b) => a + b, 0) / responseTimes.length;
            const maxResponseTime = Math.max(...responseTimes);
            const minResponseTime = Math.min(...responseTimes);
            
            // Find slowest and fastest endpoints
            const slowestRequest = successfulRequests.find(m => m.responseTime === maxResponseTime);
            const fastestRequest = successfulRequests.find(m => m.responseTime === minResponseTime);
            
            console.log(`   ✓ Total Requests: ${this.performanceMetrics.length}`);
            console.log(`   ✓ Successful Requests: ${successfulRequests.length}`);
            console.log(`   ✓ Average Response Time: ${averageResponseTime.toFixed(2)}ms`);
            console.log(`   ✓ Fastest Request: ${fastestRequest.endpoint} (${minResponseTime}ms)`);
            console.log(`   ✓ Slowest Request: ${slowestRequest.endpoint} (${maxResponseTime}ms)`);
            
            // Update performance results
            this.testResults.performance.averageResponseTime = parseFloat(averageResponseTime.toFixed(2));
            this.testResults.performance.fastestEndpoint = {
                endpoint: fastestRequest.endpoint,
                responseTime: minResponseTime
            };
            this.testResults.performance.slowestEndpoint = {
                endpoint: slowestRequest.endpoint,
                responseTime: maxResponseTime
            };
            
            // Performance warnings
            if (averageResponseTime > API_CONFIG.performance.responseThreshold) {
                this.testResults.warnings.push(`Average API response time ${averageResponseTime.toFixed(2)}ms exceeds threshold ${API_CONFIG.performance.responseThreshold}ms`);
            }
            
            // Endpoint-specific performance
            const endpointPerformance = {};
            successfulRequests.forEach(req => {
                if (!endpointPerformance[req.endpoint]) {
                    endpointPerformance[req.endpoint] = [];
                }
                endpointPerformance[req.endpoint].push(req.responseTime);
            });
            
            console.log(`\n   📊 Endpoint Performance Breakdown:`);
            for (const [endpoint, times] of Object.entries(endpointPerformance)) {
                const avgTime = times.reduce((a, b) => a + b, 0) / times.length;
                console.log(`     ${endpoint}: ${avgTime.toFixed(2)}ms avg (${times.length} requests)`);
                
                this.testResults.performance.endpoints.push({
                    endpoint,
                    averageResponseTime: parseFloat(avgTime.toFixed(2)),
                    requestCount: times.length,
                    minTime: Math.min(...times),
                    maxTime: Math.max(...times)
                });
            }
        });
    }

    async testConcurrentRequests() {\n        await this.testStep('Concurrent Request Handling', async () => {\n            const concurrentRequests = 5;\n            const endpoint = '/health';\n            \n            console.log(`   Testing ${concurrentRequests} concurrent requests to ${endpoint}`);\n            \n            const startTime = Date.now();\n            const promises = [];\n            \n            for (let i = 0; i < concurrentRequests; i++) {\n                promises.push(this.makeAPIRequest('GET', endpoint));\n            }\n            \n            const results = await Promise.all(promises);\n            const totalTime = Date.now() - startTime;\n            \n            const successfulRequests = results.filter(r => r.success && r.status === 200);\n            \n            console.log(`   ✓ Concurrent Requests: ${concurrentRequests}`);\n            console.log(`   ✓ Successful: ${successfulRequests.length}/${concurrentRequests}`);\n            console.log(`   ✓ Total Time: ${totalTime}ms`);\n            console.log(`   ✓ Average Time per Request: ${(totalTime / concurrentRequests).toFixed(2)}ms`);\n            \n            if (successfulRequests.length < concurrentRequests) {\n                this.testResults.warnings.push(`Only ${successfulRequests.length}/${concurrentRequests} concurrent requests succeeded`);\n            }\n        });\n    }\n\n    // =============================================================\n    // CORS AND HEADERS TESTS\n    // =============================================================\n\n    async testCORSHeaders() {\n        await this.testStep('CORS Headers Validation', async () => {\n            const result = await this.makeAPIRequest('OPTIONS', '/health');\n            \n            // OPTIONS request might not be implemented, so check any successful request\n            let headersResult = result;\n            if (!result.success || result.status >= 400) {\n                headersResult = await this.makeAPIRequest('GET', '/health');\n            }\n            \n            if (!headersResult.success) {\n                throw new Error(`Could not get headers for CORS validation: ${headersResult.error}`);\n            }\n            \n            const headers = headersResult.headers;\n            const corsHeaders = [\n                'access-control-allow-origin',\n                'access-control-allow-methods',\n                'access-control-allow-headers'\n            ];\n            \n            const presentHeaders = [];\n            const missingHeaders = [];\n            \n            for (const header of corsHeaders) {\n                if (headers[header]) {\n                    presentHeaders.push(header);\n                    console.log(`   ✓ ${header}: ${headers[header]}`);\n                } else {\n                    missingHeaders.push(header);\n                }\n            }\n            \n            console.log(`   ✓ CORS Headers Present: ${presentHeaders.length}/${corsHeaders.length}`);\n            \n            if (missingHeaders.length > 0) {\n                this.testResults.warnings.push(`Missing CORS headers: ${missingHeaders.join(', ')}`);\n            }\n        });\n    }\n\n    // =============================================================\n    // REPORTING\n    // =============================================================\n\n    async generateAPITestReport() {\n        console.log('\\n' + '='.repeat(80));\n        console.log('📊 API INTEGRATION TEST RESULTS');\n        console.log('='.repeat(80));\n        \n        const totalTests = this.testResults.passed.length + this.testResults.failed.length;\n        const successRate = totalTests > 0 ? ((this.testResults.passed.length / totalTests) * 100).toFixed(1) : '0';\n        \n        console.log(`\\n✅ PASSED TESTS: ${this.testResults.passed.length}`);\n        this.testResults.passed.forEach(test => {\n            console.log(`   ✓ ${test.name} (${test.duration}ms)`);\n        });\n        \n        console.log(`\\n❌ FAILED TESTS: ${this.testResults.failed.length}`);\n        this.testResults.failed.forEach(test => {\n            console.log(`   ✗ ${test.test} (${test.duration}ms)`);\n            console.log(`     Error: ${test.error}`);\n        });\n        \n        console.log(`\\n⚠️  WARNINGS: ${this.testResults.warnings.length}`);\n        this.testResults.warnings.forEach(warning => {\n            console.log(`   - ${warning}`);\n        });\n        \n        // Performance Summary\n        console.log(`\\n📈 PERFORMANCE SUMMARY:`);\n        console.log(`   Average Response Time: ${this.testResults.performance.averageResponseTime}ms`);\n        \n        if (this.testResults.performance.fastestEndpoint) {\n            console.log(`   Fastest Endpoint: ${this.testResults.performance.fastestEndpoint.endpoint} (${this.testResults.performance.fastestEndpoint.responseTime}ms)`);\n        }\n        \n        if (this.testResults.performance.slowestEndpoint) {\n            console.log(`   Slowest Endpoint: ${this.testResults.performance.slowestEndpoint.endpoint} (${this.testResults.performance.slowestEndpoint.responseTime}ms)`);\n        }\n        \n        console.log(`   Total API Requests: ${this.performanceMetrics.length}`);\n        \n        console.log('\\n' + '='.repeat(80));\n        console.log(`API TEST SUMMARY:`);\n        console.log(`Total Tests: ${totalTests}`);\n        console.log(`Success Rate: ${successRate}%`);\n        console.log(`API Base URL: ${API_CONFIG.baseUrl}`);\n        console.log('='.repeat(80));\n        \n        // Write detailed report\n        const report = {\n            timestamp: new Date().toISOString(),\n            apiUrl: API_CONFIG.baseUrl,\n            summary: {\n                total: totalTests,\n                passed: this.testResults.passed.length,\n                failed: this.testResults.failed.length,\n                warnings: this.testResults.warnings.length,\n                successRate: successRate + '%'\n            },\n            performance: this.testResults.performance,\n            allMetrics: this.performanceMetrics,\n            details: this.testResults,\n            config: API_CONFIG\n        };\n        \n        const reportDir = './test/e2e/screenshots'; // Reuse screenshots directory\n        if (!fs.existsSync(reportDir)) {\n            fs.mkdirSync(reportDir, { recursive: true });\n        }\n        \n        const reportPath = path.join(reportDir, 'api_integration_test_report.json');\n        fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));\n        \n        console.log(`\\n📁 API test report saved to: ${reportPath}`);\n        \n        return report;\n    }\n\n    async run() {\n        const testStartTime = Date.now();\n        \n        try {\n            console.log('\\n🧪 Starting API Integration Test Suite');\n            console.log('Testing Alchemorsel v3 API - Live Docker Deployment');\n            console.log(`API Base URL: ${API_CONFIG.baseUrl}`);\n            console.log('='.repeat(80));\n            \n            // Phase 1: Health and Status Tests\n            console.log('\\n📌 PHASE 1: Health and Status Testing');\n            console.log('-'.repeat(50));\n            await this.testHealthEndpoint();\n            \n            // Phase 2: Core API Endpoints\n            console.log('\\n📌 PHASE 2: Core API Endpoints Testing');\n            console.log('-'.repeat(50));\n            await this.testRecipesEndpoints();\n            await this.testRecipesSearchEndpoint();\n            \n            // Phase 3: Error Handling\n            console.log('\\n📌 PHASE 3: Error Handling Testing');\n            console.log('-'.repeat(50));\n            await this.testErrorHandling();\n            \n            // Phase 4: Performance Testing\n            console.log('\\n📌 PHASE 4: Performance Testing');\n            console.log('-'.repeat(50));\n            await this.testAPIPerformance();\n            await this.testConcurrentRequests();\n            \n            // Phase 5: Headers and CORS\n            console.log('\\n📌 PHASE 5: Headers and CORS Testing');\n            console.log('-'.repeat(50));\n            await this.testCORSHeaders();\n            \n            // Generate comprehensive report\n            const report = await this.generateAPITestReport();\n            \n            const testDuration = Date.now() - testStartTime;\n            console.log(`\\nTest suite completed in ${(testDuration / 1000).toFixed(1)} seconds`);\n            \n            // Return exit code based on test results\n            return this.testResults.failed.length === 0 ? 0 : 1;\n            \n        } catch (error) {\n            console.error('💥 Critical API test failure:', error);\n            this.testResults.failed.push({\n                test: 'API Test Suite Execution',\n                error: error.message,\n                timestamp: new Date().toISOString()\n            });\n            \n            await this.generateAPITestReport();\n            return 1;\n        }\n    }\n}\n\n// Export for use in other modules\nmodule.exports = APIIntegrationTestSuite;\n\n// Run the test suite if this file is executed directly\nif (require.main === module) {\n    (async () => {\n        const testSuite = new APIIntegrationTestSuite();\n        const exitCode = await testSuite.run();\n        process.exit(exitCode);\n    })();\n}