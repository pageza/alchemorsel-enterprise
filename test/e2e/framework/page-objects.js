/**
 * Page Object Models for Alchemorsel v3 E2E Testing
 * 
 * Comprehensive page objects covering all application pages and interactions
 */

const { BaseTest } = require('./base-test');

/**
 * Base Page Object - Foundation for all page objects
 */
class BasePage {
  constructor(test, url = '') {
    this.test = test;
    this.url = url;
    this.page = test.page;
    this.logger = test.logger;
  }

  async navigate() {
    await this.test.navigateTo(this.url);
  }

  async waitForLoad() {
    await this.page.waitForLoadState('networkidle');
    await this.test.waitForHTMXRequest();
  }

  async isDisplayed(selector, timeout = 5000) {
    try {
      await this.page.waitForSelector(selector, { timeout });
      return await this.page.isVisible(selector);
    } catch {
      return false;
    }
  }

  async getText(selector) {
    return await this.page.textContent(selector);
  }

  async getAttribute(selector, attribute) {
    return await this.page.getAttribute(selector, attribute);
  }

  async takeScreenshot(name) {
    return await this.test.screenshots.capture(this.page, `${this.constructor.name}-${name}`);
  }
}

/**
 * Home Page Object
 */
class HomePage extends BasePage {
  constructor(test) {
    super(test, '/');
  }

  get selectors() {
    return {
      title: 'h1, [data-testid="page-title"]',
      heroSection: '.hero, [data-testid="hero"]',
      featuredRecipes: '[data-testid="featured-recipes"], .featured-recipes',
      recipeCard: '.recipe-card, [data-testid="recipe-card"]',
      loginButton: 'a[href="/login"], [data-testid="login-btn"]',
      registerButton: 'a[href="/register"], [data-testid="register-btn"]',
      navigation: 'nav, [data-testid="navigation"]',
      searchBar: 'input[name="q"], [data-testid="search-input"]',
      userDashboard: '[data-testid="user-dashboard"]',
      quickActions: '[data-testid="quick-actions"]',
      recentRecipes: '[data-testid="recent-recipes"]'
    };
  }

  async isLoggedIn() {
    return await this.test.isLoggedIn();
  }

  async getFeaturedRecipeCount() {
    const recipes = await this.page.$$(`${this.selectors.recipeCard}`);
    return recipes.length;
  }

  async clickFeaturedRecipe(index = 0) {
    const recipes = await this.page.$$(`${this.selectors.recipeCard}`);
    if (recipes[index]) {
      await recipes[index].click();
      await this.test.waitForHTMXRequest();
    }
  }

  async searchRecipes(query) {
    if (await this.isDisplayed(this.selectors.searchBar)) {
      await this.test.typeText(this.selectors.searchBar, query);
      await this.page.keyboard.press('Enter');
      await this.test.waitForHTMXRequest();
    }
  }

  async getUserDashboardData() {
    if (await this.isDisplayed(this.selectors.userDashboard)) {
      return {
        hasQuickActions: await this.isDisplayed(this.selectors.quickActions),
        hasRecentRecipes: await this.isDisplayed(this.selectors.recentRecipes),
        quickActionCount: (await this.page.$$(`${this.selectors.quickActions} a`)).length
      };
    }
    return null;
  }
}

/**
 * Login Page Object
 */
class LoginPage extends BasePage {
  constructor(test) {
    super(test, '/login');
  }

  get selectors() {
    return {
      emailInput: 'input[name="email"], [data-testid="email-input"]',
      passwordInput: 'input[name="password"], [data-testid="password-input"]',
      loginButton: 'button[type="submit"], [data-testid="login-btn"]',
      errorMessage: '.error, [data-testid="error-message"]',
      successMessage: '.success, [data-testid="success-message"]',
      registerLink: 'a[href="/register"], [data-testid="register-link"]',
      forgotPasswordLink: 'a[href*="forgot"], [data-testid="forgot-password-link"]'
    };
  }

