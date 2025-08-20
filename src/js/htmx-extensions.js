// HTMX Extensions for Alchemorsel v3
// Enhanced HTMX functionality with hot reload support

// Import HTMX
import 'htmx.org';

// HTMX configuration
export function initializeHTMX() {
  // Configure HTMX
  htmx.config.defaultSwapStyle = 'outerHTML';
  htmx.config.defaultSwapDelay = 0;
  htmx.config.defaultSettleDelay = 20;
  htmx.config.historyCacheSize = 10;
  htmx.config.refreshOnHistoryMiss = false;
  htmx.config.requestClass = 'htmx-request';
  htmx.config.addedClass = 'htmx-added';
  htmx.config.settlingClass = 'htmx-settling';
  htmx.config.swappingClass = 'htmx-swapping';
  
  // Enable debug mode in development
  if (window.location.hostname === 'localhost') {
    htmx.config.withCredentials = false;
    htmx.logAll();
  }
  
  // Global request configuration
  htmx.on('htmx:configRequest', (event) => {
    // Add CSRF token if available
    const csrfToken = document.querySelector('meta[name="csrf-token"]');
    if (csrfToken) {
      event.detail.headers['X-CSRF-Token'] = csrfToken.getAttribute('content');
    }
    
    // Add custom headers
    event.detail.headers['X-Requested-With'] = 'XMLHttpRequest';
    event.detail.headers['X-Alchemorsel-Version'] = '3.0.0';
  });
  
  // Global response handlers
  htmx.on('htmx:responseError', (event) => {
    console.error('HTMX Response Error:', event.detail);
    showNotification('Request failed. Please try again.', 'error');
  });
  
  htmx.on('htmx:sendError', (event) => {
    console.error('HTMX Send Error:', event.detail);
    showNotification('Network error. Please check your connection.', 'error');
  });
  
  // Success handler
  htmx.on('htmx:afterSwap', (event) => {
    // Reinitialize components after swap
    initializeNewContent(event.target);
    
    // Show success message if present
    const successMessage = event.target.querySelector('[data-success-message]');
    if (successMessage) {
      showNotification(successMessage.textContent, 'success');
    }
  });
  
  // Loading states
  htmx.on('htmx:beforeRequest', (event) => {
    const trigger = event.detail.elt;
    if (trigger) {
      trigger.classList.add('loading');
      
      // Add spinner if button
      if (trigger.tagName === 'BUTTON') {
        const originalText = trigger.textContent;
        trigger.dataset.originalText = originalText;
        trigger.innerHTML = `<span class="spinner"></span> ${originalText}`;
        trigger.disabled = true;
      }
    }
  });
  
  htmx.on('htmx:afterRequest', (event) => {
    const trigger = event.detail.elt;
    if (trigger) {
      trigger.classList.remove('loading');
      
      // Restore button text
      if (trigger.tagName === 'BUTTON' && trigger.dataset.originalText) {
        trigger.textContent = trigger.dataset.originalText;
        trigger.disabled = false;
        delete trigger.dataset.originalText;
      }
    }
  });
  
  // Custom extensions
  registerCustomExtensions();
  
  console.log('HTMX initialized with Alchemorsel extensions');
}

// Custom HTMX extensions
function registerCustomExtensions() {
  
  // Auto-save extension
  htmx.defineExtension('auto-save', {
    onEvent: function(name, evt) {
      if (name === 'input' || name === 'change') {
        const element = evt.target;
        if (element.hasAttribute('hx-auto-save')) {
          clearTimeout(element._autoSaveTimeout);
          element._autoSaveTimeout = setTimeout(() => {
            htmx.trigger(element, 'auto-save');
          }, 2000);
        }
      }
    }
  });
  
  // Confirmation extension
  htmx.defineExtension('confirm', {
    onEvent: function(name, evt) {
      if (name === 'htmx:confirm') {
        const message = evt.target.getAttribute('hx-confirm') || 'Are you sure?';
        const confirmed = confirm(message);
        if (!confirmed) {
          evt.preventDefault();
          return false;
        }
      }
    }
  });
  
  // Debounce extension
  htmx.defineExtension('debounce', {
    onEvent: function(name, evt) {
      const element = evt.target;
      const delay = element.getAttribute('hx-debounce') || '500';
      
      if (name === 'input') {
        clearTimeout(element._debounceTimeout);
        element._debounceTimeout = setTimeout(() => {
          htmx.trigger(element, 'debounced-input');
        }, parseInt(delay));
      }
    }
  });
  
  // Loading states extension
  htmx.defineExtension('loading-states', {
    onEvent: function(name, evt) {
      const element = evt.target;
      
      if (name === 'htmx:beforeRequest') {
        // Show loading indicator
        const loadingTarget = document.querySelector(element.getAttribute('hx-loading-target') || element);
        if (loadingTarget) {
          loadingTarget.classList.add('htmx-loading');
        }
        
        // Disable form elements
        if (element.tagName === 'FORM') {
          element.querySelectorAll('input, button, select, textarea').forEach(input => {
            input.disabled = true;
          });
        }
      }
      
      if (name === 'htmx:afterRequest') {
        // Hide loading indicator
        const loadingTarget = document.querySelector(element.getAttribute('hx-loading-target') || element);
        if (loadingTarget) {
          loadingTarget.classList.remove('htmx-loading');
        }
        
        // Re-enable form elements
        if (element.tagName === 'FORM') {
          element.querySelectorAll('input, button, select, textarea').forEach(input => {
            input.disabled = false;
          });
        }
      }
    }
  });
  
  // Recipe-specific extensions
  htmx.defineExtension('recipe-actions', {
    onEvent: function(name, evt) {
      if (name === 'htmx:afterSwap') {
        const element = evt.target;
        
        // Handle like button updates
        if (element.hasAttribute('data-like-button')) {
          const isLiked = element.classList.contains('liked');
          const count = element.querySelector('.like-count');
          
          // Animate the change
          element.classList.add('like-animation');
          setTimeout(() => {
            element.classList.remove('like-animation');
          }, 300);
        }
        
        // Handle rating updates
        if (element.hasAttribute('data-rating')) {
          const rating = parseFloat(element.dataset.rating);
          updateStarRating(element, rating);
        }
      }
    }
  });
}

