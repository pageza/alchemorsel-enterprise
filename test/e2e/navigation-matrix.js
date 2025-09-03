/**
 * Comprehensive Navigation Matrix Tests for Alchemorsel v3
 * 
 * Tests every conceivable combination of navigation paths and authentication states
 * to ensure all links work correctly and no user can access unauthorized content.
 */

const { BaseTest } = require('./framework/base-test');
const { 
  HomePage, 
  LoginPage, 
  RegisterPage, 
  DashboardPage, 
  AIChatPage, 
  RecipesPage, 
  RecipeDetailPage, 
  RecipeFormPage, 
  ProfilePage, 
  NavigationComponent 
} = require('./framework/page-objects');

/**
 * Navigation Matrix Test Suite
 */
class NavigationMatrixTest extends BaseTest {
  constructor() {
    super('NavigationMatrix');
    this.testUsers = {
      validUser: {
        name: 'Test User',
        email: 'testuser@example.com',
        password: 'testpassword123'
      },
      adminUser: {
        name: 'Admin User',
        email: 'admin@example.com',
        password: 'adminpassword123'
      }
    };
  }

  /**
   * Test all navigation paths from unauthenticated state
   */
  async testUnauthenticatedNavigation() {
    this.logger.step('Testing all navigation paths for unauthenticated users');

    const pages = [
      { name: 'Home', page: new HomePage(this), expectRedirect: false },
      { name: 'Recipes', page: new RecipesPage(this), expectRedirect: false },
      { name: 'Login', page: new LoginPage(this), expectRedirect: false },
      { name: 'Register', page: new RegisterPage(this), expectRedirect: false },
      { name: 'Dashboard', page: new DashboardPage(this), expectRedirect: true, redirectTo: '/login' },
      { name: 'AI Chat', page: new AIChatPage(this), expectRedirect: true, redirectTo: '/login' },
      { name: 'Profile', page: new ProfilePage(this), expectRedirect: true, redirectTo: '/login' },
      { name: 'New Recipe', page: new RecipeFormPage(this), expectRedirect: true, redirectTo: '/login' }
    ];

    for (const { name, page, expectRedirect, redirectTo } of pages) {
      await this.testNavigationPath(name, page, expectRedirect, redirectTo, false);
    }
  }

  /**
   * Test all navigation paths from authenticated state
   */
  async testAuthenticatedNavigation() {
    this.logger.step('Testing all navigation paths for authenticated users');

    // First login
    await this.loginTestUser();

    const pages = [
      { name: 'Home', page: new HomePage(this), expectRedirect: false },
      { name: 'Recipes', page: new RecipesPage(this), expectRedirect: false },
      { name: 'Dashboard', page: new DashboardPage(this), expectRedirect: false },
      { name: 'AI Chat', page: new AIChatPage(this), expectRedirect: false },
      { name: 'Profile', page: new ProfilePage(this), expectRedirect: false },
      { name: 'New Recipe', page: new RecipeFormPage(this), expectRedirect: false },
      { name: 'Login', page: new LoginPage(this), expectRedirect: true, redirectTo: '/dashboard' },
      { name: 'Register', page: new RegisterPage(this), expectRedirect: true, redirectTo: '/dashboard' }
    ];

    for (const { name, page, expectRedirect, redirectTo } of pages) {
      await this.testNavigationPath(name, page, expectRedirect, redirectTo, true);
    }
  }

  /**
   * Test navigation through navbar links
   */
  async testNavbarNavigation() {
    this.logger.step('Testing navbar navigation links');

    const nav = new NavigationComponent(this);
    const homePage = new HomePage(this);

    // Start at home page
    await homePage.navigate();
    await homePage.waitForLoad();

    const navigationTests = [
      { method: 'navigateToRecipes', expectedPath: '/recipes' },
      { method: 'navigateToHome', expectedPath: '/' }
    ];

    for (const { method, expectedPath } of navigationTests) {
      this.logger.debug(`Testing navigation method: ${method}`);
      
      await nav[method]();
      await this.delay(1000); // Wait for navigation
      
      const currentUrl = this.page.url();
      if (!currentUrl.includes(expectedPath)) {
        throw new Error(`Navigation failed: expected ${expectedPath}, got ${currentUrl}`);
      }
      
      await this.screenshots.capture(this.page, `navbar-nav-${method}`);
    }
  }

