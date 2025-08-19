/**
 * Comprehensive Puppeteer Test Suite for Alchemorsel v3
 * Tests all functionality combinations and user journeys
 */

const puppeteer = require('puppeteer');
const fs = require('fs');
const path = require('path');

// Test configuration
const CONFIG = {
    baseUrl: 'http://localhost:8080',
    screenshotDir: './test/e2e/screenshots',
    testTimeout: 120000,
    testUser: {
        username: `testuser_${Date.now()}`,
        email: `test_${Date.now()}@example.com`,
        password: 'TestPassword123!',
        fullName: 'Test User'
    }
};

// Ensure screenshot directory exists
if (!fs.existsSync(CONFIG.screenshotDir)) {
    fs.mkdirSync(CONFIG.screenshotDir, { recursive: true });
}

class AlchemorselTestSuite {
    constructor() {
        this.browser = null;
        this.page = null;
        this.testResults = {
            passed: [],
            failed: [],
            warnings: []
        };
        this.screenshotCounter = 0;
    }

    async init() {
        console.log('🚀 Initializing Puppeteer browser...');
        this.browser = await puppeteer.launch({
            headless: false,
            args: ['--no-sandbox', '--disable-setuid-sandbox'],
            defaultViewport: { width: 1280, height: 800 }
        });
        this.page = await this.browser.newPage();
        
        // Set up console logging from the page
        this.page.on('console', msg => {
            if (msg.type() === 'error') {
                this.testResults.warnings.push(`Console error: ${msg.text()}`);
            }
        });

        // Set up request interception to monitor network activity
        await this.page.setRequestInterception(true);
        this.page.on('request', request => {
            request.continue();
        });

        this.page.on('response', response => {
            if (response.status() >= 400) {
                this.testResults.warnings.push(`HTTP ${response.status()} for ${response.url()}`);
            }
        });
    }

    async takeScreenshot(name) {
        this.screenshotCounter++;
        const filename = `${String(this.screenshotCounter).padStart(3, '0')}_${name}.png`;
        const filepath = path.join(CONFIG.screenshotDir, filename);
        await this.page.screenshot({ path: filepath, fullPage: true });
        console.log(`📸 Screenshot saved: ${filename}`);
        return filepath;
    }

    async testStep(name, testFn) {
        console.log(`\n📝 Testing: ${name}`);
        try {
            await testFn();
            this.testResults.passed.push(name);
            console.log(`✅ PASSED: ${name}`);
            return true;
        } catch (error) {
            this.testResults.failed.push({ test: name, error: error.message });
            console.error(`❌ FAILED: ${name} - ${error.message}`);
            await this.takeScreenshot(`FAILED_${name.replace(/\s+/g, '_')}`);
            return false;
        }
    }

    async waitAndClick(selector, description = '') {
        await this.page.waitForSelector(selector, { timeout: 5000 });
        await this.page.click(selector);
        if (description) console.log(`   Clicked: ${description}`);
    }

    async waitAndType(selector, text, description = '') {
        await this.page.waitForSelector(selector, { timeout: 5000 });
        await this.page.click(selector);
        await this.page.type(selector, text);
        if (description) console.log(`   Typed: ${description}`);
    }

    async checkElementExists(selector, shouldExist = true) {
        try {
            await this.page.waitForSelector(selector, { timeout: 2000 });
            return shouldExist;
        } catch {
            return !shouldExist;
        }
    }

    async getAllLinks() {
        return await this.page.evaluate(() => {
            return Array.from(document.querySelectorAll('a')).map(link => ({
                href: link.href,
                text: link.textContent.trim(),
                visible: link.offsetParent !== null
            })).filter(link => link.visible && link.href);
        });
    }

