/* Alchemorsel Application JavaScript */

// Initialize application when DOM is loaded
document.addEventListener('DOMContentLoaded', function() {
    console.log('Alchemorsel v3 initialized');
    
    // Initialize HTMX if available
    if (typeof htmx !== 'undefined') {
        htmx.process(document.body);
        console.log('HTMX processed');
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