  async login(email, password) {
    await this.test.typeText(this.selectors.emailInput, email);
    await this.test.typeText(this.selectors.passwordInput, password);
    await this.test.clickElement(this.selectors.loginButton);
    await this.test.waitForHTMXRequest();
  }

  async getErrorMessage() {
    if (await this.isDisplayed(this.selectors.errorMessage)) {
      return await this.getText(this.selectors.errorMessage);
    }
    return null;
  }

  async hasValidationErrors() {
    const emailInvalid = await this.getAttribute(this.selectors.emailInput, 'aria-invalid');
    const passwordInvalid = await this.getAttribute(this.selectors.passwordInput, 'aria-invalid');
    return emailInvalid === 'true' || passwordInvalid === 'true';
  }
}

/**
 * Register Page Object
 */
class RegisterPage extends BasePage {
  constructor(test) {
    super(test, '/register');
  }

  get selectors() {
    return {
      nameInput: 'input[name="name"], [data-testid="name-input"]',
      emailInput: 'input[name="email"], [data-testid="email-input"]',
      passwordInput: 'input[name="password"], [data-testid="password-input"]',
      confirmPasswordInput: 'input[name="password_confirm"], [data-testid="confirm-password-input"]',
      registerButton: 'button[type="submit"], [data-testid="register-btn"]',
      errorMessage: '.error, [data-testid="error-message"]',
      loginLink: 'a[href="/login"], [data-testid="login-link"]',
      termsCheckbox: 'input[name="terms"], [data-testid="terms-checkbox"]'
    };
  }

  async register(name, email, password, confirmPassword = null) {
    await this.test.typeText(this.selectors.nameInput, name);
    await this.test.typeText(this.selectors.emailInput, email);
    await this.test.typeText(this.selectors.passwordInput, password);
    
    const actualConfirmPassword = confirmPassword || password;
    if (await this.isDisplayed(this.selectors.confirmPasswordInput)) {
      await this.test.typeText(this.selectors.confirmPasswordInput, actualConfirmPassword);
    }

    // Accept terms if checkbox exists
    if (await this.isDisplayed(this.selectors.termsCheckbox)) {
      await this.test.clickElement(this.selectors.termsCheckbox);
    }

    await this.test.clickElement(this.selectors.registerButton);
    await this.test.waitForHTMXRequest();
  }

  async getErrorMessage() {
    if (await this.isDisplayed(this.selectors.errorMessage)) {
      return await this.getText(this.selectors.errorMessage);
    }
    return null;
  }
}

/**
 * Dashboard Page Object
 */
class DashboardPage extends BasePage {
  constructor(test) {
    super(test, '/dashboard');
  }

  get selectors() {
    return {
      welcomeMessage: '.welcome, [data-testid="welcome-message"]',
      recipeStats: '[data-testid="recipe-stats"]',
      conversationStats: '[data-testid="conversation-stats"]',
      recentRecipes: '[data-testid="recent-recipes"]',
      quickActions: '[data-testid="quick-actions"]',
      quickActionCreate: '[data-testid="quick-action-create"]',
      quickActionAiChat: '[data-testid="quick-action-ai-chat"]',
      quickActionMyRecipes: '[data-testid="quick-action-my-recipes"]',
      quickActionFavorites: '[data-testid="quick-action-favorites"]',
      profileLink: '[data-testid="profile-link"]',
      logoutLink: '[data-testid="logout-link"]'
    };
  }

  async getRecipeStats() {
    if (await this.isDisplayed(this.selectors.recipeStats)) {
      return {
        totalRecipes: await this.getText(`${this.selectors.recipeStats} [data-stat="total"]`),
        publishedRecipes: await this.getText(`${this.selectors.recipeStats} [data-stat="published"]`),
        totalLikes: await this.getText(`${this.selectors.recipeStats} [data-stat="likes"]`),
        avgRating: await this.getText(`${this.selectors.recipeStats} [data-stat="rating"]`)
      };
    }
    return null;
  }

