// Alchemorsel v3 Main Application JavaScript
// Hot reload enabled development build

import '../scss/main.scss';
import { initializeHTMX } from './htmx-extensions';
import { initializePerformance } from './performance';
import { initializeAccessibility } from './accessibility';

// Hot reload detection
if (module.hot) {
  module.hot.accept();
  console.log('🔥 Hot reload enabled');
}

class AlchemorselApp {
  constructor() {
    this.version = '3.0.0-dev';
    this.debug = true;
    this.components = new Map();
    
    this.log('Initializing Alchemorsel v3 Application');
    this.init();
  }
  
  init() {
    // Wait for DOM to be ready
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', () => this.startup());
    } else {
      this.startup();
    }
  }
  
  startup() {
    this.log('Starting application components');
    
    try {
      // Initialize core components
      this.initializeHTMX();
      this.initializePerformance();
      this.initializeAccessibility();
      this.initializeEventListeners();
      this.initializeServiceWorker();
      
      // Initialize feature components
      this.initializeRecipeComponents();
      this.initializeAIComponents();
      this.initializeUserComponents();
      
      // Development features
      if (this.debug) {
        this.initializeDebugTools();
        this.initializeHotReload();
      }
      
      this.log('Application startup complete');
      this.dispatchEvent('app:ready');
      
    } catch (error) {
      this.error('Application startup failed', error);
    }
  }
  
  initializeHTMX() {
    try {
      initializeHTMX();
      this.log('HTMX extensions initialized');
    } catch (error) {
      this.error('HTMX initialization failed', error);
    }
  }
  
  initializePerformance() {
    try {
      initializePerformance();
      this.log('Performance monitoring initialized');
    } catch (error) {
      this.error('Performance initialization failed', error);
    }
  }
  
  initializeAccessibility() {
    try {
      initializeAccessibility();
      this.log('Accessibility features initialized');
    } catch (error) {
      this.error('Accessibility initialization failed', error);
    }
  }
  
  initializeEventListeners() {
    // Global click handler for analytics
    document.addEventListener('click', (event) => {
      this.trackInteraction('click', event.target);
    });
    
    // Form submission tracking
    document.addEventListener('submit', (event) => {
      this.trackInteraction('form_submit', event.target);
    });
    
    // Search interactions
    document.addEventListener('input', (event) => {
      if (event.target.matches('[data-search]')) {
        this.handleSearchInput(event);
      }
    });
    
    // Recipe interactions
    document.addEventListener('click', (event) => {
      if (event.target.matches('[data-recipe-action]')) {
        this.handleRecipeAction(event);
      }
    });
  }
  
  initializeServiceWorker() {
    if ('serviceWorker' in navigator) {
      navigator.serviceWorker.register('/sw.js')
        .then(registration => {
          this.log('Service Worker registered', registration);
        })
        .catch(error => {
          this.error('Service Worker registration failed', error);
        });
    }
  }
  
  initializeRecipeComponents() {
    // Recipe card interactions
    document.querySelectorAll('[data-recipe-card]').forEach(card => {
      this.initializeRecipeCard(card);
    });
    
    // Recipe form enhancements
    document.querySelectorAll('[data-recipe-form]').forEach(form => {
      this.initializeRecipeForm(form);
    });
    
    // Ingredient management
    document.querySelectorAll('[data-ingredients]').forEach(container => {
      this.initializeIngredientsManager(container);
    });
  }
  
  initializeAIComponents() {
    // AI chat interface
    const chatContainer = document.querySelector('[data-ai-chat]');
    if (chatContainer) {
      this.initializeAIChat(chatContainer);
    }
    
    // AI suggestions
    document.querySelectorAll('[data-ai-suggest]').forEach(element => {
      this.initializeAISuggestions(element);
    });
    
    // Recipe generation
    const recipeGenerator = document.querySelector('[data-recipe-generator]');
    if (recipeGenerator) {
      this.initializeRecipeGenerator(recipeGenerator);
    }
  }
  
  initializeUserComponents() {
    // User profile interactions
    const profileForm = document.querySelector('[data-profile-form]');
    if (profileForm) {
      this.initializeProfileForm(profileForm);
    }
    
    // User preferences
    const preferencesPanel = document.querySelector('[data-preferences]');
    if (preferencesPanel) {
      this.initializePreferences(preferencesPanel);
    }
    
    // Social features
    document.querySelectorAll('[data-social-action]').forEach(element => {
      this.initializeSocialAction(element);
    });
  }
  
  initializeDebugTools() {
    // Debug panel
    const debugPanel = this.createDebugPanel();
    document.body.appendChild(debugPanel);
    
    // Console commands
    window.alchemorsel = {
      version: this.version,
      debug: this.debug,
      app: this,
      reload: () => window.location.reload(),
      components: this.components,
      performance: () => this.getPerformanceMetrics(),
      log: (...args) => this.log(...args)
    };
    
    this.log('Debug tools initialized');
  }
  
  initializeHotReload() {
    // Connect to LiveReload if available
    if (window.location.hostname === 'localhost') {
      const script = document.createElement('script');
      script.src = 'http://localhost:35729/livereload.js';
      script.async = true;
      document.head.appendChild(script);
      
      this.log('Hot reload client loaded');
    }
  }
  
  // Component initializers
  initializeRecipeCard(card) {
    // Add hover effects and interactions
    card.addEventListener('mouseenter', () => {
      card.classList.add('hover');
    });
    
    card.addEventListener('mouseleave', () => {
      card.classList.remove('hover');
    });
    
    // Like button functionality
    const likeButton = card.querySelector('[data-like-button]');
    if (likeButton) {
      likeButton.addEventListener('click', (event) => {
        event.preventDefault();
        this.handleLikeAction(likeButton);
      });
    }
  }
  
  initializeRecipeForm(form) {
    // Auto-save functionality
    const autoSave = this.debounce(() => {
      this.saveFormData(form);
    }, 2000);
    
    form.addEventListener('input', autoSave);
    form.addEventListener('change', autoSave);
  }
  
  initializeAIChat(container) {
    // Create chat interface
    const chatInterface = this.createChatInterface();
    container.appendChild(chatInterface);
    
    // Handle message sending
    const sendButton = container.querySelector('[data-send-message]');
    if (sendButton) {
      sendButton.addEventListener('click', () => {
        this.sendChatMessage(container);
      });
    }
  }
  
  // Utility methods
  log(...args) {
    if (this.debug) {
      console.log(`[Alchemorsel v${this.version}]`, ...args);
    }
  }
  
  error(...args) {
    console.error(`[Alchemorsel v${this.version}]`, ...args);
  }
  
  dispatchEvent(eventName, detail = {}) {
    const event = new CustomEvent(eventName, { detail });
    document.dispatchEvent(event);
  }
  
  debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
      const later = () => {
        clearTimeout(timeout);
        func(...args);
      };
      clearTimeout(timeout);
      timeout = setTimeout(later, wait);
    };
  }
  
  trackInteraction(type, element) {
    // Analytics tracking
    if (this.debug) {
      this.log('Interaction tracked:', type, element);
    }
  }
  
  createDebugPanel() {
    const panel = document.createElement('div');
    panel.id = 'alchemorsel-debug-panel';
    panel.className = 'debug-panel';
    panel.innerHTML = `
      <div class="debug-header">
        <h3>Alchemorsel Debug</h3>
        <button onclick="this.parentElement.parentElement.style.display='none'">×</button>
      </div>
      <div class="debug-content">
        <div>Version: ${this.version}</div>
        <div>Components: <span id="debug-component-count">0</span></div>
        <div>Performance: <button onclick="window.alchemorsel.performance()">Check</button></div>
      </div>
    `;
    
    return panel;
  }
  
  getPerformanceMetrics() {
    const metrics = {
      navigation: performance.getEntriesByType('navigation')[0],
      resources: performance.getEntriesByType('resource').length,
      memory: performance.memory,
      timing: performance.timing
    };
    
    console.table(metrics);
    return metrics;
  }
}

// Initialize application
const app = new AlchemorselApp();

// Export for hot reload
export default app;