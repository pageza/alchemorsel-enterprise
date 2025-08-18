// K6 Load Testing Script for Alchemorsel v3
// This script tests the application under various load conditions

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';
import { randomString, randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';

// Custom metrics
export let errorRate = new Rate('errors');
export let responseTime = new Trend('response_time');
export let throughput = new Counter('throughput');

// Test configuration
export let options = {
  stages: [
    // Ramp-up
    { duration: '2m', target: 10 }, // Ramp up to 10 users over 2 minutes
    { duration: '5m', target: 50 }, // Ramp up to 50 users over 5 minutes
    { duration: '10m', target: 100 }, // Steady state with 100 users for 10 minutes
    { duration: '5m', target: 200 }, // Spike to 200 users for 5 minutes
    { duration: '10m', target: 100 }, // Scale down to 100 users for 10 minutes
    { duration: '5m', target: 0 }, // Ramp down over 5 minutes
  ],
  thresholds: {
    // 95% of requests should complete within 500ms
    http_req_duration: ['p(95)<500'],
    // Error rate should be less than 1%
    errors: ['rate<0.01'],
    // 99% of requests should complete within 1s
    http_req_duration: ['p(99)<1000'],
    // Average response time should be less than 200ms
    http_req_duration: ['avg<200'],
  },
  ext: {
    loadimpact: {
      // Cloud execution options
      projectID: 3622169,
      name: 'Alchemorsel v3 Load Test'
    }
  }
};

// Test data
const BASE_URL = __ENV.BASE_URL || 'https://api.alchemorsel.com';
const API_KEY = __ENV.API_KEY || '';

// Sample recipe data for testing
const sampleRecipes = [
  {
    title: 'Chocolate Chip Cookies',
    description: 'Classic homemade chocolate chip cookies',
    ingredients: ['flour', 'butter', 'sugar', 'chocolate chips', 'eggs'],
    instructions: ['Mix ingredients', 'Form dough', 'Bake at 350°F']
  },
  {
    title: 'Pasta Carbonara',
    description: 'Traditional Italian pasta dish',
    ingredients: ['pasta', 'eggs', 'cheese', 'bacon', 'pepper'],
    instructions: ['Boil pasta', 'Cook bacon', 'Mix with eggs and cheese']
  },
  {
    title: 'Beef Tacos',
    description: 'Spicy beef tacos with fresh toppings',
    ingredients: ['ground beef', 'taco shells', 'lettuce', 'tomatoes', 'cheese'],
    instructions: ['Cook beef', 'Warm shells', 'Assemble tacos']
  }
];

// Authentication helper
function authenticate() {
  const payload = JSON.stringify({
    email: `test.user.${randomString(8)}@example.com`,
    password: 'Test123!@#'
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  const response = http.post(`${BASE_URL}/auth/register`, payload, params);
  
  if (response.status === 201) {
    const loginResponse = http.post(`${BASE_URL}/auth/login`, payload, params);
    if (loginResponse.status === 200) {
      return JSON.parse(loginResponse.body).token;
    }
  }
  
  return null;
}

// Main test function
export default function () {
  const token = authenticate();
  
  if (!token) {
    errorRate.add(1);
    return;
  }

  const authHeaders = {
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
  };

  // Test scenario selection (weighted)
  const scenario = Math.random();
  
  if (scenario < 0.4) {
    // 40% - Browse recipes
    browseRecipes(authHeaders);
  } else if (scenario < 0.7) {
    // 30% - Search recipes
    searchRecipes(authHeaders);
  } else if (scenario < 0.85) {
    // 15% - Create recipe
    createRecipe(authHeaders);
  } else if (scenario < 0.95) {
    // 10% - View specific recipe
    viewRecipe(authHeaders);
  } else {
    // 5% - AI recipe generation
    generateAIRecipe(authHeaders);
  }

  // Random think time between requests
  sleep(randomIntBetween(1, 3));
}

function browseRecipes(headers) {
  const page = randomIntBetween(1, 10);
  const limit = randomIntBetween(10, 50);
  
  const response = http.get(`${BASE_URL}/api/recipes?page=${page}&limit=${limit}`, headers);
  
  const success = check(response, {
    'browse recipes status is 200': (r) => r.status === 200,
    'browse recipes response time < 500ms': (r) => r.timings.duration < 500,
    'browse recipes has recipes': (r) => {
      try {
        const data = JSON.parse(r.body);
        return data.recipes && Array.isArray(data.recipes);
      } catch (e) {
        return false;
      }
    },
  });

  responseTime.add(response.timings.duration);
  throughput.add(1);
  
  if (!success) {
    errorRate.add(1);
  }
}

function searchRecipes(headers) {
  const searchTerms = ['chicken', 'pasta', 'vegetarian', 'dessert', 'quick', 'healthy'];
  const query = searchTerms[randomIntBetween(0, searchTerms.length - 1)];
  
  const response = http.get(`${BASE_URL}/api/recipes/search?q=${query}`, headers);
  
  const success = check(response, {
    'search recipes status is 200': (r) => r.status === 200,
    'search recipes response time < 800ms': (r) => r.timings.duration < 800,
    'search recipes has results': (r) => {
      try {
        const data = JSON.parse(r.body);
        return data.results !== undefined;
      } catch (e) {
        return false;
      }
    },
  });

  responseTime.add(response.timings.duration);
  throughput.add(1);
  
  if (!success) {
    errorRate.add(1);
  }
}

function createRecipe(headers) {
  const recipe = sampleRecipes[randomIntBetween(0, sampleRecipes.length - 1)];
  const payload = JSON.stringify({
    ...recipe,
    title: `${recipe.title} ${randomString(4)}` // Make title unique
  });
  
  const response = http.post(`${BASE_URL}/api/recipes`, payload, headers);
  
  const success = check(response, {
    'create recipe status is 201': (r) => r.status === 201,
    'create recipe response time < 1000ms': (r) => r.timings.duration < 1000,
    'create recipe returns ID': (r) => {
      try {
        const data = JSON.parse(r.body);
        return data.id !== undefined;
      } catch (e) {
        return false;
      }
    },
  });

  responseTime.add(response.timings.duration);
  throughput.add(1);
  
  if (!success) {
    errorRate.add(1);
  }
}

function viewRecipe(headers) {
  // Assume recipe IDs are UUIDs or incrementing integers
  const recipeId = randomString(8) + '-' + randomString(4) + '-' + randomString(4) + '-' + randomString(4) + '-' + randomString(12);
  
  const response = http.get(`${BASE_URL}/api/recipes/${recipeId}`, headers);
  
  const success = check(response, {
    'view recipe status is 200 or 404': (r) => r.status === 200 || r.status === 404,
    'view recipe response time < 300ms': (r) => r.timings.duration < 300,
  });

  responseTime.add(response.timings.duration);
  throughput.add(1);
  
  if (!success) {
    errorRate.add(1);
  }
}

function generateAIRecipe(headers) {
  const cuisines = ['Italian', 'Mexican', 'Asian', 'American', 'French', 'Indian'];
  const dietTypes = ['vegetarian', 'vegan', 'keto', 'paleo', 'gluten-free'];
  
  const payload = JSON.stringify({
    cuisine: cuisines[randomIntBetween(0, cuisines.length - 1)],
    diet: dietTypes[randomIntBetween(0, dietTypes.length - 1)],
    ingredients: ['chicken', 'rice', 'vegetables'],
    cookingTime: randomIntBetween(15, 60)
  });
  
  const response = http.post(`${BASE_URL}/api/ai/generate-recipe`, payload, headers);
  
  const success = check(response, {
    'AI recipe generation status is 200': (r) => r.status === 200,
    'AI recipe generation response time < 5000ms': (r) => r.timings.duration < 5000, // AI requests can be slower
    'AI recipe generation returns recipe': (r) => {
      try {
        const data = JSON.parse(r.body);
        return data.recipe !== undefined;
      } catch (e) {
        return false;
      }
    },
  });

  responseTime.add(response.timings.duration);
  throughput.add(1);
  
  if (!success) {
    errorRate.add(1);
  }
}

// Health check function
export function setup() {
  console.log('Starting load test setup...');
  
  const response = http.get(`${BASE_URL}/health`);
  
  check(response, {
    'setup: health check status is 200': (r) => r.status === 200,
  });
  
  if (response.status !== 200) {
    throw new Error('Application is not healthy, aborting test');
  }
  
  console.log('Load test setup completed successfully');
}

// Teardown function
export function teardown(data) {
  console.log('Load test completed');
  console.log(`Final error rate: ${errorRate.rate * 100}%`);
  console.log(`Average response time: ${responseTime.avg}ms`);
  console.log(`Total requests: ${throughput.count}`);
}

// Handle summary for detailed reporting
export function handleSummary(data) {
  return {
    'loadtest-summary.json': JSON.stringify(data, null, 2),
    'stdout': createSummaryText(data),
  };
}

function createSummaryText(data) {
  const summary = `
Load Test Summary
=================

Test Duration: ${data.state.testRunDurationMs / 1000}s
Total Requests: ${data.metrics.http_reqs.count}
Request Rate: ${data.metrics.http_reqs.rate.toFixed(2)} req/s

Response Times:
- Average: ${data.metrics.http_req_duration.avg.toFixed(2)}ms
- 95th Percentile: ${data.metrics.http_req_duration['p(95)'].toFixed(2)}ms
- 99th Percentile: ${data.metrics.http_req_duration['p(99)'].toFixed(2)}ms
- Max: ${data.metrics.http_req_duration.max.toFixed(2)}ms

Error Rate: ${(data.metrics.errors.rate * 100).toFixed(2)}%

Thresholds:
${Object.entries(data.thresholds)
  .map(([name, threshold]) => `- ${name}: ${threshold.ok ? 'PASS' : 'FAIL'}`)
  .join('\n')}

Data Transfer:
- Received: ${(data.metrics.data_received.count / 1024 / 1024).toFixed(2)} MB
- Sent: ${(data.metrics.data_sent.count / 1024 / 1024).toFixed(2)} MB
`;

  return summary;
}