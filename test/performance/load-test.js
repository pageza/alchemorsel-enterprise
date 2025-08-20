// k6 Load Testing Script for Alchemorsel v3
// Comprehensive performance testing for API and web endpoints

import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';
import { SharedArray } from 'k6/data';

// Custom metrics
const errorRate = new Rate('error_rate');
const responseTime = new Trend('response_time');
const authFailures = new Counter('auth_failures');
const apiRequests = new Counter('api_requests');

// Test data
const testUsers = new SharedArray('test_users', function () {
  return [
    { username: 'testuser1@example.com', password: 'testpass123' },
    { username: 'testuser2@example.com', password: 'testpass123' },
    { username: 'testuser3@example.com', password: 'testpass123' },
  ];
});

// Test configuration
export const options = {
  scenarios: {
    // Smoke test - minimal load
    smoke_test: {
      executor: 'constant-vus',
      vus: 1,
      duration: '30s',
      tags: { test_type: 'smoke' },
    },
    
    // Load test - normal load
    load_test: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '2m', target: 10 },   // Ramp up
        { duration: '5m', target: 10 },   // Stay at 10 users
        { duration: '2m', target: 20 },   // Ramp up to 20 users
        { duration: '5m', target: 20 },   // Stay at 20 users
        { duration: '2m', target: 0 },    // Ramp down
      ],
      tags: { test_type: 'load' },
      env: { TEST_TYPE: 'load' },
    },
    
    // Stress test - beyond normal capacity
    stress_test: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '2m', target: 20 },   // Ramp up to normal load
        { duration: '5m', target: 20 },   // Stay at normal load
        { duration: '2m', target: 50 },   // Ramp up to stress level
        { duration: '5m', target: 50 },   // Stay at stress level
        { duration: '2m', target: 100 },  // Ramp up to breaking point
        { duration: '5m', target: 100 },  // Stay at breaking point
        { duration: '5m', target: 0 },    // Ramp down
      ],
      tags: { test_type: 'stress' },
      env: { TEST_TYPE: 'stress' },
    },
    
    // Spike test - sudden load increase
    spike_test: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '1m', target: 10 },   // Normal load
        { duration: '30s', target: 100 }, // Spike
        { duration: '3m', target: 100 },  // Maintain spike
        { duration: '30s', target: 10 },  // Return to normal
        { duration: '1m', target: 0 },    // Ramp down
      ],
      tags: { test_type: 'spike' },
      env: { TEST_TYPE: 'spike' },
    },
    
    // Volume test - large data sets
    volume_test: {
      executor: 'constant-vus',
      vus: 5,
      duration: '10m',
      tags: { test_type: 'volume' },
      env: { TEST_TYPE: 'volume' },
    },
  },
  
  thresholds: {
    // Error rate should be less than 1%
    'error_rate': ['rate<0.01'],
    
    // Response time thresholds
    'http_req_duration': [
      'p(50)<500',   // 50% of requests under 500ms
      'p(90)<1000',  // 90% of requests under 1s
      'p(95)<2000',  // 95% of requests under 2s
      'p(99)<5000',  // 99% of requests under 5s
    ],
    
    // API-specific thresholds
    'http_req_duration{endpoint:api}': ['p(95)<1500'],
    'http_req_duration{endpoint:auth}': ['p(95)<2000'],
    'http_req_duration{endpoint:ai}': ['p(95)<10000'],
    
    // Authentication failure rate
    'auth_failures': ['count<10'],
    
    // Minimum request rate
    'http_reqs': ['rate>10'],
  },
};

// Base URL from environment or default
const BASE_URL = __ENV.BASE_URL || 'http://localhost:3010';

// Authentication token storage
let authToken = '';

export function setup() {
  console.log(`Starting performance tests against ${BASE_URL}`);
  
  // Health check before starting tests
  const healthCheck = http.get(`${BASE_URL}/health`);
  if (healthCheck.status !== 200) {
    console.error('Health check failed:', healthCheck.status);
    return null;
  }
  
  console.log('Health check passed, starting tests...');
  return { baseUrl: BASE_URL };
}

export default function (data) {
  const testType = __ENV.TEST_TYPE || 'load';
  
  group('Authentication Flow', function () {
    testAuthentication();
  });
  
  group('API Endpoints', function () {
    testAPIEndpoints();
  });
  
  group('Static Assets', function () {
    testStaticAssets();
  });
  
  if (testType === 'volume') {
    group('Data Operations', function () {
      testDataOperations();
    });
  }
  
  if (testType === 'ai') {
    group('AI Endpoints', function () {
      testAIEndpoints();
    });
  }
  
  // Random sleep between 1-5 seconds
  sleep(Math.random() * 4 + 1);
}

function testAuthentication() {
  const user = testUsers[Math.floor(Math.random() * testUsers.length)];
  
  const loginPayload = {
    email: user.username,
    password: user.password,
  };
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
    tags: { endpoint: 'auth' },
  };
  
  const response = http.post(
    `${BASE_URL}/api/auth/login`,
    JSON.stringify(loginPayload),
    params
  );
  
  const success = check(response, {
    'login status is 200 or 201': (r) => [200, 201].includes(r.status),
    'login response has token': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.token !== undefined;
      } catch (e) {
        return false;
      }
    },
    'login response time < 2s': (r) => r.timings.duration < 2000,
  });
  
  if (success && response.status === 200) {
    try {
      const body = JSON.parse(response.body);
      authToken = body.token;
    } catch (e) {
      console.error('Failed to parse login response:', e);
      authFailures.add(1);
    }
  } else {
    authFailures.add(1);
  }
  
  errorRate.add(!success);
  responseTime.add(response.timings.duration);
  apiRequests.add(1);
}

