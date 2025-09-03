/**
 * Comprehensive AI/LLM Integration Tests for Alchemorsel v3
 * 
 * Tests AI chat functionality, context retention, domain-specific responses,
 * recipe generation, and LLM integration to ensure the AI features work correctly
 * and provide cogent responses within the recipe/cooking domain.
 */

const { BaseTest } = require('./framework/base-test');
const { 
  AIChatPage, 
  LoginPage, 
  RegisterPage, 
  HomePage,
  RecipeDetailPage
} = require('./framework/page-objects');

/**
 * AI/LLM Integration Test Suite
 */
class AILLMIntegrationTest extends BaseTest {
  constructor() {
    super('AI-LLM-Integration');
    this.testUser = {
      name: 'AI Test User',
      email: 'aitest@example.com',
      password: 'aitestpassword123'
    };
    
    // Test conversation scenarios
    this.conversationScenarios = [
      {
        name: 'Recipe Generation',
        messages: [
          "I want to make pasta carbonara",
          "Make it for 4 people",
          "What wine pairs well with this dish?"
        ],
        expectedKeywords: ['pasta', 'carbonara', 'eggs', 'cheese', 'bacon', 'wine', 'white wine']
      },
      {
        name: 'Dietary Restrictions',
        messages: [
          "I'm vegetarian and want healthy recipes",
          "Something with quinoa",
          "How about making it spicy?"
        ],
        expectedKeywords: ['vegetarian', 'healthy', 'quinoa', 'vegetables', 'spicy', 'protein']
      },
      {
        name: 'Cooking Technique',
        messages: [
          "How do I properly sear chicken?",
          "What temperature should the pan be?",
          "How do I know when it's done?"
        ],
        expectedKeywords: ['sear', 'chicken', 'temperature', 'pan', 'heat', 'cooking', 'done']
      },
      {
        name: 'Ingredient Substitution',
        messages: [
          "I don't have butter, what can I use instead?",
          "Will it change the taste?",
          "What about for baking specifically?"
        ],
        expectedKeywords: ['butter', 'substitute', 'oil', 'margarine', 'taste', 'baking', 'texture']
      }
    ];
  }

  /**
   * Test basic AI chat functionality
   */
  async testBasicAIChat() {
    this.logger.step('Testing basic AI chat functionality');

    await this.loginTestUser();
    
    const aiChatPage = new AIChatPage(this);
    await aiChatPage.navigate();
    await aiChatPage.waitForLoad();

    // Send a simple message
    const testMessage = "Hello, I need help with cooking";
    await aiChatPage.sendMessage(testMessage);

    // Verify AI response
    const messages = await aiChatPage.getMessages();
    
    if (messages.length < 2) {
      throw new Error('AI should have responded to the test message');
    }

    const aiResponse = messages.find(m => m.type === 'ai');
    if (!aiResponse || aiResponse.text.length === 0) {
      throw new Error('AI response is empty');
    }

    this.logger.success(`AI responded with: ${aiResponse.text.substring(0, 100)}...`);
    
    // Check if response is cooking-related
    const cookingKeywords = ['cooking', 'recipe', 'food', 'kitchen', 'chef', 'ingredient'];
    const responseContainsCookingContent = cookingKeywords.some(keyword => 
      aiResponse.text.toLowerCase().includes(keyword)
    );

    if (!responseContainsCookingContent) {
      this.logger.warning('AI response may not be domain-specific (cooking-related)');
    } else {
      this.logger.success('AI response is appropriately domain-specific');
    }

    await this.screenshots.capture(this.page, 'basic-ai-chat-response');
  }

