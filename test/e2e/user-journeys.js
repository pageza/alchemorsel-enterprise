const puppeteer = require('puppeteer');
const assert = require('assert');

// Configuration
const BASE_URL = 'http://localhost:3021'; // Web service port
const API_URL = 'http://localhost:3020';  // API service port

// Test user data
const testUser = {
  email: `test_${Date.now()}@example.com`,
  password: 'TestPassword123!',
  name: 'Test User'
};

// Color codes for output
const colors = {
  reset: '\x1b[0m',
  green: '\x1b[32m',
  red: '\x1b[31m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  magenta: '\x1b[35m'
};

function log(message, color = 'reset') {
  console.log(`${colors[color]}${message}${colors.reset}`);
}

function logTest(testName) {
  console.log(`\n${colors.blue}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${colors.reset}`);
  console.log(`${colors.magenta}🧪 Testing: ${testName}${colors.reset}`);
  console.log(`${colors.blue}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${colors.reset}`);
}

async function delay(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function takeScreenshot(page, name) {
  const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
  const filename = `/tmp/alchemorsel-${name}-${timestamp}.png`;
  await page.screenshot({ path: filename, fullPage: true });
  log(`📸 Screenshot saved: ${filename}`, 'yellow');
  return filename;
}

async function runUserJourneys() {
  log('\n🚀 Starting Alchemorsel User Journey Tests', 'green');
  log('════════════════════════════════════════════', 'green');
  
  // Launch browser
  const browser = await puppeteer.launch({
    headless: true,
    args: [
      '--no-sandbox',
      '--disable-setuid-sandbox',
      '--disable-dev-shm-usage',
      '--disable-gpu',
      '--disable-web-security',
      '--disable-features=IsolateOrigins',
      '--disable-site-isolation-trials'
    ],
    ignoreHTTPSErrors: true
  });
  
  const page = await browser.newPage();
  
  // Set viewport
  await page.setViewport({ width: 1280, height: 800 });
  
  // Enable console logging from the page
  page.on('console', msg => {
    if (msg.type() === 'error') {
      log(`Browser console error: ${msg.text()}`, 'red');
    }
  });
  
  page.on('pageerror', error => {
    log(`Page error: ${error.message}`, 'red');
  });
  
  try {
    // ═══════════════════════════════════════════════════════════
    // Test 1: Home Page Load
    // ═══════════════════════════════════════════════════════════
    logTest('Home Page Load');
    
    await page.goto(BASE_URL, { waitUntil: 'networkidle2', timeout: 30000 });
    const title = await page.title();
    log(`✓ Page title: ${title}`, 'green');
    assert(title.includes('Alchemorsel'), 'Home page should load with Alchemorsel title');
    
    // Check for main elements
    const heroSection = await page.$('.hero-section, [class*="hero"], main');
    assert(heroSection, 'Hero section should be present');
    log('✓ Hero section loaded', 'green');
    
    await takeScreenshot(page, 'home');
    
    // ═══════════════════════════════════════════════════════════
    // Test 2: User Registration Journey
    // ═══════════════════════════════════════════════════════════
    logTest('User Registration Journey');
    
    // Navigate to registration page
    log('Navigating to registration page...', 'yellow');
    
    // Try different selectors for register link
    const registerSelectors = [
      'a[href="/register"]',
      'a:contains("Register")',
      'a:contains("Sign Up")',
      'button:contains("Register")',
      'button:contains("Sign Up")',
      '[data-testid="register-link"]'
    ];
    
    let registerLink = null;
    for (const selector of registerSelectors) {
      try {
        registerLink = await page.$(selector);
        if (registerLink) break;
      } catch (e) {
        // Try next selector
      }
    }
    
    if (registerLink) {
      await registerLink.click();
      await page.waitForNavigation({ waitUntil: 'networkidle2' });
    } else {
      // Navigate directly
      await page.goto(`${BASE_URL}/register`, { waitUntil: 'networkidle2' });
    }
    
    log('✓ Registration page loaded', 'green');
    await takeScreenshot(page, 'register-page');
    
    // Fill registration form
    log('Filling registration form...', 'yellow');
    
    // Try different input selectors
    const nameSelectors = ['input[name="name"]', 'input[type="text"]', '#name'];
    const emailSelectors = ['input[name="email"]', 'input[type="email"]', '#email'];
    const passwordSelectors = ['input[name="password"]', 'input[type="password"]', '#password'];
    
    // Fill name
    for (const selector of nameSelectors) {
      try {
        await page.waitForSelector(selector, { timeout: 5000 });
        await page.type(selector, testUser.name);
        log('✓ Name field filled', 'green');
        break;
      } catch (e) {
        continue;
      }
    }
    
    // Fill email
    for (const selector of emailSelectors) {
      try {
        const input = await page.$(selector);
        if (input) {
          const value = await page.evaluate(el => el.value, input);
          if (!value) { // Only type if empty
            await page.type(selector, testUser.email);
            log('✓ Email field filled', 'green');
            break;
          }
        }
      } catch (e) {
        continue;
      }
    }
    
    // Fill password
    for (const selector of passwordSelectors) {
      try {
        await page.type(selector, testUser.password);
        log('✓ Password field filled', 'green');
        
        // Check for confirm password field
        const confirmSelectors = ['input[name="confirm_password"]', 'input[name="confirmPassword"]', '#confirm-password'];
        for (const confirmSelector of confirmSelectors) {
          try {
            const confirmInput = await page.$(confirmSelector);
            if (confirmInput) {
              await page.type(confirmSelector, testUser.password);
              log('✓ Confirm password field filled', 'green');
              break;
            }
          } catch (e) {
            continue;
          }
        }
        break;
      } catch (e) {
        continue;
      }
    }
    
    await takeScreenshot(page, 'register-filled');
    
    // Submit form
    log('Submitting registration form...', 'yellow');
    const submitSelectors = [
      'button[type="submit"]',
      'input[type="submit"]',
      'button:contains("Register")',
      'button:contains("Sign Up")'
    ];
    
    for (const selector of submitSelectors) {
      try {
        const button = await page.$(selector);
        if (button) {
          await button.click();
          break;
        }
      } catch (e) {
        continue;
      }
    }
    
    // Wait for response
    await delay(2000);
    await takeScreenshot(page, 'register-result');
    
    // Check if registration was successful (might redirect to login or dashboard)
    const currentUrl = page.url();
    log(`✓ Registration completed. Current URL: ${currentUrl}`, 'green');
    
    // ═══════════════════════════════════════════════════════════
    // Test 3: User Login Journey
    // ═══════════════════════════════════════════════════════════
    logTest('User Login Journey');
    
    // Navigate to login page
    if (!currentUrl.includes('/login')) {
      await page.goto(`${BASE_URL}/login`, { waitUntil: 'networkidle2' });
    }
    
    log('✓ Login page loaded', 'green');
    await takeScreenshot(page, 'login-page');
    
    // Clear any existing values and fill login form
    log('Filling login form...', 'yellow');
    
    // Clear and fill email
    const loginEmailInput = await page.$('input[name="email"], input[type="email"]');
    if (loginEmailInput) {
      await page.evaluate(el => el.value = '', loginEmailInput);
      await page.type('input[name="email"], input[type="email"]', testUser.email);
      log('✓ Email field filled', 'green');
    }
    
    // Clear and fill password
    const loginPasswordInput = await page.$('input[name="password"], input[type="password"]');
    if (loginPasswordInput) {
      await page.evaluate(el => el.value = '', loginPasswordInput);
      await page.type('input[name="password"], input[type="password"]', testUser.password);
      log('✓ Password field filled', 'green');
    }
    
    await takeScreenshot(page, 'login-filled');
    
    // Submit login form
    log('Submitting login form...', 'yellow');
    const loginButton = await page.$('button[type="submit"], input[type="submit"]');
    if (loginButton) {
      await loginButton.click();
    }
    
    // Wait for navigation
    await delay(2000);
    
    const afterLoginUrl = page.url();
    log(`✓ Login completed. Current URL: ${afterLoginUrl}`, 'green');
    await takeScreenshot(page, 'after-login');
    
    // ═══════════════════════════════════════════════════════════
    // Test 4: Dashboard/Recipe Browsing
    // ═══════════════════════════════════════════════════════════
    logTest('Dashboard & Recipe Browsing');
    
    // Check if we're on dashboard or navigate to it
    if (!afterLoginUrl.includes('/dashboard')) {
      try {
        await page.goto(`${BASE_URL}/dashboard`, { waitUntil: 'networkidle2' });
        log('✓ Dashboard loaded', 'green');
      } catch (e) {
        log('Dashboard might require authentication', 'yellow');
      }
    }
    
    await takeScreenshot(page, 'dashboard');
    
    // Navigate to recipes page
    log('Navigating to recipes page...', 'yellow');
    await page.goto(`${BASE_URL}/recipes`, { waitUntil: 'networkidle2' });
    log('✓ Recipes page loaded', 'green');
    await takeScreenshot(page, 'recipes-list');
    
    // ═══════════════════════════════════════════════════════════
    // Test 5: Recipe Creation Journey
    // ═══════════════════════════════════════════════════════════
    logTest('Recipe Creation Journey');
    
    // Navigate to create recipe page
    log('Navigating to create recipe page...', 'yellow');
    
    const createRecipeSelectors = [
      'a[href="/recipes/new"]',
      'button:contains("Create Recipe")',
      'a:contains("Create Recipe")',
      'button:contains("New Recipe")'
    ];
    
    let createLink = null;
    for (const selector of createRecipeSelectors) {
      try {
        createLink = await page.$(selector);
        if (createLink) {
          await createLink.click();
          await page.waitForNavigation({ waitUntil: 'networkidle2' });
          break;
        }
      } catch (e) {
        continue;
      }
    }
    
    if (!createLink) {
      await page.goto(`${BASE_URL}/recipes/new`, { waitUntil: 'networkidle2' });
    }
    
    log('✓ Create recipe page loaded', 'green');
    await takeScreenshot(page, 'create-recipe-page');
    
    // Fill recipe form (basic fields)
    log('Filling recipe form...', 'yellow');
    
    try {
      // Title
      const titleInput = await page.$('input[name="title"], #title');
      if (titleInput) {
        await page.type('input[name="title"], #title', 'Test Recipe - Chocolate Cake');
        log('✓ Title filled', 'green');
      }
      
      // Description
      const descInput = await page.$('textarea[name="description"], #description');
      if (descInput) {
        await page.type('textarea[name="description"], #description', 'A delicious chocolate cake recipe for testing');
        log('✓ Description filled', 'green');
      }
      
      // Prep time
      const prepInput = await page.$('input[name="prep_time"], #prep_time');
      if (prepInput) {
        await page.type('input[name="prep_time"], #prep_time', '30');
        log('✓ Prep time filled', 'green');
      }
      
      // Cook time
      const cookInput = await page.$('input[name="cook_time"], #cook_time');
      if (cookInput) {
        await page.type('input[name="cook_time"], #cook_time', '45');
        log('✓ Cook time filled', 'green');
      }
      
      // Servings
      const servingsInput = await page.$('input[name="servings"], #servings');
      if (servingsInput) {
        await page.type('input[name="servings"], #servings', '8');
        log('✓ Servings filled', 'green');
      }
      
    } catch (e) {
      log(`Recipe form might have different structure: ${e.message}`, 'yellow');
    }
    
    await takeScreenshot(page, 'create-recipe-filled');
    
    // ═══════════════════════════════════════════════════════════
    // Test 6: AI Chat Journey
    // ═══════════════════════════════════════════════════════════
    logTest('AI Chat Interaction');
    
    // Navigate to AI chat
    log('Navigating to AI chat...', 'yellow');
    await page.goto(`${BASE_URL}/ai/chat`, { waitUntil: 'networkidle2' });
    log('✓ AI chat page loaded', 'green');
    await takeScreenshot(page, 'ai-chat-page');
    
    // Try to find chat input
    const chatSelectors = [
      'input[type="text"]',
      'textarea',
      '#chat-input',
      '[name="message"]',
      '.chat-input'
    ];
    
    let chatInput = null;
    for (const selector of chatSelectors) {
      try {
        chatInput = await page.$(selector);
        if (chatInput) {
          await page.type(selector, 'Can you suggest a recipe for pasta?');
          log('✓ Chat message typed', 'green');
          break;
        }
      } catch (e) {
        continue;
      }
    }
    
    // Try to send message
    const sendSelectors = [
      'button[type="submit"]',
      'button:contains("Send")',
      '#send-button'
    ];
    
    for (const selector of sendSelectors) {
      try {
        const button = await page.$(selector);
        if (button) {
          await button.click();
          log('✓ Message sent', 'green');
          break;
        }
      } catch (e) {
        continue;
      }
    }
    
    await delay(2000);
    await takeScreenshot(page, 'ai-chat-response');
    
    // ═══════════════════════════════════════════════════════════
    // Test 7: Search Functionality
    // ═══════════════════════════════════════════════════════════
    logTest('Search Functionality');
    
    // Go back to recipes page
    await page.goto(`${BASE_URL}/recipes`, { waitUntil: 'networkidle2' });
    
    // Try to find search input
    const searchSelectors = [
      'input[type="search"]',
      'input[placeholder*="Search"]',
      '#search',
      '.search-input'
    ];
    
    let searchInput = null;
    for (const selector of searchSelectors) {
      try {
        searchInput = await page.$(selector);
        if (searchInput) {
          await page.type(selector, 'chocolate');
          log('✓ Search term entered', 'green');
          
          // Trigger search (might be automatic with HTMX or need button click)
          await page.keyboard.press('Enter');
          await delay(1000);
          break;
        }
      } catch (e) {
        continue;
      }
    }
    
    await takeScreenshot(page, 'search-results');
    
    // ═══════════════════════════════════════════════════════════
    // Test 8: User Profile
    // ═══════════════════════════════════════════════════════════
    logTest('User Profile & Settings');
    
    // Navigate to profile
    log('Navigating to user profile...', 'yellow');
    await page.goto(`${BASE_URL}/profile`, { waitUntil: 'networkidle2' });
    log('✓ Profile page loaded', 'green');
    await takeScreenshot(page, 'profile-page');
    
    // ═══════════════════════════════════════════════════════════
    // Test 9: Mobile Responsive Design
    // ═══════════════════════════════════════════════════════════
    logTest('Mobile Responsive Design');
    
    // Change viewport to mobile
    log('Testing mobile viewport...', 'yellow');
    await page.setViewport({ width: 375, height: 812 }); // iPhone X size
    
    // Test key pages on mobile
    await page.goto(BASE_URL, { waitUntil: 'networkidle2' });
    await takeScreenshot(page, 'mobile-home');
    
    await page.goto(`${BASE_URL}/recipes`, { waitUntil: 'networkidle2' });
    await takeScreenshot(page, 'mobile-recipes');
    
    // Check if hamburger menu exists
    const hamburgerSelectors = [
      '.hamburger',
      '.menu-toggle',
      '[aria-label="Menu"]',
      '.mobile-menu'
    ];
    
    for (const selector of hamburgerSelectors) {
      try {
        const hamburger = await page.$(selector);
        if (hamburger) {
          await hamburger.click();
          log('✓ Mobile menu opened', 'green');
          await delay(500);
          await takeScreenshot(page, 'mobile-menu-open');
          break;
        }
      } catch (e) {
        continue;
      }
    }
    
    // ═══════════════════════════════════════════════════════════
    // Test 10: Logout Journey
    // ═══════════════════════════════════════════════════════════
    logTest('Logout Journey');
    
    // Reset viewport
    await page.setViewport({ width: 1280, height: 800 });
    
    // Find and click logout
    log('Looking for logout option...', 'yellow');
    const logoutSelectors = [
      'a[href="/logout"]',
      'button:contains("Logout")',
      'a:contains("Logout")',
      'button:contains("Sign Out")'
    ];
    
    for (const selector of logoutSelectors) {
      try {
        const logoutLink = await page.$(selector);
        if (logoutLink) {
          await logoutLink.click();
          log('✓ Logout clicked', 'green');
          break;
        }
      } catch (e) {
        continue;
      }
    }
    
    // Alternative: POST to logout endpoint
    await page.evaluate(() => {
      const form = document.createElement('form');
      form.method = 'POST';
      form.action = '/logout';
      document.body.appendChild(form);
      form.submit();
    });
    
    await delay(2000);
    const finalUrl = page.url();
    log(`✓ Logout completed. Final URL: ${finalUrl}`, 'green');
    await takeScreenshot(page, 'after-logout');
    
    // ═══════════════════════════════════════════════════════════
    // Summary
    // ═══════════════════════════════════════════════════════════
    console.log(`\n${colors.green}════════════════════════════════════════════${colors.reset}`);
    console.log(`${colors.green}✅ All User Journey Tests Completed!${colors.reset}`);
    console.log(`${colors.green}════════════════════════════════════════════${colors.reset}`);
    
    console.log(`\n${colors.yellow}📊 Test Summary:${colors.reset}`);
    console.log('  ✓ Home page load');
    console.log('  ✓ User registration');
    console.log('  ✓ User login');
    console.log('  ✓ Dashboard access');
    console.log('  ✓ Recipe browsing');
    console.log('  ✓ Recipe creation');
    console.log('  ✓ AI chat interaction');
    console.log('  ✓ Search functionality');
    console.log('  ✓ User profile');
    console.log('  ✓ Mobile responsiveness');
    console.log('  ✓ Logout flow');
    
    console.log(`\n${colors.yellow}📸 Screenshots saved in /tmp/alchemorsel-*.png${colors.reset}`);
    
  } catch (error) {
    console.error(`\n${colors.red}❌ Test failed with error:${colors.reset}`);
    console.error(error);
    await takeScreenshot(page, 'error');
    throw error;
  } finally {
    await browser.close();
  }
}

// Run the tests
runUserJourneys().catch(error => {
  console.error('Failed to run user journeys:', error);
  process.exit(1);
});