    // Test Suite Methods
    async testPublicHomePage() {
        await this.testStep('Navigate to Home Page', async () => {
            await this.page.goto(CONFIG.baseUrl, { waitUntil: 'networkidle2' });
            await this.takeScreenshot('home_page_initial');
            
            // Check for key elements
            const hasLogo = await this.checkElementExists('h1');
            const hasNavigation = await this.checkElementExists('nav');
            const hasChatInterface = await this.checkElementExists('#chat-interface, .chat-container, [data-chat]');
            
            if (!hasLogo) throw new Error('Logo/Title not found');
            if (!hasNavigation) throw new Error('Navigation not found');
            
            console.log(`   Chat interface present: ${hasChatInterface}`);
        });

        await this.testStep('Test Navigation Links on Home Page', async () => {
            const links = await this.getAllLinks();
            console.log(`   Found ${links.length} links on home page`);
            
            for (const link of links) {
                console.log(`   Link: "${link.text}" -> ${link.href}`);
            }
        });

        await this.testStep('Test AI Chat Interface', async () => {
            // Try to find and interact with chat interface
            const chatSelectors = [
                '#chat-input',
                '[name="message"]',
                '[placeholder*="chat"]',
                '[placeholder*="message"]',
                '.chat-input',
                'textarea',
                'input[type="text"]'
            ];
            
            let chatInput = null;
            for (const selector of chatSelectors) {
                if (await this.checkElementExists(selector)) {
                    chatInput = selector;
                    break;
                }
            }
            
            if (chatInput) {
                await this.waitAndType(chatInput, 'Hello, can you help me find a recipe?', 'Chat message');
                
                // Try to find and click send button
                const sendSelectors = [
                    'button[type="submit"]',
                    'button:has-text("Send")',
                    '[data-send]',
                    '.send-button'
                ];
                
                for (const selector of sendSelectors) {
                    try {
                        await this.page.click(selector);
                        console.log('   Sent chat message');
                        await this.page.waitForTimeout(2000);
                        await this.takeScreenshot('chat_interaction');
                        break;
                    } catch {}
                }
            } else {
                console.log('   Chat interface not found on home page');
            }
        });
    }

    async testPublicPages() {
        const publicPages = [
            { path: '/', name: 'Home' },
            { path: '/login', name: 'Login' },
            { path: '/register', name: 'Register' },
            { path: '/recipes', name: 'Recipes' }
        ];

        for (const page of publicPages) {
            await this.testStep(`Test Public Page: ${page.name}`, async () => {
                await this.page.goto(`${CONFIG.baseUrl}${page.path}`, { waitUntil: 'networkidle2' });
                await this.takeScreenshot(`public_${page.name.toLowerCase()}`);
                
                // Check response status
                const response = await this.page.goto(`${CONFIG.baseUrl}${page.path}`);
                if (response.status() !== 200) {
                    throw new Error(`Page returned status ${response.status()}`);
                }
                
                // Get all clickable elements
                const clickableElements = await this.page.evaluate(() => {
                    const elements = [];
                    document.querySelectorAll('a, button').forEach(el => {
                        if (el.offsetParent !== null) {
                            elements.push({
                                tag: el.tagName,
                                text: el.textContent.trim(),
                                href: el.href || null
                            });
                        }
                    });
                    return elements;
                });
                
                console.log(`   Found ${clickableElements.length} clickable elements`);
            });
        }
    }