  /**
   * Test AI context retention across multiple messages
   */
  async testContextRetention() {
    this.logger.step('Testing AI context retention across conversation');

    await this.loginTestUser();
    
    const aiChatPage = new AIChatPage(this);
    await aiChatPage.navigate();
    await aiChatPage.waitForLoad();

    // Multi-turn conversation to test context
    const contextScenario = [
      {
        message: "I like spicy Italian food",
        checkFor: ['italian', 'spicy']
      },
      {
        message: "Create a recipe based on my preferences",
        checkFor: ['italian', 'spicy', 'recipe']
      },
      {
        message: "Make it vegetarian please",
        checkFor: ['vegetarian', 'italian', 'spicy']
      },
      {
        message: "What cheese would work best?",
        checkFor: ['cheese', 'vegetarian', 'italian']
      }
    ];

    const allMessages = [];

    for (let i = 0; i < contextScenario.length; i++) {
      const { message, checkFor } = contextScenario[i];
      
      this.logger.debug(`Context test message ${i + 1}: ${message}`);
      
      await aiChatPage.sendMessage(message);
      await this.delay(1000); // Allow for response processing
      
      const currentMessages = await aiChatPage.getMessages();
      allMessages.push(...currentMessages.slice(allMessages.length));
      
      // Get the latest AI response
      const latestAIMessage = currentMessages.filter(m => m.type === 'ai').pop();
      
      if (!latestAIMessage) {
        throw new Error(`No AI response to message: "${message}"`);
      }

      this.logger.debug(`AI response ${i + 1}: ${latestAIMessage.text.substring(0, 100)}...`);

      // Check if the AI response shows context awareness
      const responseText = latestAIMessage.text.toLowerCase();
      const contextMatches = checkFor.filter(keyword => responseText.includes(keyword));
      
      if (contextMatches.length === 0) {
        this.logger.warning(`AI may not be retaining context. Expected keywords: ${checkFor.join(', ')}`);
      } else {
        this.logger.success(`Context retained: found keywords ${contextMatches.join(', ')}`);
      }

      await this.screenshots.capture(this.page, `context-test-message-${i + 1}`);
    }

    // Verify that later messages reference earlier context
    const finalMessages = await aiChatPage.getMessages();
    const finalAIResponses = finalMessages.filter(m => m.type === 'ai');
    
    if (finalAIResponses.length >= 2) {
      const lastResponse = finalAIResponses[finalAIResponses.length - 1].text.toLowerCase();
      const hasItalianContext = lastResponse.includes('italian');
      const hasSpicyContext = lastResponse.includes('spicy');
      const hasVegetarianContext = lastResponse.includes('vegetarian');
      
      if (hasItalianContext && (hasSpicyContext || hasVegetarianContext)) {
        this.logger.success('✅ AI successfully maintained conversation context');
      } else {
        this.logger.warning('⚠️ AI context retention may be incomplete');
      }
    }
  }

  /**
   * Test recipe generation functionality
   */
  async testRecipeGeneration() {
    this.logger.step('Testing AI recipe generation functionality');

    await this.loginTestUser();
    
    const aiChatPage = new AIChatPage(this);
    await aiChatPage.navigate();
    await aiChatPage.waitForLoad();

    const recipeRequests = [
      {
        request: "Create a pasta recipe with mushrooms and cream",
        expectedElements: ['pasta', 'mushrooms', 'cream', 'ingredients', 'instructions']
      },
      {
        request: "Generate a vegan stir-fry recipe for 2 people",
        expectedElements: ['vegan', 'stir-fry', 'vegetables', 'sauce', 'serves 2']
      },
      {
        request: "Make me a chocolate cake recipe that's gluten-free",
        expectedElements: ['chocolate', 'cake', 'gluten-free', 'flour', 'baking']
      }
    ];

    for (const { request, expectedElements } of recipeRequests) {
      this.logger.debug(`Testing recipe generation: ${request}`);
      
      await aiChatPage.sendMessage(request);
      
      // Wait longer for recipe generation (LLM processing time)
      await aiChatPage.waitForAIResponse(90000); // 90 second timeout
      
      const messages = await aiChatPage.getMessages();
      const latestAI = messages.filter(m => m.type === 'ai').pop();
      
      if (!latestAI) {
        throw new Error(`No AI response for recipe request: ${request}`);
      }

      const responseText = latestAI.text.toLowerCase();
      
      // Check for recipe elements
      const foundElements = expectedElements.filter(element => 
        responseText.includes(element.toLowerCase())
      );
      
      this.logger.info(`Recipe generation response length: ${latestAI.text.length} characters`);
      this.logger.info(`Found expected elements: ${foundElements.join(', ')}`);
      
      if (foundElements.length < expectedElements.length / 2) {
        this.logger.warning(`Recipe may be incomplete. Found ${foundElements.length}/${expectedElements.length} expected elements`);
      } else {
        this.logger.success(`Recipe generation successful for: ${request}`);
      }

      // Check if a recipe was actually created and saved
      const hasRecipeNotification = await aiChatPage.hasRecipeGenerated();
      if (hasRecipeNotification) {
        this.logger.success('✅ Recipe was created and saved to user account');
      } else {
        this.logger.info('Recipe response provided but may not be saved (expected for demo data)');
      }

      await this.screenshots.capture(this.page, `recipe-generation-${request.replace(/[^a-zA-Z0-9]/g, '-')}`);
      
      // Add delay between requests to avoid overwhelming the AI service
      await this.delay(2000);
    }
  }

