/* Accessibility Enhancements for Alchemorsel */

(function() {
    'use strict';
    
    // Accessibility initialization
    document.addEventListener('DOMContentLoaded', function() {
        initializeAccessibility();
        setupKeyboardNavigation();
        setupScreenReaderAnnouncements();
        setupFocusManagement();
    });
    
    function initializeAccessibility() {
        console.log('Initializing accessibility features');
        
        // Add skip links functionality
        const skipLinks = document.querySelectorAll('.skip-link');
        skipLinks.forEach(function(link) {
            link.addEventListener('click', function(e) {
                e.preventDefault();
                const target = document.querySelector(link.getAttribute('href'));
                if (target) {
                    target.focus();
                    target.scrollIntoView({ behavior: 'smooth' });
                }
            });
        });
        
        // Enhance form labels
        const inputs = document.querySelectorAll('input, textarea, select');
        inputs.forEach(function(input) {
            if (!input.getAttribute('aria-label') && !input.getAttribute('aria-labelledby')) {
                const label = document.querySelector('label[for="' + input.id + '"]');
                if (!label && input.id) {
                    console.warn('Missing label for input:', input.id);
                }
            }
        });
    }
    
    function setupKeyboardNavigation() {
        // Tab trap for modals
        document.addEventListener('keydown', function(e) {
            if (e.key === 'Escape') {
                // Close any open modals or dropdowns
                const openModals = document.querySelectorAll('.modal.open, .dropdown.open');
                openModals.forEach(function(modal) {
                    modal.classList.remove('open');
                });
            }
            
            // Handle Enter key on buttons with role="button"
            if (e.key === 'Enter' && e.target.getAttribute('role') === 'button') {
                e.target.click();
            }
        });
        
        // Improve focus visibility
        document.addEventListener('keydown', function(e) {
            if (e.key === 'Tab') {
                document.body.classList.add('keyboard-navigation');
            }
        });
        
        document.addEventListener('mousedown', function() {
            document.body.classList.remove('keyboard-navigation');
        });
    }
    
    function setupScreenReaderAnnouncements() {
        const announcer = document.getElementById('announcer');
        
        window.announceToScreenReader = function(message, priority) {
            if (!announcer) return;
            
            announcer.textContent = message;
            announcer.setAttribute('aria-live', priority || 'polite');
            
            // Clear after announcement
            setTimeout(function() {
                announcer.textContent = '';
            }, 1000);
        };
        
        // Announce navigation changes
        if ('history' in window && 'pushState' in window.history) {
            let lastUrl = location.href;
            new MutationObserver(function() {
                const url = location.href;
                if (url !== lastUrl) {
                    lastUrl = url;
                    const title = document.title;
                    announceToScreenReader('Navigated to ' + title);
                }
            }).observe(document, { subtree: true, childList: true });
        }
    }
    
    function setupFocusManagement() {
        // Restore focus after HTMX requests
        document.body.addEventListener('htmx:afterRequest', function(event) {
            const target = event.detail.target;
            if (target && target.querySelector) {
                const focusable = target.querySelector('[autofocus], input, button, select, textarea, [tabindex]:not([tabindex="-1"])');
                if (focusable) {
                    setTimeout(function() {
                        focusable.focus();
                    }, 100);
                }
            }
        });
        
        // Focus trap for chat interface
        const chatContainer = document.getElementById('chat-container');
        if (chatContainer) {
            chatContainer.addEventListener('keydown', function(e) {
                if (e.key === 'Tab' && e.shiftKey && e.target === chatContainer.querySelector('input, button')) {
                    // Handle reverse tab in chat
                }
            });
        }
    }
    
    // Utility functions for dynamic content
    window.AccessibilityUtils = {
        announce: window.announceToScreenReader,
        
        makeFocusable: function(element) {
            if (!element.hasAttribute('tabindex')) {
                element.setAttribute('tabindex', '0');
            }
        },
        
        addAriaLabel: function(element, label) {
            element.setAttribute('aria-label', label);
        },
        
        addAriaDescribedBy: function(element, describerId) {
            element.setAttribute('aria-describedby', describerId);
        }
    };
})();