  async getConversationStats() {
    if (await this.isDisplayed(this.selectors.conversationStats)) {
      return {
        totalConversations: await this.getText(`${this.selectors.conversationStats} [data-stat="total"]`),
        activeConversations: await this.getText(`${this.selectors.conversationStats} [data-stat="active"]`),
        recipesGenerated: await this.getText(`${this.selectors.conversationStats} [data-stat="generated"]`)
      };
    }
    return null;
  }

  async clickQuickAction(action) {
    const actionSelector = this.selectors[`quickAction${action}`];
    if (actionSelector && await this.isDisplayed(actionSelector)) {
      await this.test.clickElement(actionSelector);
      await this.test.waitForHTMXRequest();
    }
  }

  async getRecentRecipesCount() {
    const recipes = await this.page.$$(`${this.selectors.recentRecipes} .recipe-card`);
    return recipes.length;
  }
}

/**
 * AI Chat Page Object
 */
class AIChatPage extends BasePage {
  constructor(test) {
    super(test, '/ai/chat');
  }

  get selectors() {
    return {
      chatMessages: '[data-testid="chat-messages"]',
      userMessage: '.user-message, [data-testid="user-message"]',
      aiMessage: '.ai-message, [data-testid="ai-message"]',
      messageInput: 'input[name="message"], textarea[name="message"], [data-testid="message-input"]',
      sendButton: 'button[type="submit"], [data-testid="send-btn"]',
      voiceButton: '[data-testid="voice-btn"]',
      clearButton: '[data-testid="clear-btn"]',
      conversationHistory: '[data-testid="conversation-history"]',
      recipeCreatedNotification: '.recipe-created-notification',
      authPrompt: '.auth-prompt',
      loadingIndicator: '.loading, [data-testid="loading"]'
    };
  }

  async sendMessage(message) {
    await this.test.typeText(this.selectors.messageInput, message);
    await this.test.clickElement(this.selectors.sendButton);
    
    // Wait for AI response with longer timeout for LLM
    await this.page.waitForSelector(this.selectors.aiMessage, { timeout: 60000 });
    await this.test.waitForHTMXRequest();
  }

  async getMessages() {
    const messages = [];
    
    const userMessages = await this.page.$$(this.selectors.userMessage);
    const aiMessages = await this.page.$$(this.selectors.aiMessage);
    
    // Extract text from all messages
    for (const userMsg of userMessages) {
      const text = await userMsg.textContent();
      messages.push({ type: 'user', text: text.trim() });
    }
    
    for (const aiMsg of aiMessages) {
      const text = await aiMsg.textContent();
      messages.push({ type: 'ai', text: text.trim() });
    }
    
    return messages;
  }

  async waitForAIResponse(timeout = 60000) {
    const messageCountBefore = (await this.page.$$(this.selectors.aiMessage)).length;
    
    await this.page.waitForFunction(
      (selector, countBefore) => {
        return document.querySelectorAll(selector).length > countBefore;
      },
      { timeout },
      this.selectors.aiMessage,
      messageCountBefore
    );
  }

  async hasRecipeGenerated() {
    return await this.isDisplayed(this.selectors.recipeCreatedNotification);
  }

  async hasAuthPrompt() {
    return await this.isDisplayed(this.selectors.authPrompt);
  }

  async getLastAIMessage() {
    const aiMessages = await this.page.$$(this.selectors.aiMessage);
    if (aiMessages.length > 0) {
      const lastMessage = aiMessages[aiMessages.length - 1];
      return await lastMessage.textContent();
    }
    return null;
  }