  /**
   * Test domain-specific knowledge and responses
   */
  async testDomainSpecificKnowledge() {
    this.logger.step('Testing domain-specific cooking knowledge');

    await this.loginTestUser();
    
    const aiChatPage = new AIChatPage(this);
    await aiChatPage.navigate();
    await aiChatPage.waitForLoad();

    const domainQuestions = [
      {
        question: "What's the difference between sautéing and pan-frying?",
        expectedDomain: ['sauté', 'pan-fry', 'heat', 'oil', 'temperature', 'cooking method'],
        category: 'Cooking Techniques'
      },
      {
        question: "How do I properly season a cast iron pan?",
        expectedDomain: ['cast iron', 'season', 'oil', 'oven', 'temperature', 'care'],
        category: 'Kitchen Equipment'
      },
      {
        question: "What spices go well with lamb?",
        expectedDomain: ['lamb', 'spices', 'rosemary', 'garlic', 'herbs', 'flavor'],
        category: 'Ingredient Pairing'
      },
      {
        question: "How do I know when bread dough has risen enough?",
        expectedDomain: ['bread', 'dough', 'rise', 'yeast', 'double', 'finger test'],
        category: 'Baking Knowledge'
      }
    ];

    for (const { question, expectedDomain, category } of domainQuestions) {
      this.logger.debug(`Testing ${category}: ${question}`);
      
      await aiChatPage.sendMessage(question);
      await aiChatPage.waitForAIResponse(60000);
      
      const messages = await aiChatPage.getMessages();
      const latestAI = messages.filter(m => m.type === 'ai').pop();
      
      if (!latestAI) {
        throw new Error(`No AI response for domain question: ${question}`);
      }

      const responseText = latestAI.text.toLowerCase();
      
      // Check domain relevance
      const domainMatches = expectedDomain.filter(term => 
        responseText.includes(term.toLowerCase())
      );
      
      const relevanceScore = domainMatches.length / expectedDomain.length;
      
      this.logger.info(`${category} relevance: ${Math.round(relevanceScore * 100)}% (${domainMatches.length}/${expectedDomain.length})`);
      
      if (relevanceScore >= 0.4) { // 40% threshold for domain relevance
        this.logger.success(`✅ ${category}: Domain knowledge demonstrated`);
      } else {
        this.logger.warning(`⚠️ ${category}: Response may lack domain expertise`);
      }

      // Check response quality (length and helpfulness indicators)
      if (responseText.length < 50) {
        this.logger.warning(`${category}: Response seems too short (${responseText.length} chars)`);
      } else if (responseText.length > 2000) {
        this.logger.warning(`${category}: Response seems too long (${responseText.length} chars)`);
      } else {
        this.logger.success(`${category}: Response length appropriate (${responseText.length} chars)`);
      }

      await this.screenshots.capture(this.page, `domain-knowledge-${category.replace(/ /g, '-')}`);
      await this.delay(1000);
    }
  }

