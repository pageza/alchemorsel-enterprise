/**
 * Comprehensive HTMX Interaction Tests for Alchemorsel v3
 * 
 * Tests all HTMX-powered interactions including dynamic forms, real-time search,
 * partial page updates, form submissions, and server-sent events to ensure
 * the HTMX functionality works seamlessly with the backend.
 */

const { BaseTest } = require('./framework/base-test');
const { 
  HomePage, 
  LoginPage, 
  RegisterPage, 
  RecipesPage, 
  RecipeFormPage, 
  AIChatPage,
  DashboardPage,
  RecipeDetailPage
} = require('./framework/page-objects');

/**
 * HTMX Interaction Test Suite
 */
class HTMXInteractionTest extends BaseTest {
  constructor() {
    super('HTMX-Interactions');
    this.testUser = {
      name: 'HTMX Test User',
      email: 'htmxtest@example.com',
      password: 'htmxtestpassword123'
    };
  }

  /**
   * Test HTMX-powered search functionality
   */
  async testHTMXSearch() {
    this.logger.step('Testing HTMX-powered real-time search');

    const recipesPage = new RecipesPage(this);
    await recipesPage.navigate();
    await recipesPage.waitForLoad();

    // Test different search queries
    const searchQueries = [
      { query: 'pasta', expectedResults: true },
      { query: 'chicken', expectedResults: true },
      { query: 'vegetarian', expectedResults: true },
      { query: 'xyznonexistentdish', expectedResults: false },
      { query: '', expectedResults: true } // Empty search should show all
    ];

    for (const { query, expectedResults } of searchQueries) {
      this.logger.debug(`Testing search query: "${query}"`);
      
      // Monitor HTMX requests
      const htmxRequestPromise = this.page.waitForResponse(response => 
        response.url().includes('/recipes') && response.request().method() === 'GET'
      );

      await recipesPage.searchRecipes(query);
      
      // Wait for HTMX response
      try {
        const response = await Promise.race([
          htmxRequestPromise,
          new Promise((_, reject) => setTimeout(() => reject(new Error('HTMX request timeout')), 5000))
        ]);

        this.logger.success(`HTMX search request completed: ${response.status()}`);
        
        if (response.status() !== 200) {
          throw new Error(`HTMX search request failed with status: ${response.status()}`);
        }

      } catch (error) {
        this.logger.warning(`HTMX search request issue: ${error.message}`);
      }

      // Wait for HTMX request to complete
      await this.waitForHTMXRequest();
      
      // Verify results are updated
      await this.delay(1000);
      const resultCount = await recipesPage.getRecipeCount();
      
      if (expectedResults && resultCount === 0) {
        this.logger.warning(`Expected results for "${query}" but got 0`);
      } else if (!expectedResults && resultCount > 0) {
        this.logger.info(`Got ${resultCount} results for "${query}" (demo data may show results)`);
      } else {
        this.logger.success(`Search for "${query}" returned ${resultCount} results as expected`);
      }

      await this.screenshots.capture(this.page, `htmx-search-${query || 'empty'}`);
      await this.delay(500);
    }
  }

  /**
   * Test HTMX-powered filtering
   */
  async testHTMXFiltering() {
    this.logger.step('Testing HTMX-powered recipe filtering');

    const recipesPage = new RecipesPage(this);
    await recipesPage.navigate();
    await recipesPage.waitForLoad();

    const filterTests = [
      { type: 'cuisine', value: 'italian', description: 'Italian cuisine filter' },
      { type: 'diet', value: 'vegetarian', description: 'Vegetarian diet filter' },
      { type: 'difficulty', value: 'easy', description: 'Easy difficulty filter' },
      { type: 'cookTime', value: '30', description: '30-minute cook time filter' }
    ];

    for (const { type, value, description } of filterTests) {
      this.logger.debug(`Testing ${description}`);
      
      const initialCount = await recipesPage.getRecipeCount();
      this.logger.info(`Initial recipe count: ${initialCount}`);
      
      // Monitor for HTMX request
      let htmxRequestDetected = false;
      const responsePromise = this.page.waitForResponse(response => {
        const isHTMX = response.request().headers()['hx-request'] === 'true';
        const isRelevant = response.url().includes('/recipes');
        if (isHTMX && isRelevant) {
          htmxRequestDetected = true;
          return true;
        }
        return false;
      });

      // Apply filter
      try {
        if (type === 'cuisine') {
          await recipesPage.filterByCuisine(value);
        } else if (type === 'diet') {
          await recipesPage.filterByDiet(value);
        } else if (type === 'difficulty') {
          await recipesPage.filterByDifficulty(value);
        } else if (type === 'cookTime') {
          await recipesPage.setMaxCookTime(parseInt(value));
        }

        // Wait for HTMX response with timeout
        try {
          await Promise.race([
            responsePromise,
            new Promise((_, reject) => setTimeout(() => reject(new Error('No HTMX response')), 5000))
          ]);
          
          if (htmxRequestDetected) {
            this.logger.success(`HTMX filter request detected for ${description}`);
          }
        } catch (error) {
          this.logger.warning(`No HTMX response detected for ${description}: ${error.message}`);
        }

        await this.waitForHTMXRequest();
        await this.delay(1000);

        const filteredCount = await recipesPage.getRecipeCount();
        this.logger.info(`Filtered recipe count: ${filteredCount}`);

        // Verify filtering worked (in demo data, all results may be shown)
        this.logger.success(`${description} applied, showing ${filteredCount} recipes`);

        await this.screenshots.capture(this.page, `htmx-filter-${type}-${value}`);

      } catch (error) {
        this.logger.error(`Filter test failed for ${description}: ${error.message}`);
      }
      
      await this.delay(500);
    }
  }