  async testContextRetention() {
    // Send initial message
    await this.sendMessage("I like spicy food");
    await this.waitForAIResponse();
    
    // Send follow-up that should reference context
    await this.sendMessage("Create a recipe based on my preference");
    await this.waitForAIResponse();
    
    const response = await this.getLastAIMessage();
    return response.toLowerCase().includes('spicy');
  }
}

/**
 * Recipes Page Object
 */
class RecipesPage extends BasePage {
  constructor(test) {
    super(test, '/recipes');
  }

  get selectors() {
    return {
      recipeGrid: '[data-testid="recipe-grid"], .recipe-grid',
      recipeCard: '.recipe-card, [data-testid="recipe-card"]',
      searchInput: 'input[name="q"], [data-testid="search-input"]',
      cuisineFilter: 'select[name="cuisine"], [data-testid="cuisine-filter"]',
      dietFilter: 'select[name="diet"], [data-testid="diet-filter"]',
      difficultyFilter: 'select[name="difficulty"], [data-testid="difficulty-filter"]',
      cookTimeFilter: 'input[name="max_cook_time"], [data-testid="cook-time-filter"]',
      searchResults: '[data-testid="search-results"]',
      noResults: '.no-results, [data-testid="no-results"]',
      loadMoreButton: '[data-testid="load-more"]',
      createRecipeButton: 'a[href="/recipes/new"], [data-testid="create-recipe-btn"]',
      sortSelect: 'select[name="sort"], [data-testid="sort-select"]'
    };
  }

  async searchRecipes(query) {
    await this.test.typeText(this.selectors.searchInput, query);
    await this.page.keyboard.press('Enter');
    await this.test.waitForHTMXRequest();
  }

  async filterByCuisine(cuisine) {
    if (await this.isDisplayed(this.selectors.cuisineFilter)) {
      await this.page.selectOption(this.selectors.cuisineFilter, cuisine);
      await this.test.waitForHTMXRequest();
    }
  }

  async filterByDiet(diet) {
    if (await this.isDisplayed(this.selectors.dietFilter)) {
      await this.page.selectOption(this.selectors.dietFilter, diet);
      await this.test.waitForHTMXRequest();
    }
  }

  async filterByDifficulty(difficulty) {
    if (await this.isDisplayed(this.selectors.difficultyFilter)) {
      await this.page.selectOption(this.selectors.difficultyFilter, difficulty);
      await this.test.waitForHTMXRequest();
    }
  }

  async setMaxCookTime(minutes) {
    if (await this.isDisplayed(this.selectors.cookTimeFilter)) {
      await this.test.typeText(this.selectors.cookTimeFilter, minutes.toString());
      await this.test.waitForHTMXRequest();
    }
  }

  async getRecipeCount() {
    const recipes = await this.page.$$(this.selectors.recipeCard);
    return recipes.length;
  }

  async getRecipeData(index = 0) {
    const recipes = await this.page.$$(this.selectors.recipeCard);
    if (recipes[index]) {
      const recipe = recipes[index];
      return {
        title: await recipe.$eval('.recipe-title', el => el.textContent.trim()).catch(() => ''),
        cuisine: await recipe.$eval('.recipe-cuisine', el => el.textContent.trim()).catch(() => ''),
        difficulty: await recipe.$eval('.recipe-difficulty', el => el.textContent.trim()).catch(() => ''),
        cookTime: await recipe.$eval('.recipe-cook-time', el => el.textContent.trim()).catch(() => ''),
        rating: await recipe.$eval('.recipe-rating', el => el.textContent.trim()).catch(() => '')
      };
    }
    return null;
  }

  async clickRecipe(index = 0) {
    const recipes = await this.page.$$(this.selectors.recipeCard);
    if (recipes[index]) {
      await recipes[index].click();
      await this.test.waitForHTMXRequest();
    }
  }
}

/**
 * Recipe Detail Page Object
 */
class RecipeDetailPage extends BasePage {
  constructor(test, recipeId = '') {
    super(test, `/recipes/${recipeId}`);
  }