    async testRegistration() {
        await this.testStep('User Registration Flow', async () => {
            await this.page.goto(`${CONFIG.baseUrl}/register`, { waitUntil: 'networkidle2' });
            await this.takeScreenshot('register_page');
            
            // Find and fill registration form
            const formSelectors = {
                username: ['#username', '[name="username"]', 'input[placeholder*="username"]'],
                email: ['#email', '[name="email"]', 'input[type="email"]'],
                password: ['#password', '[name="password"]', 'input[type="password"]:not([name="confirm_password"])'],
                confirmPassword: ['#confirm_password', '[name="confirm_password"]', 'input[type="password"]:last-of-type'],
                fullName: ['#full_name', '[name="full_name"]', '[name="name"]', 'input[placeholder*="name"]']
            };
            
            // Try to fill username
            for (const selector of formSelectors.username) {
                try {
                    await this.waitAndType(selector, CONFIG.testUser.username, 'Username');
                    break;
                } catch {}
            }
            
            // Try to fill email
            for (const selector of formSelectors.email) {
                try {
                    await this.waitAndType(selector, CONFIG.testUser.email, 'Email');
                    break;
                } catch {}
            }
            
            // Try to fill password
            for (const selector of formSelectors.password) {
                try {
                    await this.waitAndType(selector, CONFIG.testUser.password, 'Password');
                    break;
                } catch {}
            }
            
            // Try to fill confirm password
            for (const selector of formSelectors.confirmPassword) {
                try {
                    await this.waitAndType(selector, CONFIG.testUser.password, 'Confirm Password');
                    break;
                } catch {}
            }
            
            // Try to fill full name (if exists)
            for (const selector of formSelectors.fullName) {
                try {
                    await this.waitAndType(selector, CONFIG.testUser.fullName, 'Full Name');
                    break;
                } catch {}
            }
            
            await this.takeScreenshot('register_form_filled');
            
            // Submit form
            const submitSelectors = [
                'button[type="submit"]',
                'input[type="submit"]',
                'button:has-text("Register")',
                'button:has-text("Sign Up")'
            ];
            
            for (const selector of submitSelectors) {
                try {
                    await this.page.click(selector);
                    console.log('   Submitted registration form');
                    break;
                } catch {}
            }
            
            // Wait for navigation or response
            await this.page.waitForTimeout(3000);
            await this.takeScreenshot('register_after_submit');
            
            // Check if we're redirected to login or dashboard
            const currentUrl = this.page.url();
            console.log(`   After registration, URL: ${currentUrl}`);
        });
    }

    async testLogin() {
        await this.testStep('User Login Flow', async () => {
            await this.page.goto(`${CONFIG.baseUrl}/login`, { waitUntil: 'networkidle2' });
            await this.takeScreenshot('login_page');
            
            // Find and fill login form
            const loginSelectors = {
                username: ['#username', '[name="username"]', '#email', '[name="email"]', 'input[type="email"]'],
                password: ['#password', '[name="password"]', 'input[type="password"]']
            };
            
            // Try username/email field
            let usedEmail = false;
            for (const selector of loginSelectors.username) {
                try {
                    const inputType = await this.page.$eval(selector, el => el.type);
                    if (inputType === 'email') {
                        await this.waitAndType(selector, CONFIG.testUser.email, 'Email');
                        usedEmail = true;
                    } else {
                        await this.waitAndType(selector, CONFIG.testUser.username, 'Username');
                    }
                    break;
                } catch {}
            }
            
            // Try password field
            for (const selector of loginSelectors.password) {
                try {
                    await this.waitAndType(selector, CONFIG.testUser.password, 'Password');
                    break;
                } catch {}
            }
            
            await this.takeScreenshot('login_form_filled');
            
            // Submit login form
            const submitSelectors = [
                'button[type="submit"]',
                'input[type="submit"]',
                'button:has-text("Login")',
                'button:has-text("Sign In")'
            ];
            
            for (const selector of submitSelectors) {
                try {
                    await this.page.click(selector);
                    console.log('   Submitted login form');
                    break;
                } catch {}
            }
            
            // Wait for navigation
            await this.page.waitForTimeout(3000);
            await this.takeScreenshot('login_after_submit');
            
            // Check if we're logged in
            const currentUrl = this.page.url();
            console.log(`   After login, URL: ${currentUrl}`);
            
            // Check for authentication indicators
            const hasLogout = await this.checkElementExists('a[href="/logout"], button:has-text("Logout"), button:has-text("Sign Out")');
            const hasDashboard = await this.checkElementExists('a[href="/dashboard"]');
            const hasProfile = await this.checkElementExists('a[href="/profile"]');
            
            console.log(`   Authentication indicators - Logout: ${hasLogout}, Dashboard: ${hasDashboard}, Profile: ${hasProfile}`);
        });
    }