  /**
   * Test authenticated navbar navigation
   */
  async testAuthenticatedNavbarNavigation() {
    this.logger.step('Testing navbar navigation for authenticated users');

    await this.loginTestUser();
    const nav = new NavigationComponent(this);

    const authNavigationTests = [
      { method: 'navigateToDashboard', expectedPath: '/dashboard' },
      { method: 'navigateToAIChat', expectedPath: '/ai/chat' },
      { method: 'navigateToProfile', expectedPath: '/profile' },
      { method: 'navigateToRecipes', expectedPath: '/recipes' },
      { method: 'navigateToHome', expectedPath: '/' }
    ];

    for (const { method, expectedPath } of authNavigationTests) {
      this.logger.debug(`Testing authenticated navigation method: ${method}`);
      
      await nav[method]();
      await this.delay(1000); // Wait for navigation
      
      const currentUrl = this.page.url();
      if (!currentUrl.includes(expectedPath)) {
        throw new Error(`Navigation failed: expected ${expectedPath}, got ${currentUrl}`);
      }
      
      await this.screenshots.capture(this.page, `auth-navbar-nav-${method}`);
    }
  }

  /**
   * Test deep linking and direct URL access
   */
  async testDeepLinking() {
    this.logger.step('Testing deep linking and direct URL access');

    const deepLinkTests = [
      { url: '/recipes/123', name: 'Recipe Detail', authRequired: false },
      { url: '/recipes/123/edit', name: 'Recipe Edit', authRequired: true },
      { url: '/dashboard', name: 'Dashboard', authRequired: true },
      { url: '/profile', name: 'Profile', authRequired: true },
      { url: '/ai/chat', name: 'AI Chat', authRequired: true },
      { url: '/recipes/new', name: 'New Recipe', authRequired: true }
    ];

    // Test unauthenticated deep links
    for (const { url, name, authRequired } of deepLinkTests) {
      this.logger.debug(`Testing unauthenticated deep link: ${name} (${url})`);
      
      await this.page.goto(`${this.options.BASE_URL}${url}`);
      await this.delay(1000);
      
      const currentUrl = this.page.url();
      
      if (authRequired && !currentUrl.includes('/login')) {
        throw new Error(`Deep link should redirect to login: ${name} (${url}), got ${currentUrl}`);
      } else if (!authRequired && currentUrl.includes('/login')) {
        throw new Error(`Deep link should not redirect to login: ${name} (${url}), got ${currentUrl}`);
      }
      
      await this.screenshots.capture(this.page, `deeplink-unauth-${name.replace(/ /g, '-')}`);
    }

    // Test authenticated deep links
    await this.loginTestUser();
    
    for (const { url, name } of deepLinkTests) {
      this.logger.debug(`Testing authenticated deep link: ${name} (${url})`);
      
      await this.page.goto(`${this.options.BASE_URL}${url}`);
      await this.delay(1000);
      
      const currentUrl = this.page.url();
      
      if (currentUrl.includes('/login')) {
        throw new Error(`Authenticated deep link should not redirect to login: ${name} (${url}), got ${currentUrl}`);
      }
      
      await this.screenshots.capture(this.page, `deeplink-auth-${name.replace(/ /g, '-')}`);
    }
  }

  /**
   * Test breadcrumb navigation
   */
  async testBreadcrumbNavigation() {
    this.logger.step('Testing breadcrumb navigation');

    await this.loginTestUser();
    
    // Navigate to recipe detail to create breadcrumb trail
    const recipesPage = new RecipesPage(this);
    await recipesPage.navigate();
    await recipesPage.waitForLoad();
    
    // Simulate clicking a recipe if available
    const recipeCount = await recipesPage.getRecipeCount();
    if (recipeCount > 0) {
      await recipesPage.clickRecipe(0);
      await this.delay(1000);
      
      // Check if breadcrumbs exist
      const breadcrumbs = await this.page.$$('.breadcrumb, [data-testid="breadcrumbs"]');
      if (breadcrumbs.length > 0) {
        this.logger.success('Breadcrumbs found - testing navigation');
        
        // Test clicking breadcrumb links
        const breadcrumbLinks = await this.page.$$('.breadcrumb a, [data-testid="breadcrumbs"] a');
        for (let i = 0; i < Math.min(breadcrumbLinks.length, 3); i++) {
          const link = breadcrumbLinks[i];
          const href = await link.getAttribute('href');
          const text = await link.textContent();
          
          this.logger.debug(`Testing breadcrumb link: "${text}" -> ${href}`);
          
          await link.click();
          await this.delay(1000);
          
          const currentUrl = this.page.url();
          this.logger.debug(`Breadcrumb navigation result: ${currentUrl}`);
          
          await this.screenshots.capture(this.page, `breadcrumb-nav-${i}`);
          
          // Navigate back to continue testing
          await this.page.goBack();
          await this.delay(1000);
        }
      } else {
        this.logger.warning('No breadcrumbs found on recipe detail page');
      }
    }
  }

