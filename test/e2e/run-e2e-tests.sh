#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 Starting E2E Test Runner for Alchemorsel${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════${NC}"

# Check if services are running
echo -e "\n${YELLOW}📋 Checking service status...${NC}"
docker compose ps

# Get the correct host IP for Docker network
HOST_IP=$(docker network inspect alchemorsel-enterprise_alchemorsel-network -f '{{range .IPAM.Config}}{{.Gateway}}{{end}}')
echo -e "${BLUE}Using host IP: ${HOST_IP}${NC}"

# Create a modified test script that uses the correct URLs
cat > /tmp/user-journeys-docker.js << 'EOF'
const puppeteer = require('puppeteer');
const assert = require('assert');

// Configuration - Use host.docker.internal or Docker bridge IP
const BASE_URL = process.env.BASE_URL || 'http://web:8080';
const API_URL = process.env.API_URL || 'http://api:8080';

console.log('Testing against:', { BASE_URL, API_URL });
EOF

# Append the rest of the test script
tail -n +7 /workspace/alchemorsel-enterprise/test/e2e/user-journeys.js >> /tmp/user-journeys-docker.js

# Run Puppeteer in Docker container
echo -e "\n${YELLOW}🧪 Running Puppeteer tests in Docker container...${NC}"

docker run --rm \
  --network alchemorsel-enterprise_alchemorsel-network \
  -v /tmp/user-journeys-docker.js:/app/user-journeys.js:ro \
  -v /tmp:/tmp \
  -e BASE_URL="http://web:8080" \
  -e API_URL="http://api:8080" \
  -e NODE_ENV=test \
  --shm-size=2gb \
  zenika/alpine-chrome:with-puppeteer \
  sh -c "cd /app && npm init -y && npm install puppeteer && node user-journeys.js"

# Check exit code
if [ $? -eq 0 ]; then
    echo -e "\n${GREEN}✅ All E2E tests passed successfully!${NC}"
    echo -e "${YELLOW}📸 Screenshots are available in /tmp/alchemorsel-*.png${NC}"
    
    # List screenshots
    echo -e "\n${BLUE}Available screenshots:${NC}"
    ls -la /tmp/alchemorsel-*.png 2>/dev/null || echo "No screenshots found"
else
    echo -e "\n${RED}❌ E2E tests failed!${NC}"
    exit 1
fi

echo -e "\n${GREEN}═══════════════════════════════════════════════${NC}"
echo -e "${GREEN}✨ E2E Test Run Complete${NC}"