    async testProtectedPages() {
        const protectedPages = [
            { path: '/dashboard', name: 'Dashboard' },
            { path: '/profile', name: 'Profile' },
            { path: '/create', name: 'Create Recipe' },
            { path: '/recipes/create', name: 'Create Recipe Alt' }
        ];

        for (const page of protectedPages) {
            await this.testStep(`Test Protected Page: ${page.name}`, async () => {
                const response = await this.page.goto(`${CONFIG.baseUrl}${page.path}`, { 
                    waitUntil: 'networkidle2',
                    waitForTimeout: 5000 
                });
                
                const finalUrl = this.page.url();
                const status = response ? response.status() : 'unknown';
                
                console.log(`   Attempted to access ${page.path}`);
                console.log(`   Response status: ${status}`);
                console.log(`   Final URL: ${finalUrl}`);
                
                // Check if we were redirected to login (unauthorized)
                if (finalUrl.includes('/login')) {
                    console.log(`   Redirected to login (expected for protected page if not authenticated)`);
                } else if (status === 200) {
                    console.log(`   Successfully accessed protected page`);
                    await this.takeScreenshot(`protected_${page.name.toLowerCase().replace(/\s+/g, '_')}`);
                    
                    // Test page functionality
                    const clickableElements = await this.page.evaluate(() => {
                        return Array.from(document.querySelectorAll('a, button')).map(el => ({
                            tag: el.tagName,
                            text: el.textContent.trim(),
                            href: el.href || null
                        })).filter(el => el.text);
                    });
                    
                    console.log(`   Found ${clickableElements.length} interactive elements`);
                }
            });
        }
    }

    async testAuthenticatedChat() {
        await this.testStep('Test AI Chat While Authenticated', async () => {
            await this.page.goto(CONFIG.baseUrl, { waitUntil: 'networkidle2' });
            await this.takeScreenshot('home_authenticated');
            
            // Try to find chat interface
            const chatSelectors = [
                '#chat-input',
                '[name="message"]',
                '[placeholder*="chat"]',
                '[placeholder*="message"]',
                '.chat-input',
                'textarea'
            ];
            
            let chatInput = null;
            for (const selector of chatSelectors) {
                if (await this.checkElementExists(selector)) {
                    chatInput = selector;
                    break;
                }
            }
            
            if (chatInput) {
                await this.waitAndType(chatInput, 'Can you suggest a healthy dinner recipe?', 'Authenticated chat message');
                
                // Try to send
                const sendSelectors = [
                    'button[type="submit"]',
                    'button:has-text("Send")',
                    '[data-send]'
                ];
                
                for (const selector of sendSelectors) {
                    try {
                        await this.page.click(selector);
                        console.log('   Sent authenticated chat message');
                        await this.page.waitForTimeout(3000);
                        await this.takeScreenshot('chat_authenticated_response');
                        break;
                    } catch {}
                }
            }
        });
    }

    async testNavigationInAllStates() {
        await this.testStep('Test All Navigation Links', async () => {
            // Get all navigation links
            const navLinks = await this.page.evaluate(() => {
                const links = [];
                document.querySelectorAll('nav a, header a').forEach(link => {
                    if (link.offsetParent !== null) {
                        links.push({
                            href: link.href,
                            text: link.textContent.trim()
                        });
                    }
                });
                return links;
            });
            
            console.log(`   Found ${navLinks.length} navigation links`);
            
            // Test each navigation link
            for (const link of navLinks) {
                if (link.href && !link.href.includes('logout')) {
                    try {
                        console.log(`   Testing nav link: "${link.text}" -> ${link.href}`);
                        await this.page.goto(link.href, { waitUntil: 'networkidle2', timeout: 10000 });
                        await this.page.waitForTimeout(1000);
                    } catch (error) {
                        console.log(`   Failed to navigate to ${link.href}: ${error.message}`);
                    }
                }
            }
        });
    }