  get selectors() {
    return {
      title: 'h1, [data-testid="recipe-title"]',
      description: '.recipe-description, [data-testid="recipe-description"]',
      ingredients: '.ingredients, [data-testid="ingredients"]',
      instructions: '.instructions, [data-testid="instructions"]',
      likeButton: '.like-button, [data-testid="like-btn"]',
      ratingStars: '.rating-stars, [data-testid="rating-stars"]',
      editButton: '[data-testid="edit-btn"]',
      deleteButton: '[data-testid="delete-btn"]',
      shareButton: '[data-testid="share-btn"]',
      nutritionInfo: '.nutrition, [data-testid="nutrition"]',
      authorInfo: '.recipe-author, [data-testid="author-info"]',
      cookingTime: '[data-testid="cooking-time"]',
      servings: '[data-testid="servings"]',
      difficulty: '[data-testid="difficulty"]',
      cuisine: '[data-testid="cuisine"]',
      comments: '[data-testid="comments"]',
      addCommentForm: '[data-testid="add-comment-form"]'
    };
  }

  async getRecipeDetails() {
    return {
      title: await this.getText(this.selectors.title),
      description: await this.getText(this.selectors.description),
      cookingTime: await this.getText(this.selectors.cookingTime),
      servings: await this.getText(this.selectors.servings),
      difficulty: await this.getText(this.selectors.difficulty),
      cuisine: await this.getText(this.selectors.cuisine)
    };
  }

  async getIngredients() {
    const ingredientElements = await this.page.$$(`${this.selectors.ingredients} li`);
    const ingredients = [];
    
    for (const ingredient of ingredientElements) {
      const text = await ingredient.textContent();
      ingredients.push(text.trim());
    }
    
    return ingredients;
  }

  async getInstructions() {
    const instructionElements = await this.page.$$(`${this.selectors.instructions} li, ${this.selectors.instructions} ol li`);
    const instructions = [];
    
    for (const instruction of instructionElements) {
      const text = await instruction.textContent();
      instructions.push(text.trim());
    }
    
    return instructions;
  }

  async likeRecipe() {
    if (await this.isDisplayed(this.selectors.likeButton)) {
      await this.test.clickElement(this.selectors.likeButton);
      await this.test.waitForHTMXRequest();
    }
  }

  async rateRecipe(stars) {
    if (await this.isDisplayed(this.selectors.ratingStars)) {
      const starElements = await this.page.$$(`${this.selectors.ratingStars} .star`);
      if (starElements[stars - 1]) {
        await starElements[stars - 1].click();
        await this.test.waitForHTMXRequest();
      }
    }
  }

  async canEdit() {
    return await this.isDisplayed(this.selectors.editButton);
  }

  async canDelete() {
    return await this.isDisplayed(this.selectors.deleteButton);
  }

  async editRecipe() {
    if (await this.canEdit()) {
      await this.test.clickElement(this.selectors.editButton);
      await this.test.waitForHTMXRequest();
    }
  }

  async deleteRecipe() {
    if (await this.canDelete()) {
      // Handle confirmation dialog
      this.page.on('dialog', async dialog => {
        await dialog.accept();
      });
      
      await this.test.clickElement(this.selectors.deleteButton);
      await this.test.waitForHTMXRequest();
    }
  }
}

/**
 * Recipe Form Page Object (New/Edit Recipe)
 */
class RecipeFormPage extends BasePage {
  constructor(test, recipeId = null) {
    const url = recipeId ? `/recipes/${recipeId}/edit` : '/recipes/new';
    super(test, url);
  }

