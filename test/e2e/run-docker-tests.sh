#!/bin/bash
# Comprehensive E2E Test Runner for Alchemorsel v3 Docker Deployment
# This script runs all E2E tests against the live Docker deployment

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
LOG_DIR="${PROJECT_ROOT}/test/e2e/logs"
SCREENSHOT_DIR="${PROJECT_ROOT}/test/e2e/screenshots"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Create necessary directories
create_directories() {
    log_info "Creating test directories..."
    mkdir -p "${LOG_DIR}"
    mkdir -p "${SCREENSHOT_DIR}"
}

# Check if Docker containers are running
check_docker_containers() {
    log_info "Checking Docker container status..."
    
    # Check if API container is running
    if ! docker ps | grep -q "alchemorsel-api"; then
        log_error "API container (alchemorsel-api) is not running"
        log_info "Please start the containers with: docker-compose up -d"
        exit 1
    fi
    
    # Check if Web container is running
    if ! docker ps | grep -q "alchemorsel-web"; then
        log_error "Web container (alchemorsel-web) is not running"
        log_info "Please start the containers with: docker-compose up -d"
        exit 1
    fi
    
    log_success "Docker containers are running"
}

# Check if services are responsive
check_service_health() {
    log_info "Checking service health..."
    
    # Check API health
    if ! curl -s --max-time 10 http://localhost:3013/health > /dev/null; then
        log_error "API service at http://localhost:3013 is not responsive"
        log_info "Waiting 30 seconds for services to start up..."
        sleep 30
        
        if ! curl -s --max-time 10 http://localhost:3013/health > /dev/null; then
            log_error "API service still not responsive after waiting"
            exit 1
        fi
    fi
    
    # Check Web frontend
    if ! curl -s --max-time 10 http://localhost:3014/ > /dev/null; then
        log_error "Web service at http://localhost:3014 is not responsive"
        log_info "Waiting 30 seconds for services to start up..."
        sleep 30
        
        if ! curl -s --max-time 10 http://localhost:3014/ > /dev/null; then
            log_error "Web service still not responsive after waiting"
            exit 1
        fi
    fi
    
    log_success "Services are responsive"
}

# Install dependencies if needed
install_dependencies() {
    log_info "Checking Node.js dependencies..."
    
    cd "${PROJECT_ROOT}"
    
    if [ ! -d "node_modules" ]; then
        log_info "Installing Node.js dependencies..."
        npm install
    else
        log_success "Node.js dependencies are installed"
    fi
}

# Run API Integration Tests
run_api_tests() {
    log_info "Running API Integration Tests..."
    
    cd "${PROJECT_ROOT}"
    
    local log_file="${LOG_DIR}/api_integration_test.log"
    
    if node test/e2e/api_integration_test.js 2>&1 | tee "${log_file}"; then
        log_success "API Integration Tests completed successfully"
        return 0
    else
        log_error "API Integration Tests failed"
        return 1
    fi
}

# Run Comprehensive E2E Tests
run_e2e_tests() {
    log_info "Running Comprehensive E2E Tests..."
    
    cd "${PROJECT_ROOT}"
    
    local log_file="${LOG_DIR}/docker_deployment_e2e_test.log"
    
    # Set environment variables for E2E tests
    export E2E_HEADLESS=${E2E_HEADLESS:-true}
    export E2E_TIMEOUT=${E2E_TIMEOUT:-180000}
    
    if node test/e2e/docker_deployment_e2e_test.js 2>&1 | tee "${log_file}"; then
        log_success "Comprehensive E2E Tests completed successfully"
        return 0
    else
        log_error "Comprehensive E2E Tests failed"
        return 1
    fi
}

# Run original Puppeteer tests (updated for correct ports)
run_original_puppeteer_tests() {
    log_info "Running Original Puppeteer Tests (if available)..."
    
    cd "${PROJECT_ROOT}"
    
    if [ -f "test/e2e/comprehensive_puppeteer_test.js" ]; then
        local log_file="${LOG_DIR}/original_puppeteer_test.log"
        
        # Update the original test file to use correct ports
        if node test/e2e/comprehensive_puppeteer_test.js 2>&1 | tee "${log_file}"; then
            log_success "Original Puppeteer Tests completed successfully"
            return 0
        else
            log_warning "Original Puppeteer Tests failed (this may be expected)"
            return 1
        fi
    else
        log_warning "Original Puppeteer test file not found, skipping"
        return 0
    fi
}