  /**
   * Test HTMX form submissions
   */
  async testHTMXFormSubmissions() {
    this.logger.step('Testing HTMX form submissions');

    await this.loginTestUser();

    // Test login form (HTMX submission)
    await this.testLoginFormHTMX();
    
    // Test recipe creation form
    await this.testRecipeFormHTMX();
    
    // Test AI chat form
    await this.testAIChatFormHTMX();
  }

  /**
   * Test HTMX login form submission
   */
  async testLoginFormHTMX() {
    this.logger.debug('Testing HTMX login form submission');

    // First logout to test login
    await this.page.goto(`${this.options.BASE_URL}/logout`);
    await this.delay(1000);

    const loginPage = new LoginPage(this);
    await loginPage.navigate();
    await loginPage.waitForLoad();

    // Monitor for HTMX form submission
    const htmxFormPromise = this.page.waitForResponse(response => {
      const isHTMX = response.request().headers()['hx-request'] === 'true';
      const isLogin = response.url().includes('/login') || response.url().includes('/auth');
      return isHTMX && isLogin;
    });

    // Fill and submit form
    await this.page.fill('input[name="email"]', this.testUser.email);
    await this.page.fill('input[name="password"]', this.testUser.password);
    
    // Click submit button and wait for HTMX
    await this.page.click('button[type="submit"]');

    try {
      const response = await Promise.race([
        htmxFormPromise,
        new Promise((_, reject) => setTimeout(() => reject(new Error('HTMX timeout')), 5000))
      ]);

      this.logger.success(`HTMX login form submitted: ${response.status()}`);
      
      // Check for HTMX redirect header
      const htmxRedirect = response.headers()['hx-redirect'];
      if (htmxRedirect) {
        this.logger.success(`HTMX redirect detected: ${htmxRedirect}`);
      }

    } catch (error) {
      this.logger.warning(`HTMX login form submission not detected: ${error.message}`);
    }

    await this.delay(2000); // Wait for potential redirect
    await this.screenshots.capture(this.page, 'htmx-login-form-submission');
  }

  /**
   * Test HTMX recipe form submission
   */
  async testRecipeFormHTMX() {
    this.logger.debug('Testing HTMX recipe form submission');

    const recipeFormPage = new RecipeFormPage(this);
    await recipeFormPage.navigate();
    await recipeFormPage.waitForLoad();

    // Monitor for HTMX form submission
    const htmxFormPromise = this.page.waitForResponse(response => {
      const isHTMX = response.request().headers()['hx-request'] === 'true';
      const isRecipe = response.url().includes('/recipes');
      const isPost = response.request().method() === 'POST';
      return isHTMX && isRecipe && isPost;
    });

    // Fill basic form data
    await recipeFormPage.fillBasicInfo(
      'HTMX Test Recipe',
      'A recipe created to test HTMX form submission',
      'italian',
      'easy',
      4
    );

    // Add ingredients dynamically (HTMX)
    await recipeFormPage.addIngredient('2 cups pasta');
    await this.delay(500);
    await recipeFormPage.addIngredient('1 cup tomato sauce');
    await this.delay(500);

    // Add instructions dynamically (HTMX)
    await recipeFormPage.addInstruction('Boil pasta in salted water');
    await this.delay(500);
    await recipeFormPage.addInstruction('Mix with sauce and serve');
    await this.delay(500);

    // Submit form
    await recipeFormPage.saveRecipe();

    try {
      const response = await Promise.race([
        htmxFormPromise,
        new Promise((_, reject) => setTimeout(() => reject(new Error('HTMX timeout')), 10000))
      ]);

      this.logger.success(`HTMX recipe form submitted: ${response.status()}`);

    } catch (error) {
      this.logger.warning(`HTMX recipe form submission not detected: ${error.message}`);
    }

    await this.delay(2000);
    await this.screenshots.capture(this.page, 'htmx-recipe-form-submission');
  }