function testAPIEndpoints() {
  const endpoints = [
    { path: '/api/health', method: 'GET', auth: false },
    { path: '/api/recipes', method: 'GET', auth: true },
    { path: '/api/users/profile', method: 'GET', auth: true },
    { path: '/api/ingredients', method: 'GET', auth: false },
  ];
  
  endpoints.forEach(endpoint => {
    const headers = {
      'Content-Type': 'application/json',
    };
    
    if (endpoint.auth && authToken) {
      headers['Authorization'] = `Bearer ${authToken}`;
    }
    
    const params = {
      headers: headers,
      tags: { endpoint: 'api' },
    };
    
    let response;
    if (endpoint.method === 'GET') {
      response = http.get(`${BASE_URL}${endpoint.path}`, params);
    } else if (endpoint.method === 'POST') {
      response = http.post(`${BASE_URL}${endpoint.path}`, '{}', params);
    }
    
    const success = check(response, {
      [`${endpoint.path} status is 200`]: (r) => r.status === 200,
      [`${endpoint.path} response time < 1s`]: (r) => r.timings.duration < 1000,
      [`${endpoint.path} response has content`]: (r) => r.body.length > 0,
    });
    
    errorRate.add(!success);
    responseTime.add(response.timings.duration);
    apiRequests.add(1);
  });
}

function testStaticAssets() {
  const staticFiles = [
    '/favicon.ico',
    '/robots.txt',
    '/',  // Main page
  ];
  
  staticFiles.forEach(file => {
    const response = http.get(`${BASE_URL}${file}`, {
      tags: { endpoint: 'static' },
    });
    
    const success = check(response, {
      [`${file} status is 200`]: (r) => r.status === 200,
      [`${file} response time < 500ms`]: (r) => r.timings.duration < 500,
    });
    
    errorRate.add(!success);
    responseTime.add(response.timings.duration);
  });
}

function testDataOperations() {
  if (!authToken) return;
  
  // Test creating a recipe
  const recipePayload = {
    title: `Test Recipe ${Date.now()}`,
    description: 'A test recipe for performance testing',
    ingredients: [
      { name: 'Test Ingredient', amount: '1 cup' }
    ],
    instructions: ['Mix everything together'],
    cookingTime: 30,
    difficulty: 'easy'
  };
  
  const createParams = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${authToken}`,
    },
    tags: { endpoint: 'api', operation: 'create' },
  };
  
  const createResponse = http.post(
    `${BASE_URL}/api/recipes`,
    JSON.stringify(recipePayload),
    createParams
  );
  
  const createSuccess = check(createResponse, {
    'recipe creation status is 201': (r) => r.status === 201,
    'recipe creation response time < 2s': (r) => r.timings.duration < 2000,
  });
  
  errorRate.add(!createSuccess);
  responseTime.add(createResponse.timings.duration);
  apiRequests.add(1);
  
  // Test updating the recipe (if creation was successful)
  if (createSuccess && createResponse.status === 201) {
    let recipeId;
    try {
      const body = JSON.parse(createResponse.body);
      recipeId = body.id;
    } catch (e) {
      console.error('Failed to parse recipe creation response:', e);
      return;
    }
    
    const updatePayload = {
      ...recipePayload,
      title: `Updated ${recipePayload.title}`,
    };
    
    const updateResponse = http.put(
      `${BASE_URL}/api/recipes/${recipeId}`,
      JSON.stringify(updatePayload),
      createParams
    );
    
    const updateSuccess = check(updateResponse, {
      'recipe update status is 200': (r) => r.status === 200,
      'recipe update response time < 1.5s': (r) => r.timings.duration < 1500,
    });
    
    errorRate.add(!updateSuccess);
    responseTime.add(updateResponse.timings.duration);
    apiRequests.add(1);
  }
}

function testAIEndpoints() {
  if (!authToken) return;
  
  const aiPayload = {
    message: 'Suggest a recipe for chicken and rice',
    context: 'dinner',
  };
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${authToken}`,
    },
    tags: { endpoint: 'ai' },
    timeout: '30s',  // AI endpoints may take longer
  };
  
  const response = http.post(
    `${BASE_URL}/api/ai/suggest`,
    JSON.stringify(aiPayload),
    params
  );
  
  const success = check(response, {
    'AI suggestion status is 200': (r) => r.status === 200,
    'AI suggestion response time < 15s': (r) => r.timings.duration < 15000,
    'AI suggestion has content': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.suggestion && body.suggestion.length > 0;
      } catch (e) {
        return false;
      }
    },
  });
  
  errorRate.add(!success);
  responseTime.add(response.timings.duration);
  apiRequests.add(1);
}

export function teardown(data) {
  console.log('Performance tests completed');
  
  // Log final metrics
  console.log('Final Metrics Summary:');
  console.log(`- Total API Requests: ${apiRequests.count}`);
  console.log(`- Authentication Failures: ${authFailures.count}`);
  console.log(`- Average Response Time: ${responseTime.avg}ms`);
  console.log(`- Error Rate: ${(errorRate.rate * 100).toFixed(2)}%`);
}