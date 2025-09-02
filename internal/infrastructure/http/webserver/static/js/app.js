/* Alchemorsel Application JavaScript */

// Initialize application when DOM is loaded
document.addEventListener('DOMContentLoaded', function() {
    console.log('Alchemorsel v3 initialized');
    
    // Wait a bit for HTMX to fully load before initializing
    setTimeout(initializeHTMX, 100);
    
    function initializeHTMX() {
        console.log('Checking HTMX availability...', {
            htmx: typeof htmx,
            htmxProcess: typeof htmx !== 'undefined' ? typeof htmx.process : 'undefined',
            window: typeof window !== 'undefined',
            windowHtmx: typeof window.htmx,
            htmxKeys: typeof htmx !== 'undefined' ? Object.keys(htmx) : 'undefined',
            htmxVersion: typeof htmx !== 'undefined' ? htmx.version : 'undefined'
        });
        
        if (typeof htmx !== 'undefined' && htmx) {
            // HTMX is loaded and ready
            // Modern HTMX auto-processes, no need to call process() manually
            console.log('HTMX initialized successfully');
            
            // Add HTMX error handling for chat functionality
            document.body.addEventListener('htmx:sendError', function(event) {
                console.error('HTMX request failed:', event.detail);
                showChatError('Network error. Please check your connection and try again.');
            });
            
            document.body.addEventListener('htmx:responseError', function(event) {
                console.error('HTMX response error:', event.detail);
                if (event.detail.xhr.status === 401) {
                    showChatError('Please log in to use AI chat features.');
                } else {
                    showChatError('Something went wrong. Please try again.');
                }
            });
            
        } else {
            // HTMX not loaded, try listening for HTMX load event
            console.warn('HTMX not ready, retrying... attempt:', initializeHTMX.attempts || 0);
            initializeHTMX.attempts = (initializeHTMX.attempts || 0) + 1;
            
            // Try listening for htmx:load event
            if (initializeHTMX.attempts === 1) {
                document.addEventListener('htmx:load', function() {
                    console.log('HTMX load event received');
                    initializeHTMX();
                });
            }
            
            // Stop after 50 attempts (5 seconds)
            if (initializeHTMX.attempts < 50) {
                setTimeout(initializeHTMX, 100);
            } else {
                console.error('HTMX failed to initialize after 5 seconds. Forms will fall back to standard submission.');
            }
        }
    }
    
    // Show error messages in chat
    function showChatError(message) {
        const chatContainer = document.getElementById('chat-container');
        if (chatContainer) {
            const errorDiv = document.createElement('div');
            errorDiv.className = 'chat-message error-message';
            errorDiv.style.cssText = 'margin-bottom: 1rem; padding: 1rem; background: #fee2e2; border: 1px solid #fecaca; border-radius: 0.5rem; color: #dc2626;';
            errorDiv.innerHTML = `<strong>Error:</strong> ${message}`;
            chatContainer.appendChild(errorDiv);
            chatContainer.scrollTop = chatContainer.scrollHeight;
        }
    }
    
    // Mobile menu functionality
    const menuButton = document.querySelector('.mobile-menu-button');
    const mobileMenu = document.querySelector('.mobile-menu');
    
    if (menuButton && mobileMenu) {
        menuButton.addEventListener('click', function() {
            const isOpen = mobileMenu.style.display === 'block';
            mobileMenu.style.display = isOpen ? 'none' : 'block';
            menuButton.setAttribute('aria-expanded', !isOpen);
            menuButton.setAttribute('aria-label', isOpen ? 'Open main menu' : 'Close main menu');
        });
    }
    
    // Form validation helpers
    window.validateForm = function(form) {
        const requiredFields = form.querySelectorAll('[required]');
        let isValid = true;
        
        requiredFields.forEach(function(field) {
            if (!field.value.trim()) {
                field.classList.add('error');
                isValid = false;
            } else {
                field.classList.remove('error');
            }
        });
        
        return isValid;
    };
    
    // Auto-resize textareas
    document.querySelectorAll('textarea[data-auto-resize]').forEach(function(textarea) {
        function resize() {
            textarea.style.height = 'auto';
            textarea.style.height = textarea.scrollHeight + 'px';
        }
        textarea.addEventListener('input', resize);
        resize(); // Initial resize
    });
});

// Global error handler
window.addEventListener('error', function(e) {
    console.error('Application error:', e.error);
});

// Performance monitoring
if (window.performanceStart) {
    window.addEventListener('load', function() {
        const loadTime = performance.now() - window.performanceStart;
        console.log('Page loaded in:', loadTime.toFixed(2), 'ms');
        
        // Send performance data if analytics is available
        if (typeof gtag !== 'undefined') {
            gtag('event', 'page_load_time', {
                value: Math.round(loadTime),
                custom_parameter: 'alchemorsel_v3'
            });
        }
    });
}