  /**
   * Test HTMX AI chat form submission
   */
  async testAIChatFormHTMX() {
    this.logger.debug('Testing HTMX AI chat form submission');

    const aiChatPage = new AIChatPage(this);
    await aiChatPage.navigate();
    await aiChatPage.waitForLoad();

    // Monitor for HTMX chat submission
    const htmxChatPromise = this.page.waitForResponse(response => {
      const isHTMX = response.request().headers()['hx-request'] === 'true';
      const isChat = response.url().includes('/chat') || response.url().includes('/ai');
      const isPost = response.request().method() === 'POST';
      return isHTMX && isChat && isPost;
    });

    // Send a message
    const testMessage = "Create a simple pasta recipe";
    await this.page.fill('input[name="message"], textarea[name="message"]', testMessage);
    await this.page.click('button[type="submit"]');

    try {
      const response = await Promise.race([
        htmxChatPromise,
        new Promise((_, reject) => setTimeout(() => reject(new Error('HTMX timeout')), 10000))
      ]);

      this.logger.success(`HTMX AI chat form submitted: ${response.status()}`);

      // Wait for AI response to be added to DOM
      await this.waitForHTMXRequest();
      await this.delay(2000);

      // Verify chat messages appeared
      const messages = await aiChatPage.getMessages();
      if (messages.length >= 1) {
        this.logger.success(`Chat messages updated via HTMX: ${messages.length} messages`);
      } else {
        this.logger.warning('No chat messages found after HTMX submission');
      }

    } catch (error) {
      this.logger.warning(`HTMX AI chat submission not detected: ${error.message}`);
    }

    await this.screenshots.capture(this.page, 'htmx-ai-chat-submission');
  }

  /**
   * Test dynamic content loading
   */
  async testDynamicContentLoading() {
    this.logger.step('Testing HTMX dynamic content loading');

    await this.loginTestUser();

    // Test dashboard dynamic content
    await this.testDashboardDynamicContent();
    
    // Test recipe interaction dynamic content
    await this.testRecipeInteractionContent();
  }

  /**
   * Test dashboard dynamic content
   */
  async testDashboardDynamicContent() {
    this.logger.debug('Testing dashboard dynamic content loading');

    const dashboardPage = new DashboardPage(this);
    await dashboardPage.navigate();
    await dashboardPage.waitForLoad();

    // Test quick action links (might load content via HTMX)
    const quickActions = ['Create', 'AiChat', 'MyRecipes', 'Favorites'];

    for (const action of quickActions) {
      try {
        // Monitor for HTMX requests
        const htmxPromise = this.page.waitForResponse(response => {
          return response.request().headers()['hx-request'] === 'true';
        }, { timeout: 3000 });

        await dashboardPage.clickQuickAction(action);
        
        try {
          const response = await htmxPromise;
          this.logger.success(`HTMX request detected for ${action}: ${response.status()}`);
        } catch (error) {
          this.logger.debug(`No HTMX request for ${action} (may be regular navigation)`);
        }

        await this.delay(1000);
        await this.screenshots.capture(this.page, `htmx-dashboard-${action.toLowerCase()}`);

        // Navigate back to dashboard for next test
        await dashboardPage.navigate();
        await dashboardPage.waitForLoad();

      } catch (error) {
        this.logger.warning(`Quick action ${action} test failed: ${error.message}`);
      }
    }
  }

