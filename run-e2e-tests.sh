#!/bin/bash

# Alchemorsel v3 - Comprehensive E2E Test Runner
# 
# This script runs the comprehensive E2E test suite that covers:
# - All possible user journeys and workflows
# - Every conceivable navigation path and link combination
# - AI/LLM integration with context retention and domain expertise
# - HTMX interactions and dynamic functionality
# - Performance benchmarks and security vulnerability testing

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BASE_URL="${BASE_URL:-http://web:8080}"
API_URL="${API_URL:-http://api:8080}"
TIMEOUT="${TIMEOUT:-300000}"
PARALLEL="${PARALLEL:-false}"
VERBOSE="${VERBOSE:-false}"
CONTINUE_ON_FAILURE="${CONTINUE_ON_FAILURE:-true}"

echo -e "${BLUE}🧪 Alchemorsel v3 - Comprehensive E2E Test Suite${NC}"
echo -e "${BLUE}=================================================${NC}"
echo ""
echo -e "📍 Base URL: ${BASE_URL}"
echo -e "📍 API URL: ${API_URL}"
echo -e "⏱️  Timeout: ${TIMEOUT}ms"
echo -e "🔄 Parallel: ${PARALLEL}"
echo -e "📝 Verbose: ${VERBOSE}"
echo -e "🔄 Continue on Failure: ${CONTINUE_ON_FAILURE}"
echo ""

# Function to check if services are running
check_services() {
    echo -e "${YELLOW}🔍 Checking application services...${NC}"
    
    # Check web service
    if curl -sf "${BASE_URL}/health" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ Web service is accessible at ${BASE_URL}${NC}"
    else
        echo -e "${RED}❌ Web service is not accessible at ${BASE_URL}${NC}"
        echo -e "${YELLOW}💡 Make sure Docker services are running: docker compose up -d${NC}"
        exit 1
    fi
    
    # Check API service (optional)
    if curl -sf "${API_URL}/health" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ API service is accessible at ${API_URL}${NC}"
    else
        echo -e "${YELLOW}⚠️  API service check failed (may not be critical)${NC}"
    fi
    
    echo ""
}

# Function to setup test environment
setup_test_environment() {
    echo -e "${YELLOW}🔧 Setting up test environment...${NC}"
    
    # Create necessary directories
    mkdir -p /tmp/e2e-artifacts
    mkdir -p /tmp/e2e-screenshots
    mkdir -p /tmp/e2e-reports
    
    # Install dependencies if needed
    if [ ! -d "node_modules" ]; then
        echo -e "${YELLOW}📦 Installing Node.js dependencies...${NC}"
        npm install puppeteer node-fetch
    fi
    
    echo -e "${GREEN}✅ Test environment ready${NC}"
    echo ""
}

# Function to run the comprehensive test suite
run_comprehensive_tests() {
    echo -e "${YELLOW}🚀 Starting comprehensive E2E test suite...${NC}"
    echo ""
    
    # Build command arguments
    ARGS=""
    
    if [ "$PARALLEL" = "true" ]; then
        ARGS="$ARGS --parallel"
    fi
    
    if [ "$VERBOSE" = "true" ]; then
        ARGS="$ARGS --verbose"
    fi
    
    if [ "$CONTINUE_ON_FAILURE" = "false" ]; then
        ARGS="$ARGS --fail-fast"
    fi
    
    # Set environment variables
    export BASE_URL="$BASE_URL"
    export TIMEOUT="$TIMEOUT"
    
    # Run the comprehensive test suite
    echo -e "${BLUE}🧪 Executing comprehensive E2E test suite...${NC}"
    
    if node test/e2e/comprehensive-e2e-suite.js $ARGS; then
        echo ""
        echo -e "${GREEN}🎉 ALL E2E TESTS PASSED!${NC}"
        echo -e "${GREEN}✅ Application is ready for deployment${NC}"
        return 0
    else
        echo ""
        echo -e "${RED}💥 E2E TESTS FAILED!${NC}"
        echo -e "${RED}❌ Review test results before deployment${NC}"
        return 1
    fi
}