# Generate combined test report
generate_combined_report() {
    log_info "Generating combined test report..."
    
    local report_file="${SCREENSHOT_DIR}/combined_test_report.md"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    cat > "${report_file}" << EOF
# Alchemorsel v3 E2E Test Report

**Test Execution Date:** ${timestamp}  
**Docker Deployment Tested:** API (localhost:3013), Web (localhost:3014)

## Test Summary

### Test Suites Executed:
1. API Integration Tests
2. Comprehensive E2E Tests
3. Original Puppeteer Tests (legacy)

## Detailed Results

### API Integration Tests
EOF

    if [ -f "${SCREENSHOT_DIR}/api_integration_test_report.json" ]; then
        echo "- ✅ API Integration Test Report: [api_integration_test_report.json](./api_integration_test_report.json)" >> "${report_file}"
    else
        echo "- ❌ API Integration Test Report: Not generated" >> "${report_file}"
    fi

    cat >> "${report_file}" << EOF

### Comprehensive E2E Tests
EOF

    if [ -f "${SCREENSHOT_DIR}/docker_e2e_test_report.json" ]; then
        echo "- ✅ E2E Test Report: [docker_e2e_test_report.json](./docker_e2e_test_report.json)" >> "${report_file}"
    else
        echo "- ❌ E2E Test Report: Not generated" >> "${report_file}"
    fi

    # Count screenshots
    local screenshot_count=$(find "${SCREENSHOT_DIR}" -name "*.png" 2>/dev/null | wc -l)
    
    cat >> "${report_file}" << EOF

### Screenshots and Evidence
- **Screenshots Captured:** ${screenshot_count}
- **Screenshot Directory:** \`test/e2e/screenshots/\`

### Log Files
- **Log Directory:** \`test/e2e/logs/\`

## Performance Metrics

Check the JSON reports for detailed performance metrics including:
- API response times
- First packet sizes (14KB optimization validation)
- Network request analysis
- Page load performance

## Next Steps

1. Review detailed JSON reports for specific test results
2. Examine screenshots for visual validation
3. Check log files for detailed execution traces
4. Address any failing tests or performance issues

---
*Generated by Alchemorsel v3 E2E Test Suite*
EOF

    log_success "Combined test report generated: ${report_file}"
}

# Main execution
main() {
    echo "🧪 Alchemorsel v3 E2E Test Suite"
    echo "Testing Docker Deployment (API: localhost:3013, Web: localhost:3014)"
    echo "================================================================================"
    
    local start_time=$(date +%s)
    local test_results=()
    
    # Setup
    create_directories
    check_docker_containers
    check_service_health
    install_dependencies
    
    echo ""
    echo "🚀 Starting Test Execution..."
    echo "================================================================================"
    
    # Run API Integration Tests
    echo ""
    if run_api_tests; then
        test_results+=("API_TESTS:PASS")
    else
        test_results+=("API_TESTS:FAIL")
    fi
    
    # Run Comprehensive E2E Tests
    echo ""
    if run_e2e_tests; then
        test_results+=("E2E_TESTS:PASS")
    else
        test_results+=("E2E_TESTS:FAIL")
    fi
    
    # Run Original Puppeteer Tests (optional)
    echo ""
    if run_original_puppeteer_tests; then
        test_results+=("ORIGINAL_TESTS:PASS")
    else
        test_results+=("ORIGINAL_TESTS:FAIL")
    fi
    
    # Generate combined report
    echo ""
    generate_combined_report
    
    # Final summary
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    echo ""
    echo "================================================================================"
    echo "🏁 Test Execution Complete"
    echo "================================================================================"
    echo "Duration: ${duration} seconds"
    echo ""
    echo "Test Results:"
    
    local failed_tests=0
    for result in "${test_results[@]}"; do
        local test_name=$(echo "$result" | cut -d: -f1)
        local test_status=$(echo "$result" | cut -d: -f2)
        
        if [ "$test_status" = "PASS" ]; then
            log_success "${test_name}: PASSED"
        else
            log_error "${test_name}: FAILED"
            ((failed_tests++))
        fi
    done
    
    echo ""
    echo "📊 Summary:"
    echo "- Total Test Suites: ${#test_results[@]}"
    echo "- Passed: $((${#test_results[@]} - failed_tests))"
    echo "- Failed: ${failed_tests}"
    
    if [ $failed_tests -gt 0 ]; then
        echo ""
        log_error "Some tests failed. Check the detailed reports and logs for more information."
        echo ""
        echo "Reports available at:"
        echo "- ${SCREENSHOT_DIR}/combined_test_report.md"
        echo "- ${SCREENSHOT_DIR}/*.json"
        echo "- ${LOG_DIR}/*.log"
        
        exit 1
    else
        echo ""
        log_success "All tests passed successfully!"
        echo ""
        echo "Reports available at:"
        echo "- ${SCREENSHOT_DIR}/combined_test_report.md"
        echo "- ${SCREENSHOT_DIR}/*.json"
        
        exit 0
    fi
}

# Handle script arguments
case "${1:-}" in
    --api-only)
        log_info "Running API tests only..."
        create_directories
        check_docker_containers
        check_service_health
        install_dependencies
        run_api_tests
        exit $?
        ;;
    --e2e-only)
        log_info "Running E2E tests only..."
        create_directories
        check_docker_containers
        check_service_health
        install_dependencies
        run_e2e_tests
        exit $?
        ;;
    --help)
        echo "Usage: $0 [OPTIONS]"
        echo ""
        echo "Options:"
        echo "  --api-only    Run only API integration tests"
        echo "  --e2e-only    Run only comprehensive E2E tests"
        echo "  --help        Show this help message"
        echo ""
        echo "Environment Variables:"
        echo "  E2E_HEADLESS  Set to 'false' to run browser tests in non-headless mode"
        echo "  E2E_TIMEOUT   Set custom timeout for E2E tests (default: 180000ms)"
        exit 0
        ;;
    "")
        # Default: run all tests
        main
        ;;
    *)
        log_error "Unknown option: $1"
        echo "Use --help for usage information"
        exit 1
        ;;
esac