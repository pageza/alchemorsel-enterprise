#!/usr/bin/env node

// Quick E2E Validation Test for CI Readiness
// Tests core application functionality and service health

const puppeteer = require('puppeteer');

const BASE_URL = process.env.BASE_URL || 'http://web:8080';
const API_URL = process.env.API_URL || 'http://api:8080';
const TIMEOUT = parseInt(process.env.TIMEOUT) || 30000;

console.log('🧪 Quick E2E Validation Test');
console.log('=============================');
console.log(`📍 Testing: ${BASE_URL}`);
console.log(`⏱️  Timeout: ${TIMEOUT}ms`);
console.log();

async function runValidation() {
  let browser;
  const results = {
    tests: [],
    passed: 0,
    failed: 0,
    errors: []
  };

  function addTest(name, success, error = null) {
    results.tests.push({ name, success, error });
    if (success) results.passed++;
    else results.failed++;
    
    const status = success ? '✅' : '❌';
    console.log(`${status} ${name}${error ? ': ' + error : ''}`);
  }

  try {
    // Test 1: Launch Browser
    try {
      browser = await puppeteer.launch({
        headless: true,
        args: [
          '--no-sandbox',
          '--disable-setuid-sandbox',
          '--disable-dev-shm-usage',
          '--disable-gpu',
          '--disable-web-security'
        ]
      });
      addTest('Browser Launch', true);
    } catch (error) {
      addTest('Browser Launch', false, error.message);
      throw error;
    }

    const page = await browser.newPage();
    await page.setViewport({ width: 1280, height: 800 });

    // Test 2: Home Page Load
    try {
      await page.goto(BASE_URL, { 
        waitUntil: 'networkidle2', 
        timeout: TIMEOUT 
      });
      
      const title = await page.title();
      const hasTitle = title && title.toLowerCase().includes('alchemorsel');
      
      if (hasTitle) {
        addTest(`Home Page Load (${title})`, true);
      } else {
        addTest('Home Page Load', false, `Invalid title: ${title}`);
      }
    } catch (error) {
      addTest('Home Page Load', false, error.message);
    }

    // Test 3: Page Content Check
    try {
      const bodyText = await page.evaluate(() => document.body.innerText);
      const hasContent = bodyText && bodyText.length > 100;
      
      if (hasContent) {
        addTest(`Page Content (${bodyText.length} chars)`, true);
      } else {
        addTest('Page Content', false, 'Insufficient content');
      }
    } catch (error) {
      addTest('Page Content', false, error.message);
    }

    // Test 4: Navigation Elements
    try {
      const navElements = await page.$$eval('nav, header, [role="navigation"]', 
        elements => elements.length
      );
      
      if (navElements > 0) {
        addTest(`Navigation Elements (${navElements} found)`, true);
      } else {
        addTest('Navigation Elements', false, 'No navigation found');
      }
    } catch (error) {
      addTest('Navigation Elements', false, error.message);
    }

    // Test 5: JavaScript Execution
    try {
      const jsWorking = await page.evaluate(() => {
        return typeof window !== 'undefined' && 
               typeof document !== 'undefined' &&
               window.location.href.length > 0;
      });
      
      addTest('JavaScript Execution', jsWorking);
    } catch (error) {
      addTest('JavaScript Execution', false, error.message);
    }

    // Test 6: HTMX Detection (if present)
    try {
      const htmxPresent = await page.evaluate(() => {
        return typeof htmx !== 'undefined' || 
               document.querySelector('[hx-get], [hx-post], [hx-trigger]') !== null;
      });
      
      if (htmxPresent) {
        addTest('HTMX Integration', true);
      } else {
        addTest('HTMX Integration', true, 'Not detected (optional)');
      }
    } catch (error) {
      addTest('HTMX Integration', false, error.message);
    }

  } catch (criticalError) {
    results.errors.push(`Critical Error: ${criticalError.message}`);
    console.log(`💥 Critical Error: ${criticalError.message}`);
  } finally {
    if (browser) {
      await browser.close();
    }
  }

  // Summary
  console.log();
  console.log('📊 Test Summary');
  console.log('===============');
  console.log(`✅ Passed: ${results.passed}`);
  console.log(`❌ Failed: ${results.failed}`);
  console.log(`📈 Success Rate: ${Math.round((results.passed / results.tests.length) * 100)}%`);
  
  if (results.errors.length > 0) {
    console.log();
    console.log('🚨 Critical Errors:');
    results.errors.forEach(error => console.log(`  - ${error}`));
  }

  console.log();
  
  // CI Readiness Assessment
  const successRate = (results.passed / results.tests.length) * 100;
  const criticalFailures = results.failed > 2 || results.errors.length > 0;
  
  if (successRate >= 80 && !criticalFailures) {
    console.log('🎉 CI READY: Application is functioning properly');
    console.log('   ✅ Core functionality verified');
    console.log('   ✅ No critical failures detected');
    console.log('   ✅ Safe to proceed with CI/CD pipeline');
    process.exit(0);
  } else {
    console.log('⚠️  CI NOT READY: Issues detected');
    console.log(`   📊 Success rate: ${Math.round(successRate)}% (need ≥80%)`);
    console.log(`   🔍 Failed tests: ${results.failed}`);
    console.log('   🚨 Review failures before pushing to CI');
    process.exit(1);
  }
}

// Handle interruption
process.on('SIGINT', () => {
  console.log('\n⚠️  Test interrupted');
  process.exit(130);
});

// Run the validation
runValidation().catch(error => {
  console.error('💥 Validation failed:', error.message);
  process.exit(1);
});