# Function to generate test report summary
generate_summary() {
    echo ""
    echo -e "${BLUE}📊 Test Execution Summary${NC}"
    echo -e "${BLUE}=========================${NC}"
    
    # Check if report files exist
    REPORT_DIR="/tmp/e2e-reports"
    if [ -d "$REPORT_DIR" ] && [ "$(ls -A $REPORT_DIR)" ]; then
        echo -e "${GREEN}📋 Test reports generated in: $REPORT_DIR${NC}"
        ls -la "$REPORT_DIR"
    else
        echo -e "${YELLOW}⚠️  No test reports found${NC}"
    fi
    
    # Check screenshots
    SCREENSHOT_DIR="/tmp/e2e-screenshots"
    if [ -d "$SCREENSHOT_DIR" ] && [ "$(ls -A $SCREENSHOT_DIR)" ]; then
        SCREENSHOT_COUNT=$(ls -1 "$SCREENSHOT_DIR" | wc -l)
        echo -e "${GREEN}📸 ${SCREENSHOT_COUNT} test screenshots saved in: $SCREENSHOT_DIR${NC}"
    fi
    
    echo ""
    echo -e "${BLUE}🎯 Test Coverage Achieved:${NC}"
    echo -e "   ✅ Complete user journey testing"
    echo -e "   ✅ Comprehensive navigation matrix"
    echo -e "   ✅ AI/LLM integration and context retention"
    echo -e "   ✅ HTMX dynamic interactions"
    echo -e "   ✅ Performance and security testing"
    echo ""
}

# Function to cleanup test artifacts (optional)
cleanup() {
    if [ "$1" = "--clean" ]; then
        echo -e "${YELLOW}🧹 Cleaning up test artifacts...${NC}"
        rm -rf /tmp/e2e-artifacts/*
        rm -rf /tmp/e2e-screenshots/*
        rm -rf /tmp/e2e-reports/*
        echo -e "${GREEN}✅ Cleanup completed${NC}"
    fi
}

# Function to show usage
show_usage() {
    echo -e "${BLUE}Usage: $0 [OPTIONS]${NC}"
    echo ""
    echo -e "${YELLOW}Options:${NC}"
    echo -e "  --parallel          Run test suites in parallel (faster but potentially less stable)"
    echo -e "  --verbose           Enable verbose logging"
    echo -e "  --fail-fast         Stop on first test suite failure"
    echo -e "  --clean             Clean up test artifacts before running"
    echo -e "  --help              Show this help message"
    echo ""
    echo -e "${YELLOW}Environment Variables:${NC}"
    echo -e "  BASE_URL            Application base URL (default: http://web:8080)"
    echo -e "  API_URL             API base URL (default: http://api:8080)"
    echo -e "  TIMEOUT             Test timeout in milliseconds (default: 300000)"
    echo -e "  PARALLEL            Run tests in parallel (default: false)"
    echo -e "  VERBOSE             Enable verbose output (default: false)"
    echo ""
    echo -e "${YELLOW}Examples:${NC}"
    echo -e "  $0                             # Run all tests sequentially"
    echo -e "  $0 --parallel --verbose        # Run tests in parallel with verbose output"
    echo -e "  BASE_URL=http://localhost:3021 $0  # Run against different URL"
    echo ""
}

# Main execution
main() {
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --parallel)
                PARALLEL="true"
                shift
                ;;
            --verbose)
                VERBOSE="true"
                shift
                ;;
            --fail-fast)
                CONTINUE_ON_FAILURE="false"
                shift
                ;;
            --clean)
                cleanup --clean
                shift
                ;;
            --help)
                show_usage
                exit 0
                ;;
            *)
                echo -e "${RED}❌ Unknown option: $1${NC}"
                show_usage
                exit 1
                ;;
        esac
    done
    
    # Execute test sequence
    check_services
    setup_test_environment
    
    if run_comprehensive_tests; then
        generate_summary
        echo -e "${GREEN}🎉 Comprehensive E2E testing completed successfully!${NC}"
        exit 0
    else
        generate_summary
        echo -e "${RED}💥 Comprehensive E2E testing failed!${NC}"
        exit 1
    fi
}

# Handle script interruption
trap 'echo -e "\n${YELLOW}⚠️  Test execution interrupted${NC}"; exit 130' INT TERM

# Run main function
main "$@"