// Helper functions
function initializeNewContent(container) {
  // Initialize any new components in the swapped content
  
  // Form validation
  container.querySelectorAll('form[data-validate]').forEach(form => {
    initializeFormValidation(form);
  });
  
  // Image lazy loading
  container.querySelectorAll('img[data-lazy]').forEach(img => {
    initializeLazyLoading(img);
  });
  
  // Tooltips
  container.querySelectorAll('[data-tooltip]').forEach(element => {
    initializeTooltip(element);
  });
  
  // Dropdowns
  container.querySelectorAll('[data-dropdown]').forEach(dropdown => {
    initializeDropdown(dropdown);
  });
  
  // File uploads
  container.querySelectorAll('input[type="file"][data-enhanced]').forEach(input => {
    initializeFileUpload(input);
  });
  
  console.log('Reinitialized components in new content:', container);
}

function showNotification(message, type = 'info') {
  const notification = document.createElement('div');
  notification.className = `notification notification-${type}`;
  notification.innerHTML = `
    <div class="notification-content">
      <span class="notification-message">${message}</span>
      <button class="notification-close" onclick="this.parentElement.parentElement.remove()">&times;</button>
    </div>
  `;
  
  // Add to notification container or body
  const container = document.querySelector('#notifications') || document.body;
  container.appendChild(notification);
  
  // Auto-remove after 5 seconds
  setTimeout(() => {
    if (notification.parentElement) {
      notification.remove();
    }
  }, 5000);
  
  // Animate in
  requestAnimationFrame(() => {
    notification.classList.add('notification-show');
  });
}

function initializeFormValidation(form) {
  form.addEventListener('submit', (event) => {
    const isValid = validateForm(form);
    if (!isValid) {
      event.preventDefault();
      showNotification('Please fix the form errors', 'error');
    }
  });
}

function validateForm(form) {
  let isValid = true;
  
  form.querySelectorAll('input[required], select[required], textarea[required]').forEach(field => {
    if (!field.value.trim()) {
      field.classList.add('error');
      isValid = false;
    } else {
      field.classList.remove('error');
    }
  });
  
  return isValid;
}

function initializeLazyLoading(img) {
  if ('IntersectionObserver' in window) {
    const observer = new IntersectionObserver((entries) => {
      entries.forEach(entry => {
        if (entry.isIntersecting) {
          const img = entry.target;
          img.src = img.dataset.lazy;
          img.classList.add('loaded');
          observer.unobserve(img);
        }
      });
    });
    
    observer.observe(img);
  } else {
    // Fallback for browsers without IntersectionObserver
    img.src = img.dataset.lazy;
    img.classList.add('loaded');
  }
}

function updateStarRating(element, rating) {
  const stars = element.querySelectorAll('.star');
  stars.forEach((star, index) => {
    if (index < Math.floor(rating)) {
      star.classList.add('filled');
    } else if (index < rating) {
      star.classList.add('half-filled');
    } else {
      star.classList.remove('filled', 'half-filled');
    }
  });
}

function initializeTooltip(element) {
  // Simple tooltip implementation
  element.addEventListener('mouseenter', () => {
    const tooltip = document.createElement('div');
    tooltip.className = 'tooltip';
    tooltip.textContent = element.getAttribute('data-tooltip');
    document.body.appendChild(tooltip);
    
    const rect = element.getBoundingClientRect();
    tooltip.style.left = rect.left + (rect.width / 2) - (tooltip.offsetWidth / 2) + 'px';
    tooltip.style.top = rect.top - tooltip.offsetHeight - 5 + 'px';
    
    element._tooltip = tooltip;
  });
  
  element.addEventListener('mouseleave', () => {
    if (element._tooltip) {
      element._tooltip.remove();
      element._tooltip = null;
    }
  });
}

function initializeDropdown(dropdown) {
  const trigger = dropdown.querySelector('[data-dropdown-trigger]');
  const menu = dropdown.querySelector('[data-dropdown-menu]');
  
  if (trigger && menu) {
    trigger.addEventListener('click', () => {
      dropdown.classList.toggle('open');
    });
    
    // Close when clicking outside
    document.addEventListener('click', (event) => {
      if (!dropdown.contains(event.target)) {
        dropdown.classList.remove('open');
      }
    });
  }
}

function initializeFileUpload(input) {
  const wrapper = document.createElement('div');
  wrapper.className = 'file-upload-wrapper';
  
  const preview = document.createElement('div');
  preview.className = 'file-preview';
  
  input.parentNode.insertBefore(wrapper, input);
  wrapper.appendChild(input);
  wrapper.appendChild(preview);
  
  input.addEventListener('change', (event) => {
    const files = Array.from(event.target.files);
    preview.innerHTML = '';
    
    files.forEach(file => {
      const item = document.createElement('div');
      item.className = 'file-item';
      item.innerHTML = `
        <span class="file-name">${file.name}</span>
        <span class="file-size">${formatFileSize(file.size)}</span>
      `;
      preview.appendChild(item);
    });
  });
}

function formatFileSize(bytes) {
  if (bytes === 0) return '0 Bytes';
  
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

export { showNotification, initializeNewContent };