  get selectors() {
    return {
      titleInput: 'input[name="title"], [data-testid="title-input"]',
      descriptionInput: 'textarea[name="description"], [data-testid="description-input"]',
      cuisineSelect: 'select[name="cuisine"], [data-testid="cuisine-select"]',
      difficultySelect: 'select[name="difficulty"], [data-testid="difficulty-select"]',
      servingsInput: 'input[name="servings"], [data-testid="servings-input"]',
      prepTimeInput: 'input[name="prep_time"], [data-testid="prep-time-input"]',
      cookTimeInput: 'input[name="cook_time"], [data-testid="cook-time-input"]',
      ingredientInputs: '.ingredient-input, [data-testid="ingredient-input"]',
      addIngredientButton: '[data-testid="add-ingredient-btn"]',
      removeIngredientButton: '[data-testid="remove-ingredient-btn"]',
      instructionInputs: '.instruction-input, [data-testid="instruction-input"]',
      addInstructionButton: '[data-testid="add-instruction-btn"]',
      removeInstructionButton: '[data-testid="remove-instruction-btn"]',
      imageUpload: 'input[type="file"], [data-testid="image-upload"]',
      tagsInput: 'input[name="tags"], [data-testid="tags-input"]',
      saveButton: 'button[type="submit"], [data-testid="save-btn"]',
      cancelButton: '[data-testid="cancel-btn"]',
      previewButton: '[data-testid="preview-btn"]'
    };
  }

  async fillBasicInfo(title, description, cuisine, difficulty, servings) {
    await this.test.typeText(this.selectors.titleInput, title);
    await this.test.typeText(this.selectors.descriptionInput, description);
    
    if (await this.isDisplayed(this.selectors.cuisineSelect)) {
      await this.page.selectOption(this.selectors.cuisineSelect, cuisine);
    }
    
    if (await this.isDisplayed(this.selectors.difficultySelect)) {
      await this.page.selectOption(this.selectors.difficultySelect, difficulty);
    }
    
    if (await this.isDisplayed(this.selectors.servingsInput)) {
      await this.test.typeText(this.selectors.servingsInput, servings.toString());
    }
  }

  async addIngredient(ingredient) {
    const addButton = this.selectors.addIngredientButton;
    if (await this.isDisplayed(addButton)) {
      await this.test.clickElement(addButton);
      await this.test.waitForHTMXRequest();
    }
    
    const ingredients = await this.page.$$(this.selectors.ingredientInputs);
    const lastIngredient = ingredients[ingredients.length - 1];
    if (lastIngredient) {
      await lastIngredient.fill(ingredient);
    }
  }

  async addInstruction(instruction) {
    const addButton = this.selectors.addInstructionButton;
    if (await this.isDisplayed(addButton)) {
      await this.test.clickElement(addButton);
      await this.test.waitForHTMXRequest();
    }
    
    const instructions = await this.page.$$(this.selectors.instructionInputs);
    const lastInstruction = instructions[instructions.length - 1];
    if (lastInstruction) {
      await lastInstruction.fill(instruction);
    }
  }

  async saveRecipe() {
    await this.test.clickElement(this.selectors.saveButton);
    await this.test.waitForHTMXRequest();
  }

  async uploadImage(imagePath) {
    if (await this.isDisplayed(this.selectors.imageUpload)) {
      await this.page.setInputFiles(this.selectors.imageUpload, imagePath);
      await this.test.waitForHTMXRequest();
    }
  }
}

/**
 * Profile Page Object
 */
class ProfilePage extends BasePage {
  constructor(test) {
    super(test, '/profile');
  }

  get selectors() {
    return {
      nameInput: 'input[name="name"], [data-testid="name-input"]',
      emailInput: 'input[name="email"], [data-testid="email-input"]',
      bioTextarea: 'textarea[name="bio"], [data-testid="bio-textarea"]',
      avatarUpload: 'input[type="file"], [data-testid="avatar-upload"]',
      saveButton: 'button[type="submit"], [data-testid="save-btn"]',
      changePasswordButton: '[data-testid="change-password-btn"]',
      deleteAccountButton: '[data-testid="delete-account-btn"]',
      successMessage: '.success, [data-testid="success-message"]',
      errorMessage: '.error, [data-testid="error-message"]',
      profileStats: '[data-testid="profile-stats"]',
      userRecipes: '[data-testid="user-recipes"]',
      accountSettings: '[data-testid="account-settings"]'
    };
  }