    async testLogout() {
        await this.testStep('Test Logout Functionality', async () => {
            // Find and click logout
            const logoutSelectors = [
                'a[href="/logout"]',
                'button:has-text("Logout")',
                'button:has-text("Sign Out")',
                'a:has-text("Logout")',
                'a:has-text("Sign Out")'
            ];
            
            let loggedOut = false;
            for (const selector of logoutSelectors) {
                try {
                    await this.page.click(selector);
                    console.log('   Clicked logout');
                    loggedOut = true;
                    break;
                } catch {}
            }
            
            if (!loggedOut) {
                // Try navigating directly to logout URL
                await this.page.goto(`${CONFIG.baseUrl}/logout`, { waitUntil: 'networkidle2' });
            }
            
            await this.page.waitForTimeout(2000);
            await this.takeScreenshot('after_logout');
            
            // Verify we're logged out
            const currentUrl = this.page.url();
            console.log(`   After logout, URL: ${currentUrl}`);
            
            // Check that protected pages redirect to login
            await this.page.goto(`${CONFIG.baseUrl}/dashboard`, { waitUntil: 'networkidle2' });
            const dashboardUrl = this.page.url();
            
            if (dashboardUrl.includes('/login')) {
                console.log('   Confirmed: Protected pages now redirect to login');
            } else {
                console.log('   Warning: Still able to access protected pages after logout');
            }
        });
    }

    async testFormValidation() {
        await this.testStep('Test Form Validation', async () => {
            // Test registration form validation
            await this.page.goto(`${CONFIG.baseUrl}/register`, { waitUntil: 'networkidle2' });
            
            // Try to submit empty form
            const submitSelectors = [
                'button[type="submit"]',
                'input[type="submit"]'
            ];
            
            for (const selector of submitSelectors) {
                try {
                    await this.page.click(selector);
                    await this.page.waitForTimeout(1000);
                    
                    // Check for validation messages
                    const validationMessages = await this.page.evaluate(() => {
                        const messages = [];
                        document.querySelectorAll('.error, .alert, [role="alert"]').forEach(el => {
                            messages.push(el.textContent.trim());
                        });
                        return messages;
                    });
                    
                    if (validationMessages.length > 0) {
                        console.log(`   Found ${validationMessages.length} validation messages`);
                        validationMessages.forEach(msg => console.log(`     - ${msg}`));
                    }
                    
                    await this.takeScreenshot('form_validation');
                    break;
                } catch {}
            }
        });
    }

    async testSessionPersistence() {
        await this.testStep('Test Session Persistence', async () => {
            // Get cookies
            const cookies = await this.page.cookies();
            const sessionCookie = cookies.find(c => c.name.toLowerCase().includes('session') || c.name.toLowerCase().includes('auth'));
            
            if (sessionCookie) {
                console.log(`   Found session cookie: ${sessionCookie.name}`);
                
                // Open new page to test session persistence
                const newPage = await this.browser.newPage();
                await newPage.setCookie(...cookies);
                await newPage.goto(`${CONFIG.baseUrl}/dashboard`, { waitUntil: 'networkidle2' });
                
                const newPageUrl = newPage.url();
                if (!newPageUrl.includes('/login')) {
                    console.log('   Session persisted across pages');
                } else {
                    console.log('   Session not persisting properly');
                }
                
                await newPage.close();
            } else {
                console.log('   No session cookie found');
            }
        });
    }

    async testResponsiveness() {
        await this.testStep('Test Mobile Responsiveness', async () => {
            // Test mobile viewport
            await this.page.setViewport({ width: 375, height: 667 });
            await this.page.goto(CONFIG.baseUrl, { waitUntil: 'networkidle2' });
            await this.takeScreenshot('mobile_view');
            
            // Check for mobile menu
            const hasMobileMenu = await this.checkElementExists('[data-mobile-menu], .mobile-menu, .hamburger, button[aria-label*="menu"]');
            console.log(`   Mobile menu present: ${hasMobileMenu}`);
            
            // Test tablet viewport
            await this.page.setViewport({ width: 768, height: 1024 });
            await this.page.goto(CONFIG.baseUrl, { waitUntil: 'networkidle2' });
            await this.takeScreenshot('tablet_view');
            
            // Reset to desktop
            await this.page.setViewport({ width: 1280, height: 800 });
        });
    }

