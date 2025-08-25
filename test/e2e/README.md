# Alchemorsel v3 E2E Testing Framework

Comprehensive End-to-End testing suite for the Alchemorsel v3 Docker deployment, including API integration tests, web frontend tests, and performance validation.

## Overview

This testing framework validates the live Docker deployment running on:
- **API Backend**: `http://localhost:3013` 
- **Web Frontend**: `http://localhost:3014`

## Test Suite Components

### 1. API Integration Tests (`api_integration_test.js`)
- **Purpose**: Test all API endpoints against the live Docker deployment
- **Coverage**: 
  - Health endpoint validation
  - Recipe API endpoints
  - Error handling and edge cases
  - CORS headers and security
  - Performance metrics and response times
  - Concurrent request handling

### 2. Comprehensive E2E Tests (`docker_deployment_e2e_test.js`)
- **Purpose**: Full user journey and web frontend testing
- **Coverage**:
  - Web frontend loading and navigation
  - User interaction flows
  - Performance optimization validation (14KB first packet)
  - Responsive design testing
  - Network activity monitoring
  - Security headers validation

### 3. Test Runner Script (`run-docker-tests.sh`)
- **Purpose**: Orchestrate all test execution with proper setup and reporting
- **Features**:
  - Docker container health checks
  - Service responsiveness validation
  - Dependency installation
  - Combined test execution
  - Comprehensive reporting

## Quick Start

### Prerequisites
1. Docker and Docker Compose installed
2. Node.js 18+ and npm installed
3. Alchemorsel v3 Docker containers running

### Start Docker Containers
```bash
# From project root
docker-compose up -d

# Verify containers are running
docker ps | grep alchemorsel
```

### Install Dependencies
```bash
# From project root
npm install
```

### Run Tests

#### Run All Tests (Recommended)
```bash
npm run test:docker
```

#### Run Individual Test Suites
```bash
# API Integration Tests only
npm run test:docker:api

# Web E2E Tests only  
npm run test:docker:e2e

# Individual test files
npm run test:api:docker
npm run test:e2e:docker
```

### Run Tests with Visual Browser (Non-headless)
```bash
E2E_HEADLESS=false npm run test:docker
```

## Test Output and Reports

### Generated Files
```
test/e2e/screenshots/
├── combined_test_report.md           # Human-readable summary
├── api_integration_test_report.json  # API test results
├── docker_e2e_test_report.json      # E2E test results
├── *.png                            # Screenshots from tests
└── test_report.json                 # Legacy report format

test/e2e/logs/
├── api_integration_test.log          # API test execution log
├── docker_deployment_e2e_test.log    # E2E test execution log
└── original_puppeteer_test.log       # Legacy test log
```

### Report Contents
- **Test Success/Failure Summary**: Pass/fail counts and rates
- **Performance Metrics**: Response times, first packet sizes, network analysis  
- **Security Validation**: CORS headers, security headers
- **Error Analysis**: Detailed failure information and warnings
- **Visual Evidence**: Screenshots of test execution

## Performance Validation

### 14KB First Packet Optimization
The E2E tests specifically validate the 14KB first packet optimization:
- Monitors network responses for HTML content
- Measures first packet sizes
- Reports optimization compliance rate
- Flags responses exceeding the 14KB target

### API Performance Thresholds
- **Health Endpoint**: < 50ms response time
- **Data Endpoints**: < 200ms response time  
- **General API**: < 100ms average response time

## Customization

### Environment Variables
```bash
E2E_HEADLESS=false     # Run browser tests in visible mode
E2E_TIMEOUT=300000     # Set custom timeout (5 minutes)
```

### Configuration Files
- `api_integration_test.js`: API test configuration and thresholds
- `docker_deployment_e2e_test.js`: E2E test configuration and viewports
- `run-docker-tests.sh`: Test runner configuration

## Test Architecture

### API Integration Testing
1. **Health Validation**: Verify service health and dependencies
2. **Endpoint Testing**: Test all available API endpoints
3. **Error Handling**: Validate proper error responses
4. **Performance Testing**: Measure response times and concurrent handling
5. **Security Testing**: Validate CORS and security headers

### E2E Web Testing  
1. **Infrastructure Validation**: Check API health before web testing
2. **Frontend Loading**: Validate initial page load and HTMX integration
3. **Navigation Testing**: Test all navigation elements and links
4. **Performance Analysis**: Monitor network requests and optimize
5. **User Journey Simulation**: Test realistic user workflows
6. **Responsive Design**: Validate multiple viewport sizes
7. **Security Headers**: Check web security implementations

## Debugging Failed Tests

### Common Issues
1. **Containers Not Running**: Check `docker ps` and restart with `docker-compose up -d`
2. **Services Not Ready**: Wait for health checks to pass (can take 30+ seconds)
3. **Network Issues**: Ensure ports 3013 and 3014 are accessible
4. **Dependencies Missing**: Run `npm install` in project root

### Debug Commands
```bash
# Check container health
docker-compose ps

# View container logs
docker-compose logs api
docker-compose logs web

# Test service endpoints manually
curl http://localhost:3013/health
curl http://localhost:3014/

# Run tests with verbose output
DEBUG=true npm run test:docker

# Run single test with visible browser
E2E_HEADLESS=false npm run test:e2e:docker
```

### Log Analysis
- Check `test/e2e/logs/` for detailed execution logs
- Review `test/e2e/screenshots/` for visual debugging
- Examine JSON reports for specific error details

## Contributing

### Adding New Tests
1. Add test functions to appropriate test files
2. Update configuration if needed
3. Run tests locally to verify
4. Update documentation

### Test Naming Conventions
- **API Tests**: `testAPIFunctionName()`
- **E2E Tests**: `testWebFunctionName()`  
- **Test Steps**: Clear descriptive names in `testStep()` calls

### Performance Benchmarks
- Always include response time validation
- Set appropriate thresholds for test types
- Monitor trends in performance reports

## Integration with CI/CD

The test framework is designed for integration with continuous integration:

```yaml
# Example GitHub Actions integration
- name: Start Docker Services
  run: docker-compose up -d
  
- name: Wait for Services
  run: sleep 30
  
- name: Run E2E Tests
  run: npm run test:docker
  
- name: Upload Test Reports
  uses: actions/upload-artifact@v2
  with:
    name: test-reports
    path: test/e2e/screenshots/
```

## Support

For issues with the E2E testing framework:
1. Check Docker container status and logs
2. Verify service endpoints are responding
3. Review test logs and screenshots
4. Check for recent changes to API or web frontend
5. Ensure test dependencies are up to date

---

*This testing framework ensures comprehensive validation of the Alchemorsel v3 Docker deployment, covering functionality, performance, and security requirements.*