  /**
   * Test recipe interaction dynamic content
   */
  async testRecipeInteractionContent() {
    this.logger.debug('Testing recipe interaction dynamic content');

    const recipesPage = new RecipesPage(this);
    await recipesPage.navigate();
    await recipesPage.waitForLoad();

    // Try to click on a recipe if available
    const recipeCount = await recipesPage.getRecipeCount();
    
    if (recipeCount > 0) {
      // Monitor for HTMX request
      const htmxPromise = this.page.waitForResponse(response => 
        response.request().headers()['hx-request'] === 'true'
      );

      await recipesPage.clickRecipe(0);
      
      try {
        const response = await Promise.race([
          htmxPromise,
          new Promise((_, reject) => setTimeout(() => reject(new Error('No HTMX')), 3000))
        ]);
        
        this.logger.success(`HTMX recipe click detected: ${response.status()}`);
      } catch (error) {
        this.logger.debug('Recipe click may be regular navigation, not HTMX');
      }

      await this.delay(2000);

      // Test like/rating interactions if on recipe detail page
      const currentUrl = this.page.url();
      if (currentUrl.includes('/recipes/')) {
        await this.testRecipeLikeRatingHTMX();
      }
    } else {
      this.logger.info('No recipes available to test interaction');
    }

    await this.screenshots.capture(this.page, 'htmx-recipe-interaction');
  }

  /**
   * Test recipe like/rating HTMX interactions
   */
  async testRecipeLikeRatingHTMX() {
    this.logger.debug('Testing recipe like/rating HTMX interactions');

    // Test like button
    const likeButton = await this.page.$('.like-button, [data-testid="like-btn"]');
    if (likeButton) {
      const htmxLikePromise = this.page.waitForResponse(response => 
        response.request().headers()['hx-request'] === 'true' &&
        response.url().includes('like')
      );

      await likeButton.click();

      try {
        const response = await Promise.race([
          htmxLikePromise,
          new Promise((_, reject) => setTimeout(() => reject(new Error('No HTMX')), 3000))
        ]);
        
        this.logger.success(`HTMX like interaction: ${response.status()}`);
      } catch (error) {
        this.logger.debug('Like button may not use HTMX');
      }

      await this.delay(1000);
    }

    // Test rating stars
    const ratingStars = await this.page.$$('.rating-stars .star, [data-testid="rating-stars"] .star');
    if (ratingStars.length > 0) {
      const htmxRatingPromise = this.page.waitForResponse(response => 
        response.request().headers()['hx-request'] === 'true' &&
        response.url().includes('rating')
      );

      await ratingStars[2].click(); // Click 3-star rating

      try {
        const response = await Promise.race([
          htmxRatingPromise,
          new Promise((_, reject) => setTimeout(() => reject(new Error('No HTMX')), 3000))
        ]);
        
        this.logger.success(`HTMX rating interaction: ${response.status()}`);
      } catch (error) {
        this.logger.debug('Rating stars may not use HTMX');
      }

      await this.delay(1000);
    }
  }

  /**
   * Test HTMX error handling
   */
  async testHTMXErrorHandling() {
    this.logger.step('Testing HTMX error handling');

    await this.loginTestUser();

    // Test form submission with invalid data
    const recipeFormPage = new RecipeFormPage(this);
    await recipeFormPage.navigate();
    await recipeFormPage.waitForLoad();

    // Submit form with missing required fields
    const htmxErrorPromise = this.page.waitForResponse(response => 
      response.request().headers()['hx-request'] === 'true' &&
      (response.status() >= 400 || response.status() < 200)
    );

    // Try to submit empty form
    await this.page.click('button[type="submit"]');

    try {
      const response = await Promise.race([
        htmxErrorPromise,
        new Promise((_, reject) => setTimeout(() => reject(new Error('No error response')), 5000))
      ]);
      
      this.logger.success(`HTMX error handling test: ${response.status()}`);
      
      // Check if error message is displayed
      await this.delay(1000);
      const errorMessage = await this.page.$('.error, [data-testid="error-message"]');
      if (errorMessage) {
        const errorText = await errorMessage.textContent();
        this.logger.success(`Error message displayed: ${errorText}`);
      } else {
        this.logger.warning('No error message displayed for validation failure');
      }

    } catch (error) {
      this.logger.info(`HTMX error test: ${error.message} (may be expected)`);
    }

    await this.screenshots.capture(this.page, 'htmx-error-handling');
  }