  /**
   * Test cross-page navigation combinations
   */
  async testCrossPageNavigation() {
    this.logger.step('Testing complex cross-page navigation combinations');

    await this.loginTestUser();

    const navigationScenarios = [
      {
        name: 'Recipe Discovery Flow',
        steps: [
          { page: 'Home', action: 'navigate' },
          { page: 'Recipes', action: 'searchRecipes', params: ['pasta'] },
          { page: 'RecipeDetail', action: 'clickRecipe', params: [0] },
          { page: 'Dashboard', action: 'navigateToDashboard' }
        ]
      },
      {
        name: 'Recipe Creation Flow',
        steps: [
          { page: 'Dashboard', action: 'navigate' },
          { page: 'RecipeForm', action: 'clickQuickAction', params: ['Create'] },
          { page: 'Recipes', action: 'navigateToRecipes' },
          { page: 'Home', action: 'navigateToHome' }
        ]
      },
      {
        name: 'AI Chat to Recipe Flow',
        steps: [
          { page: 'AIChatPage', action: 'navigate' },
          { page: 'Recipes', action: 'navigateToRecipes' },
          { page: 'Profile', action: 'navigateToProfile' },
          { page: 'Dashboard', action: 'navigateToDashboard' }
        ]
      }
    ];

    const pageInstances = {
      Home: new HomePage(this),
      Dashboard: new DashboardPage(this),
      Recipes: new RecipesPage(this),
      RecipeDetail: new RecipeDetailPage(this),
      RecipeForm: new RecipeFormPage(this),
      AIChatPage: new AIChatPage(this),
      Profile: new ProfilePage(this)
    };

    const nav = new NavigationComponent(this);

    for (const scenario of navigationScenarios) {
      this.logger.debug(`Testing navigation scenario: ${scenario.name}`);
      
      try {
        for (let i = 0; i < scenario.steps.length; i++) {
          const step = scenario.steps[i];
          this.logger.debug(`Step ${i + 1}: ${step.action} on ${step.page}`);
          
          if (step.action === 'navigate') {
            const pageInstance = pageInstances[step.page];
            await pageInstance.navigate();
            await pageInstance.waitForLoad();
          } else if (step.action.startsWith('navigateTo')) {
            const navMethod = step.action;
            if (typeof nav[navMethod] === 'function') {
              await nav[navMethod]();
            }
          } else if (pageInstances[step.page] && typeof pageInstances[step.page][step.action] === 'function') {
            const pageInstance = pageInstances[step.page];
            if (step.params) {
              await pageInstance[step.action](...step.params);
            } else {
              await pageInstance[step.action]();
            }
          }
          
          await this.delay(1000);
          await this.screenshots.capture(this.page, `scenario-${scenario.name.replace(/ /g, '-')}-step-${i + 1}`);
        }
        
        this.logger.success(`Navigation scenario completed: ${scenario.name}`);
      } catch (error) {
        this.logger.error(`Navigation scenario failed: ${scenario.name} - ${error.message}`);
        throw error;
      }
    }
  }

  /**
   * Test back/forward browser navigation
   */
  async testBrowserNavigation() {
    this.logger.step('Testing browser back/forward navigation');

    const homePage = new HomePage(this);
    const recipesPage = new RecipesPage(this);
    const loginPage = new LoginPage(this);

    // Build navigation history
    await homePage.navigate();
    await homePage.waitForLoad();
    await this.delay(500);

    await recipesPage.navigate();
    await recipesPage.waitForLoad();
    await this.delay(500);

    await loginPage.navigate();
    await loginPage.waitForLoad();
    await this.delay(500);

    // Test back navigation
    await this.page.goBack(); // Should be back to recipes
    await this.delay(1000);
    
    let currentUrl = this.page.url();
    if (!currentUrl.includes('/recipes')) {
      throw new Error(`Back navigation failed: expected /recipes, got ${currentUrl}`);
    }
    await this.screenshots.capture(this.page, 'browser-nav-back-1');

    await this.page.goBack(); // Should be back to home
    await this.delay(1000);
    
    currentUrl = this.page.url();
    if (!currentUrl.includes('/') || currentUrl.includes('/recipes') || currentUrl.includes('/login')) {
      throw new Error(`Back navigation failed: expected home, got ${currentUrl}`);
    }
    await this.screenshots.capture(this.page, 'browser-nav-back-2');

    // Test forward navigation
    await this.page.goForward(); // Should be forward to recipes
    await this.delay(1000);
    
    currentUrl = this.page.url();
    if (!currentUrl.includes('/recipes')) {
      throw new Error(`Forward navigation failed: expected /recipes, got ${currentUrl}`);
    }
    await this.screenshots.capture(this.page, 'browser-nav-forward-1');

    this.logger.success('Browser navigation tests completed successfully');
  }

