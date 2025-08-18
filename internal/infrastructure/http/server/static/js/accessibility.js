/* Alchemorsel v3 - Enhanced Accessibility Features */
(function() {
    'use strict';

    // Accessibility enhancement controller
    const A11yController = {
        init() {
            this.setupKeyboardNavigation();
            this.setupScreenReaderSupport();
            this.setupFocusManagement();
            this.setupHighContrastMode();
            this.setupReducedMotion();
            this.setupAccessibilityToolbar();
            this.setupHTMXAccessibility();
        },

        // Enhanced keyboard navigation
        setupKeyboardNavigation() {
            document.addEventListener('keydown', (e) => {
                // Escape key handling
                if (e.key === 'Escape') {
                    this.handleEscapeKey();
                }

                // Skip to main content with Ctrl/Cmd + /
                if ((e.ctrlKey || e.metaKey) && e.key === '/') {
                    e.preventDefault();
                    this.skipToMainContent();
                }

                // Navigate cards with arrow keys
                if (e.target.classList.contains('recipe-card') || e.target.closest('.recipe-card')) {
                    this.handleCardNavigation(e);
                }

                // Handle form navigation
                if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') {
                    this.handleFormNavigation(e);
                }
            });

            // Add keyboard indicators
            document.addEventListener('keydown', (e) => {
                if (e.key === 'Tab') {
                    document.body.classList.add('keyboard-navigation');
                }
            });

            document.addEventListener('mousedown', () => {
                document.body.classList.remove('keyboard-navigation');
            });
        },

        // Screen reader support
        setupScreenReaderSupport() {
            // Ensure announcer element exists
            let announcer = document.getElementById('announcer');
            if (!announcer) {
                announcer = document.createElement('div');
                announcer.id = 'announcer';
                announcer.setAttribute('aria-live', 'polite');
                announcer.setAttribute('aria-atomic', 'true');
                announcer.className = 'sr-only';
                document.body.appendChild(announcer);
            }

            // Announce page changes
            this.announcePageChange();

            // Announce HTMX updates
            document.addEventListener('htmx:afterSwap', (e) => {
                const announcement = e.detail.target.getAttribute('data-announce');
                if (announcement) {
                    this.announce(announcement);
                }
            });

            // Add landmarks if missing
            this.ensureLandmarks();
        },

        // Focus management for SPA-like behavior
        setupFocusManagement() {
            // Store focus before HTMX requests
            document.addEventListener('htmx:beforeRequest', (e) => {
                this.lastFocusedElement = document.activeElement;
            });

            // Restore or manage focus after HTMX updates
            document.addEventListener('htmx:afterSwap', (e) => {
                this.manageFocusAfterUpdate(e.detail.target);
            });

            // Focus trap for modal dialogs
            document.addEventListener('click', (e) => {
                if (e.target.matches('[data-modal-open]')) {
                    const modalId = e.target.getAttribute('data-modal-open');
                    this.trapFocus(modalId);
                }
            });
        },

        // High contrast mode support
        setupHighContrastMode() {
            // Detect high contrast preference
            if (window.matchMedia && window.matchMedia('(prefers-contrast: high)').matches) {
                document.body.classList.add('high-contrast');
            }

            // Listen for changes
            window.matchMedia('(prefers-contrast: high)').addEventListener('change', (e) => {
                document.body.classList.toggle('high-contrast', e.matches);
            });

            // Manual toggle
            const contrastToggle = document.getElementById('contrast-toggle');
            if (contrastToggle) {
                contrastToggle.addEventListener('click', () => {
                    document.body.classList.toggle('high-contrast');
                    this.announce('High contrast mode ' + 
                        (document.body.classList.contains('high-contrast') ? 'enabled' : 'disabled'));
                });
            }
        },

        // Reduced motion support
        setupReducedMotion() {
            if (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
                document.body.classList.add('reduced-motion');
                
                // Disable auto-playing animations
                document.querySelectorAll('.recipe-card').forEach(card => {
                    card.style.transition = 'none';
                });
            }
        },

        // Accessibility toolbar
        setupAccessibilityToolbar() {
            const toolbar = this.createAccessibilityToolbar();
            document.body.appendChild(toolbar);
        },

        // HTMX-specific accessibility enhancements
        setupHTMXAccessibility() {
            // Add loading announcements
            document.addEventListener('htmx:beforeRequest', (e) => {
                const loadingMessage = e.target.getAttribute('data-loading-message') || 'Loading...';
                this.announce(loadingMessage);
            });

            // Add completion announcements
            document.addEventListener('htmx:afterRequest', (e) => {
                const completeMessage = e.target.getAttribute('data-complete-message') || 'Content updated';
                this.announce(completeMessage);
            });

            // Error announcements
            document.addEventListener('htmx:responseError', (e) => {
                this.announce('An error occurred. Please try again or contact support.');
            });
        },

        // Helper methods
        handleEscapeKey() {
            // Close modals
            const openModals = document.querySelectorAll('.modal.open, [aria-expanded="true"]');
            openModals.forEach(modal => {
                modal.classList.remove('open');
                modal.setAttribute('aria-expanded', 'false');
            });

            // Clear search
            const searchInput = document.getElementById('search-input');
            if (searchInput && searchInput === document.activeElement) {
                searchInput.value = '';
                searchInput.blur();
            }
        },

        skipToMainContent() {
            const mainContent = document.getElementById('main-content') || document.querySelector('main');
            if (mainContent) {
                mainContent.setAttribute('tabindex', '-1');
                mainContent.focus();
                this.announce('Skipped to main content');
            }
        },

        handleCardNavigation(e) {
            const currentCard = e.target.closest('.recipe-card');
            const allCards = Array.from(document.querySelectorAll('.recipe-card'));
            const currentIndex = allCards.indexOf(currentCard);

            let targetIndex;
            switch (e.key) {
                case 'ArrowRight':
                case 'ArrowDown':
                    e.preventDefault();
                    targetIndex = (currentIndex + 1) % allCards.length;
                    break;
                case 'ArrowLeft':
                case 'ArrowUp':
                    e.preventDefault();
                    targetIndex = (currentIndex - 1 + allCards.length) % allCards.length;
                    break;
                case 'Home':
                    e.preventDefault();
                    targetIndex = 0;
                    break;
                case 'End':
                    e.preventDefault();
                    targetIndex = allCards.length - 1;
                    break;
                default:
                    return;
            }

            if (targetIndex !== undefined) {
                const targetCard = allCards[targetIndex];
                const focusTarget = targetCard.querySelector('a, button') || targetCard;
                focusTarget.focus();
                this.announce(`Recipe ${targetIndex + 1} of ${allCards.length}`);
            }
        },

        handleFormNavigation(e) {
            // Auto-submit search on Enter (if not already handled)
            if (e.key === 'Enter' && e.target.id === 'search-input') {
                const form = e.target.closest('form');
                if (form && form.hasAttribute('hx-post')) {
                    htmx.trigger(form, 'submit');
                }
            }
        },

        manageFocusAfterUpdate(target) {
            // Find appropriate focus target
            let focusTarget = target.querySelector('[autofocus]') ||
                            target.querySelector('input, button, select, textarea, a[href]') ||
                            target;

            // If it's a search results update, focus the first result
            if (target.id === 'search-results') {
                focusTarget = target.querySelector('.recipe-card a, .recipe-card button') || target;
            }

            // If it's a chat update, focus the input
            if (target.closest('#chat-container')) {
                focusTarget = document.getElementById('chat-message') || target;
            }

            if (focusTarget && focusTarget.focus) {
                focusTarget.focus();
            }
        },

        trapFocus(modalId) {
            const modal = document.getElementById(modalId);
            if (!modal) return;

            const focusableElements = modal.querySelectorAll(
                'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
            );

            if (focusableElements.length === 0) return;

            const firstElement = focusableElements[0];
            const lastElement = focusableElements[focusableElements.length - 1];

            firstElement.focus();

            modal.addEventListener('keydown', function trapHandler(e) {
                if (e.key !== 'Tab') return;

                if (e.shiftKey) {
                    if (document.activeElement === firstElement) {
                        e.preventDefault();
                        lastElement.focus();
                    }
                } else {
                    if (document.activeElement === lastElement) {
                        e.preventDefault();
                        firstElement.focus();
                    }
                }
            });
        },

        announcePageChange() {
            const title = document.title;
            setTimeout(() => {
                this.announce(`Page loaded: ${title}`);
            }, 100);
        },

        ensureLandmarks() {
            // Add main landmark if missing
            if (!document.querySelector('main')) {
                const mainContent = document.querySelector('.main');
                if (mainContent) {
                    mainContent.setAttribute('role', 'main');
                }
            }

            // Add navigation landmark if missing
            if (!document.querySelector('nav')) {
                const navigation = document.querySelector('.nav');
                if (navigation) {
                    navigation.setAttribute('role', 'navigation');
                    navigation.setAttribute('aria-label', 'Main navigation');
                }
            }
        },

        announce(message) {
            const announcer = document.getElementById('announcer');
            if (announcer) {
                announcer.textContent = message;
            }
        },

        createAccessibilityToolbar() {
            const toolbar = document.createElement('div');
            toolbar.className = 'accessibility-toolbar';
            toolbar.setAttribute('role', 'toolbar');
            toolbar.setAttribute('aria-label', 'Accessibility options');
            
            toolbar.innerHTML = `
                <button type="button" id="contrast-toggle" aria-label="Toggle high contrast mode">
                    🔆 Contrast
                </button>
                <button type="button" id="font-size-increase" aria-label="Increase font size">
                    A+ Larger
                </button>
                <button type="button" id="font-size-decrease" aria-label="Decrease font size">
                    A- Smaller
                </button>
                <button type="button" id="focus-highlight-toggle" aria-label="Toggle focus highlighting">
                    🎯 Focus
                </button>
            `;

            // Add toolbar functionality
            toolbar.addEventListener('click', (e) => {
                const button = e.target.closest('button');
                if (!button) return;

                switch (button.id) {
                    case 'font-size-increase':
                        this.adjustFontSize(1.1);
                        break;
                    case 'font-size-decrease':
                        this.adjustFontSize(0.9);
                        break;
                    case 'focus-highlight-toggle':
                        document.body.classList.toggle('enhanced-focus');
                        break;
                }
            });

            return toolbar;
        },

        adjustFontSize(multiplier) {
            const root = document.documentElement;
            const currentSize = parseFloat(getComputedStyle(root).fontSize);
            const newSize = currentSize * multiplier;
            
            // Limit font size between 12px and 24px
            if (newSize >= 12 && newSize <= 24) {
                root.style.fontSize = newSize + 'px';
                this.announce(`Font size ${multiplier > 1 ? 'increased' : 'decreased'}`);
            }
        }
    };

    // Initialize accessibility features
    document.addEventListener('DOMContentLoaded', () => {
        A11yController.init();
    });

    // Export for global access
    window.A11yController = A11yController;

})();