  /**
   * Test AI error handling and edge cases
   */
  async testAIErrorHandling() {
    this.logger.step('Testing AI error handling and edge cases');

    await this.loginTestUser();
    
    const aiChatPage = new AIChatPage(this);
    await aiChatPage.navigate();
    await aiChatPage.waitForLoad();

    const edgeCaseTests = [
      {
        input: "", // Empty message
        expectedBehavior: "Should reject empty messages"
      },
      {
        input: "a".repeat(1500), // Very long message
        expectedBehavior: "Should handle or reject very long messages"
      },
      {
        input: "<script>alert('xss')</script>", // XSS attempt
        expectedBehavior: "Should sanitize dangerous input"
      },
      {
        input: "Tell me about nuclear weapons", // Off-topic request
        expectedBehavior: "Should redirect to cooking topics"
      },
      {
        input: "What's the weather like?", // Off-domain question
        expectedBehavior: "Should guide back to cooking domain"
      }
    ];

    for (const { input, expectedBehavior } of edgeCaseTests) {
      this.logger.debug(`Testing edge case: ${expectedBehavior}`);
      
      try {
        if (input === "") {
          // Test empty message handling
          const messageInput = await this.page.$('[data-testid="message-input"], textarea[name="message"], input[name="message"]');
          if (messageInput) {
            await messageInput.fill("");
            const sendButton = await this.page.$('[data-testid="send-btn"], button[type="submit"]');
            if (sendButton) {
              await sendButton.click();
              await this.delay(1000);
              
              // Check if message was rejected
              const messages = await aiChatPage.getMessages();
              const userMessages = messages.filter(m => m.type === 'user');
              
              if (userMessages.length === 0 || userMessages[userMessages.length - 1].text.trim() !== "") {
                this.logger.success('✅ Empty messages properly rejected');
              } else {
                this.logger.warning('⚠️ Empty message may have been accepted');
              }
            }
          }
        } else {
          await aiChatPage.sendMessage(input);
          await aiChatPage.waitForAIResponse(30000);
          
          const messages = await aiChatPage.getMessages();
          const latestAI = messages.filter(m => m.type === 'ai').pop();
          
          if (latestAI) {
            const responseText = latestAI.text.toLowerCase();
            
            // Check for appropriate handling
            if (input.includes('<script>')) {
              // XSS test - should be sanitized
              if (!responseText.includes('<script>')) {
                this.logger.success('✅ XSS input properly sanitized');
              } else {
                this.logger.error('❌ XSS input not properly sanitized');
              }
            } else if (input.includes('nuclear') || input.includes('weather')) {
              // Off-topic test - should redirect to cooking
              const cookingRedirect = ['cooking', 'recipe', 'food', 'kitchen', 'ingredient'].some(word => 
                responseText.includes(word)
              );
              
              if (cookingRedirect) {
                this.logger.success('✅ Off-topic request redirected to cooking domain');
              } else {
                this.logger.warning('⚠️ Off-topic request not properly handled');
              }
            }
          }
        }
        
        await this.screenshots.capture(this.page, `edge-case-${expectedBehavior.replace(/[^a-zA-Z0-9]/g, '-')}`);
        
      } catch (error) {
        // Some edge cases are expected to fail
        this.logger.info(`Edge case handling: ${error.message}`);
      }
      
      await this.delay(1000);
    }
  }

  /**
   * Test unauthenticated AI chat behavior
   */
  async testUnauthenticatedAIChat() {
    this.logger.step('Testing AI chat behavior for unauthenticated users');

    // Make sure we're logged out
    await this.page.goto(`${this.options.BASE_URL}/logout`);
    await this.delay(2000);

    const aiChatPage = new AIChatPage(this);
    await aiChatPage.navigate();
    await aiChatPage.waitForLoad();

    // Try to send a message as unauthenticated user
    try {
      await aiChatPage.sendMessage("Create a pasta recipe");
      await this.delay(2000);

      // Check if auth prompt is shown
      const hasAuthPrompt = await aiChatPage.hasAuthPrompt();
      
      if (hasAuthPrompt) {
        this.logger.success('✅ Authentication prompt shown for unauthenticated users');
      } else {
        // Check if redirected to login
        const currentUrl = this.page.url();
        if (currentUrl.includes('/login')) {
          this.logger.success('✅ Unauthenticated user redirected to login');
        } else {
          this.logger.warning('⚠️ Unauthenticated user behavior unclear');
        }
      }

      await this.screenshots.capture(this.page, 'unauthenticated-ai-chat');

    } catch (error) {
      // This might be expected behavior
      this.logger.info(`Unauthenticated AI chat error (may be expected): ${error.message}`);
    }
  }

  /**
   * Test conversation history persistence
   */
  async testConversationPersistence() {
    this.logger.step('Testing conversation history persistence');

    await this.loginTestUser();
    
    const aiChatPage = new AIChatPage(this);
    await aiChatPage.navigate();
    await aiChatPage.waitForLoad();

    // Send initial messages
    const testMessages = [
      "I want to cook Italian food",
      "Something with pasta and tomatoes"
    ];

    for (const message of testMessages) {
      await aiChatPage.sendMessage(message);
      await this.delay(2000);
    }

    const messagesBeforeReload = await aiChatPage.getMessages();
    const messageCountBefore = messagesBeforeReload.length;

    this.logger.info(`Messages before reload: ${messageCountBefore}`);

    // Reload the page to test persistence
    await this.page.reload();
    await aiChatPage.waitForLoad();

    const messagesAfterReload = await aiChatPage.getMessages();
    const messageCountAfter = messagesAfterReload.length;

    this.logger.info(`Messages after reload: ${messageCountAfter}`);

    if (messageCountAfter >= messageCountBefore) {
      this.logger.success('✅ Conversation history persisted across page reload');
    } else {
      this.logger.warning('⚠️ Conversation history may not be fully persistent');
    }

    // Test navigating away and back
    const homePage = new HomePage(this);
    await homePage.navigate();
    await homePage.waitForLoad();

    await aiChatPage.navigate();
    await aiChatPage.waitForLoad();

    const messagesAfterNavigation = await aiChatPage.getMessages();
    const messageCountAfterNav = messagesAfterNavigation.length;

    this.logger.info(`Messages after navigation: ${messageCountAfterNav}`);

    if (messageCountAfterNav >= messageCountBefore) {
      this.logger.success('✅ Conversation history persisted across navigation');
    } else {
      this.logger.warning('⚠️ Conversation history lost during navigation');
    }

    await this.screenshots.capture(this.page, 'conversation-persistence-test');
  }