  /**
   * Test logout navigation and state cleanup
   */
  async testLogoutNavigation() {
    this.logger.step('Testing logout navigation and state cleanup');

    // Login first
    await this.loginTestUser();
    
    // Navigate to protected page
    const dashboard = new DashboardPage(this);
    await dashboard.navigate();
    await dashboard.waitForLoad();
    
    // Verify we're authenticated
    const isLoggedIn = await this.isLoggedIn();
    if (!isLoggedIn) {
      throw new Error('User should be logged in before logout test');
    }

    // Logout
    const nav = new NavigationComponent(this);
    await nav.logout();
    await this.delay(2000); // Wait for redirect and state cleanup

    // Try to access protected page after logout
    await dashboard.navigate();
    await this.delay(1000);
    
    const currentUrl = this.page.url();
    if (!currentUrl.includes('/login')) {
      throw new Error(`After logout, should redirect to login. Got: ${currentUrl}`);
    }

    // Verify authentication state is cleared
    const isStillLoggedIn = await this.isLoggedIn();
    if (isStillLoggedIn) {
      throw new Error('User should not be logged in after logout');
    }

    await this.screenshots.capture(this.page, 'logout-redirect-to-login');
    this.logger.success('Logout navigation test completed successfully');
  }

  /**
   * Helper method to test individual navigation paths
   */
  async testNavigationPath(name, page, expectRedirect, redirectTo, isAuthenticated) {
    this.logger.debug(`Testing ${name} navigation (authenticated: ${isAuthenticated})`);
    
    try {
      await page.navigate();
      await this.delay(1000); // Wait for potential redirect
      
      const currentUrl = this.page.url();
      
      if (expectRedirect && redirectTo) {
        if (!currentUrl.includes(redirectTo)) {
          throw new Error(`Expected redirect to ${redirectTo}, but got ${currentUrl}`);
        }
        this.logger.success(`✅ ${name}: Correctly redirected to ${redirectTo}`);
      } else if (expectRedirect && !redirectTo) {
        if (currentUrl === page.url) {
          throw new Error(`Expected redirect from ${name}, but stayed on same page`);
        }
        this.logger.success(`✅ ${name}: Correctly redirected away from page`);
      } else {
        if (currentUrl.includes('/login') && !page.url.includes('/login')) {
          throw new Error(`Unexpected redirect to login from ${name}`);
        }
        this.logger.success(`✅ ${name}: Accessible without redirect`);
      }
      
      await this.screenshots.capture(this.page, `nav-${name.toLowerCase().replace(/ /g, '-')}-${isAuthenticated ? 'auth' : 'unauth'}`);
      
    } catch (error) {
      this.logger.error(`❌ ${name}: Navigation test failed - ${error.message}`);
      await this.screenshots.capture(this.page, `nav-error-${name.toLowerCase().replace(/ /g, '-')}`);
      throw error;
    }
  }

  /**
   * Helper method to login test user
   */
  async loginTestUser() {
    const loginPage = new LoginPage(this);
    await loginPage.navigate();
    await loginPage.waitForLoad();
    
    // Try login (may fail if user doesn't exist, that's ok for navigation testing)
    try {
      await loginPage.login(this.testUsers.validUser.email, this.testUsers.validUser.password);
      await this.delay(2000); // Wait for login and potential redirect
    } catch (error) {
      this.logger.warning(`Login failed (this is expected for navigation testing): ${error.message}`);
      
      // Register user if login fails
      const registerPage = new RegisterPage(this);
      await registerPage.navigate();
      await registerPage.waitForLoad();
      
      await registerPage.register(
        this.testUsers.validUser.name,
        this.testUsers.validUser.email,
        this.testUsers.validUser.password
      );
      await this.delay(2000); // Wait for registration and redirect
    }
  }
}

/**
 * Run comprehensive navigation matrix tests
 */
async function runNavigationMatrixTests() {
  const navigationTest = new NavigationMatrixTest();
  
  await navigationTest.run(async function() {
    // Test unauthenticated navigation
    await this.testUnauthenticatedNavigation();
    
    // Test authenticated navigation
    await this.testAuthenticatedNavigation();
    
    // Test navbar navigation
    await this.testNavbarNavigation();
    await this.testAuthenticatedNavbarNavigation();
    
    // Test deep linking
    await this.testDeepLinking();
    
    // Test breadcrumb navigation
    await this.testBreadcrumbNavigation();
    
    // Test cross-page navigation
    await this.testCrossPageNavigation();
    
    // Test browser navigation
    await this.testBrowserNavigation();
    
    // Test logout navigation
    await this.testLogoutNavigation();
  });
}

// Export for use in main test suite
module.exports = { NavigationMatrixTest, runNavigationMatrixTests };

// Run tests if this file is executed directly
if (require.main === module) {
  runNavigationMatrixTests().catch(error => {
    console.error('Navigation matrix tests failed:', error);
    process.exit(1);
  });
}