  async updateProfile(name, bio) {
    if (await this.isDisplayed(this.selectors.nameInput)) {
      await this.test.typeText(this.selectors.nameInput, name);
    }
    
    if (await this.isDisplayed(this.selectors.bioTextarea)) {
      await this.test.typeText(this.selectors.bioTextarea, bio);
    }
    
    await this.test.clickElement(this.selectors.saveButton);
    await this.test.waitForHTMXRequest();
  }

  async uploadAvatar(imagePath) {
    if (await this.isDisplayed(this.selectors.avatarUpload)) {
      await this.page.setInputFiles(this.selectors.avatarUpload, imagePath);
      await this.test.waitForHTMXRequest();
    }
  }

  async getSuccessMessage() {
    if (await this.isDisplayed(this.selectors.successMessage)) {
      return await this.getText(this.selectors.successMessage);
    }
    return null;
  }
}

/**
 * Navigation Component Object
 */
class NavigationComponent {
  constructor(test) {
    this.test = test;
    this.page = test.page;
  }

  get selectors() {
    return {
      navbar: 'nav, [data-testid="navbar"]',
      homeLink: 'a[href="/"], [data-testid="home-link"]',
      recipesLink: 'a[href="/recipes"], [data-testid="recipes-link"]',
      aiChatLink: 'a[href*="chat"], [data-testid="ai-chat-link"]',
      dashboardLink: 'a[href="/dashboard"], [data-testid="dashboard-link"]',
      profileLink: 'a[href="/profile"], [data-testid="profile-link"]',
      loginLink: 'a[href="/login"], [data-testid="login-link"]',
      registerLink: 'a[href="/register"], [data-testid="register-link"]',
      logoutButton: '[data-testid="logout-btn"]',
      userMenu: '[data-testid="user-menu"]',
      mobileMenuToggle: '[data-testid="mobile-menu-toggle"]',
      searchForm: '[data-testid="search-form"]',
      notificationIcon: '[data-testid="notifications"]'
    };
  }

  async navigateToHome() {
    await this.test.clickElement(this.selectors.homeLink);
    await this.test.waitForHTMXRequest();
  }

  async navigateToRecipes() {
    await this.test.clickElement(this.selectors.recipesLink);
    await this.test.waitForHTMXRequest();
  }

  async navigateToAIChat() {
    await this.test.clickElement(this.selectors.aiChatLink);
    await this.test.waitForHTMXRequest();
  }

  async navigateToDashboard() {
    await this.test.clickElement(this.selectors.dashboardLink);
    await this.test.waitForHTMXRequest();
  }

  async navigateToProfile() {
    await this.test.clickElement(this.selectors.profileLink);
    await this.test.waitForHTMXRequest();
  }

  async navigateToLogin() {
    await this.test.clickElement(this.selectors.loginLink);
    await this.test.waitForHTMXRequest();
  }

  async navigateToRegister() {
    await this.test.clickElement(this.selectors.registerLink);
    await this.test.waitForHTMXRequest();
  }

  async logout() {
    await this.test.clickElement(this.selectors.logoutButton);
    await this.test.waitForHTMXRequest();
  }

  async isUserLoggedIn() {
    return await this.test.isLoggedIn();
  }

  async openMobileMenu() {
    if (await this.isDisplayed(this.selectors.mobileMenuToggle)) {
      await this.test.clickElement(this.selectors.mobileMenuToggle);
      await this.test.delay(500); // Animation time
    }
  }

  async isDisplayed(selector) {
    try {
      return await this.page.isVisible(selector);
    } catch {
      return false;
    }
  }
}

module.exports = {
  BasePage,
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
};