  /**
   * Run comprehensive conversation scenarios
   */
  async testComprehensiveConversationScenarios() {
    this.logger.step('Testing comprehensive conversation scenarios');

    await this.loginTestUser();
    
    const aiChatPage = new AIChatPage(this);
    await aiChatPage.navigate();
    await aiChatPage.waitForLoad();

    for (const scenario of this.conversationScenarios) {
      this.logger.info(`Starting scenario: ${scenario.name}`);
      
      // Send all messages in the scenario
      for (let i = 0; i < scenario.messages.length; i++) {
        const message = scenario.messages[i];
        this.logger.debug(`Scenario ${scenario.name} - Message ${i + 1}: ${message}`);
        
        await aiChatPage.sendMessage(message);
        await aiChatPage.waitForAIResponse(60000);
        
        // Take screenshot after each message
        await this.screenshots.capture(this.page, `scenario-${scenario.name.replace(/ /g, '-')}-msg-${i + 1}`);
        
        await this.delay(1000);
      }
      
      // Analyze the full conversation
      const allMessages = await aiChatPage.getMessages();
      const aiResponses = allMessages.filter(m => m.type === 'ai');
      
      if (aiResponses.length === 0) {
        throw new Error(`No AI responses in scenario: ${scenario.name}`);
      }
      
      // Check if responses contain expected keywords
      const allResponseText = aiResponses.map(m => m.text).join(' ').toLowerCase();
      const foundKeywords = scenario.expectedKeywords.filter(keyword => 
        allResponseText.includes(keyword.toLowerCase())
      );
      
      const keywordScore = foundKeywords.length / scenario.expectedKeywords.length;
      
      this.logger.info(`Scenario ${scenario.name} keyword relevance: ${Math.round(keywordScore * 100)}%`);
      this.logger.info(`Found keywords: ${foundKeywords.join(', ')}`);
      
      if (keywordScore >= 0.5) {
        this.logger.success(`✅ Scenario ${scenario.name}: Passed relevance test`);
      } else {
        this.logger.warning(`⚠️ Scenario ${scenario.name}: Low relevance score`);
      }
      
      await this.screenshots.capture(this.page, `scenario-${scenario.name.replace(/ /g, '-')}-complete`);
      
      // Clear conversation for next scenario (if supported)
      try {
        const clearButton = await this.page.$('[data-testid="clear-btn"]');
        if (clearButton) {
          await clearButton.click();
          await this.delay(1000);
        }
      } catch (error) {
        this.logger.debug('No clear button found, continuing with existing conversation');
      }
    }
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
 * Run comprehensive AI/LLM integration tests
 */
async function runAILLMIntegrationTests() {
  const aiTest = new AILLMIntegrationTest();
  
  await aiTest.run(async function() {
    // Test basic AI chat functionality
    await this.testBasicAIChat();
    
    // Test context retention
    await this.testContextRetention();
    
    // Test recipe generation
    await this.testRecipeGeneration();
    
    // Test domain-specific knowledge
    await this.testDomainSpecificKnowledge();
    
    // Test error handling
    await this.testAIErrorHandling();
    
    // Test unauthenticated behavior
    await this.testUnauthenticatedAIChat();
    
    // Test conversation persistence
    await this.testConversationPersistence();
    
    // Test comprehensive scenarios
    await this.testComprehensiveConversationScenarios();
  });
}

// Export for use in main test suite
module.exports = { AILLMIntegrationTest, runAILLMIntegrationTests };

// Run tests if this file is executed directly
if (require.main === module) {
  runAILLMIntegrationTests().catch(error => {
    console.error('AI/LLM integration tests failed:', error);
    process.exit(1);
  });
}