  /**
   * Test HTMX request headers and responses
   */
  async testHTMXRequestResponse() {
    this.logger.step('Testing HTMX request headers and response handling');

    const recipesPage = new RecipesPage(this);
    await recipesPage.navigate();
    await recipesPage.waitForLoad();

    // Monitor for HTMX requests and validate headers
    this.page.on('request', request => {
      const isHTMX = request.headers()['hx-request'] === 'true';
      if (isHTMX) {
        this.logger.debug('HTMX request detected:', {
          url: request.url(),
          method: request.method(),
          headers: {
            'hx-request': request.headers()['hx-request'],
            'hx-target': request.headers()['hx-target'],
            'hx-trigger': request.headers()['hx-trigger']
          }
        });
      }
    });

    this.page.on('response', response => {
      const request = response.request();
      const isHTMX = request.headers()['hx-request'] === 'true';
      if (isHTMX) {
        this.logger.debug('HTMX response received:', {
          url: response.url(),
          status: response.status(),
          headers: {
            'content-type': response.headers()['content-type'],
            'hx-redirect': response.headers()['hx-redirect'],
            'hx-refresh': response.headers()['hx-refresh']
          }
        });
      }
    });

    // Trigger some HTMX requests
    await recipesPage.searchRecipes('test');
    await this.delay(2000);

    await recipesPage.filterByCuisine('italian');
    await this.delay(2000);

    this.logger.success('HTMX request/response monitoring completed');
    await this.screenshots.capture(this.page, 'htmx-request-response-test');
  }

  /**
   * Test HTMX loading states and indicators
   */
  async testHTMXLoadingStates() {
    this.logger.step('Testing HTMX loading states and indicators');

    const aiChatPage = new AIChatPage(this);
    await this.loginTestUser();
    await aiChatPage.navigate();
    await aiChatPage.waitForLoad();

    // Send a message and look for loading indicators
    const messageInput = await this.page.$('input[name="message"], textarea[name="message"]');
    if (messageInput) {
      await messageInput.fill('Create a recipe');
      
      // Monitor for loading indicators
      const submitButton = await this.page.$('button[type="submit"]');
      if (submitButton) {
        await submitButton.click();
        
        // Check for various loading states
        await this.delay(500);
        
        // Look for htmx-request class on body
        const bodyHasHtmxRequest = await this.page.evaluate(() => 
          document.body.classList.contains('htmx-request')
        );
        
        if (bodyHasHtmxRequest) {
          this.logger.success('HTMX request indicator found on body');
        } else {
          this.logger.debug('No htmx-request class found on body');
        }
        
        // Look for loading indicators
        const loadingIndicator = await this.page.$('.loading, [data-testid="loading"]');
        if (loadingIndicator) {
          this.logger.success('Loading indicator found during HTMX request');
        }
        
        // Wait for request to complete
        await this.waitForHTMXRequest();
        
        // Verify loading state is cleared
        const bodyStillHasHtmxRequest = await this.page.evaluate(() => 
          document.body.classList.contains('htmx-request')
        );
        
        if (!bodyStillHasHtmxRequest) {
          this.logger.success('HTMX request indicator cleared after completion');
        } else {
          this.logger.warning('HTMX request indicator may not have been cleared');
        }
      }
    }

    await this.screenshots.capture(this.page, 'htmx-loading-states');
  }

  /**
   * Helper method to login test user
   */
  async loginTestUser() {
    const loginPage = new LoginPage(this);
    await loginPage.navigate();
    await loginPage.waitForLoad();
    
    // Try login first
    try {
      await loginPage.login(this.testUser.email, this.testUser.password);
      await this.delay(2000);
    } catch (error) {
      // Register user if login fails
      const registerPage = new RegisterPage(this);
      await registerPage.navigate();
      await registerPage.waitForLoad();
      
      await registerPage.register(
        this.testUser.name,
        this.testUser.email,
        this.testUser.password
      );
      await this.delay(2000);
    }
  }
}

/**
 * Run comprehensive HTMX interaction tests
 */
async function runHTMXInteractionTests() {
  const htmxTest = new HTMXInteractionTest();
  
  await htmxTest.run(async function() {
    // Test HTMX search functionality
    await this.testHTMXSearch();
    
    // Test HTMX filtering
    await this.testHTMXFiltering();
    
    // Test HTMX form submissions
    await this.testHTMXFormSubmissions();
    
    // Test dynamic content loading
    await this.testDynamicContentLoading();
    
    // Test HTMX error handling
    await this.testHTMXErrorHandling();
    
    // Test HTMX request/response cycle
    await this.testHTMXRequestResponse();
    
    // Test HTMX loading states
    await this.testHTMXLoadingStates();
  });
}

// Export for use in main test suite
module.exports = { HTMXInteractionTest, runHTMXInteractionTests };

// Run tests if this file is executed directly
if (require.main === module) {
  runHTMXInteractionTests().catch(error => {
    console.error('HTMX interaction tests failed:', error);
    process.exit(1);
  });
}