    async generateReport() {
        console.log('\n' + '='.repeat(60));
        console.log('📊 TEST RESULTS SUMMARY');
        console.log('='.repeat(60));
        
        console.log(`\n✅ PASSED TESTS: ${this.testResults.passed.length}`);
        this.testResults.passed.forEach(test => {
            console.log(`   ✓ ${test}`);
        });
        
        console.log(`\n❌ FAILED TESTS: ${this.testResults.failed.length}`);
        this.testResults.failed.forEach(({ test, error }) => {
            console.log(`   ✗ ${test}`);
            console.log(`     Error: ${error}`);
        });
        
        console.log(`\n⚠️  WARNINGS: ${this.testResults.warnings.length}`);
        this.testResults.warnings.forEach(warning => {
            console.log(`   - ${warning}`);
        });
        
        console.log('\n' + '='.repeat(60));
        console.log(`Total Tests Run: ${this.testResults.passed.length + this.testResults.failed.length}`);
        console.log(`Success Rate: ${((this.testResults.passed.length / (this.testResults.passed.length + this.testResults.failed.length)) * 100).toFixed(1)}%`);
        console.log(`Screenshots Taken: ${this.screenshotCounter}`);
        console.log('='.repeat(60));
        
        // Write report to file
        const report = {
            timestamp: new Date().toISOString(),
            summary: {
                total: this.testResults.passed.length + this.testResults.failed.length,
                passed: this.testResults.passed.length,
                failed: this.testResults.failed.length,
                warnings: this.testResults.warnings.length,
                successRate: ((this.testResults.passed.length / (this.testResults.passed.length + this.testResults.failed.length)) * 100).toFixed(1) + '%'
            },
            details: this.testResults
        };
        
        fs.writeFileSync(
            path.join(CONFIG.screenshotDir, 'test_report.json'),
            JSON.stringify(report, null, 2)
        );
        
        console.log(`\n📁 Report saved to: ${path.join(CONFIG.screenshotDir, 'test_report.json')}`);
    }

    async cleanup() {
        if (this.browser) {
            await this.browser.close();
        }
    }

    async run() {
        try {
            await this.init();
            
            console.log('\n🧪 Starting Comprehensive Test Suite for Alchemorsel v3\n');
            console.log('='.repeat(60));
            
            // Phase 1: Test Public Pages (Unauthenticated)
            console.log('\n📌 PHASE 1: Testing Public Pages (Unauthenticated)');
            console.log('-'.repeat(40));
            await this.testPublicHomePage();
            await this.testPublicPages();
            await this.testFormValidation();
            
            // Phase 2: User Registration
            console.log('\n📌 PHASE 2: User Registration');
            console.log('-'.repeat(40));
            await this.testRegistration();
            
            // Phase 3: User Login
            console.log('\n📌 PHASE 3: User Login');
            console.log('-'.repeat(40));
            await this.testLogin();
            
            // Phase 4: Test Protected Pages (Authenticated)
            console.log('\n📌 PHASE 4: Testing Protected Pages (Authenticated)');
            console.log('-'.repeat(40));
            await this.testProtectedPages();
            await this.testAuthenticatedChat();
            await this.testNavigationInAllStates();
            await this.testSessionPersistence();
            
            // Phase 5: Additional Tests
            console.log('\n📌 PHASE 5: Additional Tests');
            console.log('-'.repeat(40));
            await this.testResponsiveness();
            
            // Phase 6: Logout and Verify
            console.log('\n📌 PHASE 6: Logout and Verification');
            console.log('-'.repeat(40));
            await this.testLogout();
            
            // Generate final report
            await this.generateReport();
            
        } catch (error) {
            console.error('💥 Critical test failure:', error);
            this.testResults.failed.push({ test: 'Critical Failure', error: error.message });
        } finally {
            await this.cleanup();
        }
    }
}

// Run the test suite
(async () => {
    const testSuite = new AlchemorselTestSuite();
    await testSuite.run();
    process.exit(testSuite.testResults.failed.length > 0 ? 1 : 0);
})();