/* Alchemorsel v3 - Enhanced Frontend Application */
(function() {
    'use strict';

    // Performance measurement for 14KB optimization validation
    const performanceMetrics = {
        navigationStart: performance.timing.navigationStart,
        firstPaint: 0,
        firstContentfulPaint: 0,
        domContentLoaded: 0,
        loadComplete: 0
    };

    // Register service worker for offline support
    if ('serviceWorker' in navigator) {
        window.addEventListener('load', function() {
            navigator.serviceWorker.register('/sw.js')
                .then(function(registration) {
                    console.log('ServiceWorker registered: ', registration.scope);
                })
                .catch(function(error) {
                    console.log('ServiceWorker registration failed: ', error);
                });
        });
    }

    // Progressive enhancement for HTMX
    document.addEventListener('DOMContentLoaded', function() {
        performanceMetrics.domContentLoaded = performance.now();

        // Initialize components
        initializeSearch();
        initializeVoiceInterface();
        initializeLazyLoading();
        initializeFormEnhancements();
        initializeAccessibility();
        
        // Measure performance
        measurePerformance();
    });

    // Enhanced real-time search with debouncing
    function initializeSearch() {
        const searchInput = document.querySelector('#search-input');
        if (!searchInput) return;

        let debounceTimer;
        searchInput.addEventListener('input', function(e) {
            clearTimeout(debounceTimer);
            debounceTimer = setTimeout(function() {
                const query = e.target.value;
                if (query.length > 2) {
                    htmx.ajax('POST', '/htmx/recipes/search', '#search-results');
                }
            }, 300);
        });
    }

    // Voice interface for AI chat
    function initializeVoiceInterface() {
        if (!('webkitSpeechRecognition' in window) && !('SpeechRecognition' in window)) {
            return; // Speech recognition not supported
        }

        const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition;
        const recognition = new SpeechRecognition();
        recognition.continuous = false;
        recognition.interimResults = false;
        recognition.lang = 'en-US';

        const voiceButton = document.querySelector('#voice-button');
        if (!voiceButton) return;

        voiceButton.addEventListener('click', function() {
            recognition.start();
            voiceButton.classList.add('recording');
            voiceButton.innerHTML = '<span class="spinner"></span> Listening...';
        });

        recognition.onresult = function(event) {
            const transcript = event.results[0][0].transcript;
            const messageInput = document.querySelector('#chat-message');
            if (messageInput) {
                messageInput.value = transcript;
                // Auto-submit the message
                htmx.trigger(messageInput.closest('form'), 'submit');
            }
        };

        recognition.onend = function() {
            voiceButton.classList.remove('recording');
            voiceButton.innerHTML = '🎤 Voice';
        };

        recognition.onerror = function(event) {
            console.error('Speech recognition error:', event.error);
            voiceButton.classList.remove('recording');
            voiceButton.innerHTML = '🎤 Voice';
        };
    }

    // Lazy loading for non-critical images
    function initializeLazyLoading() {
        if ('IntersectionObserver' in window) {
            const imageObserver = new IntersectionObserver(function(entries, observer) {
                entries.forEach(function(entry) {
                    if (entry.isIntersecting) {
                        const img = entry.target;
                        img.src = img.dataset.src;
                        img.classList.remove('lazy-load');
                        img.classList.add('loaded');
                        observer.unobserve(img);
                    }
                });
            });

            document.querySelectorAll('img[data-src]').forEach(function(img) {
                imageObserver.observe(img);
            });
        } else {
            // Fallback for older browsers
            document.querySelectorAll('img[data-src]').forEach(function(img) {
                img.src = img.dataset.src;
                img.classList.add('loaded');
            });
        }
    }

    // Form enhancements
    function initializeFormEnhancements() {
        // Dynamic ingredient/instruction forms
        document.addEventListener('click', function(e) {
            if (e.target.matches('.add-ingredient')) {
                e.preventDefault();
                const container = e.target.closest('.ingredients-container');
                const count = container.querySelectorAll('.ingredient-input').length + 1;
                htmx.ajax('GET', `/htmx/forms/ingredients/${count}`, '#ingredients-container');
            }

            if (e.target.matches('.add-instruction')) {
                e.preventDefault();
                const container = e.target.closest('.instructions-container');
                const count = container.querySelectorAll('.instruction-input').length + 1;
                htmx.ajax('GET', `/htmx/forms/instructions/${count}`, '#instructions-container');
            }
        });

        // Form validation enhancement
        document.querySelectorAll('form[data-validate]').forEach(function(form) {
            form.addEventListener('submit', function(e) {
                let isValid = true;
                const requiredFields = form.querySelectorAll('[required]');
                
                requiredFields.forEach(function(field) {
                    if (!field.value.trim()) {
                        field.classList.add('error');
                        isValid = false;
                    } else {
                        field.classList.remove('error');
                    }
                });

                if (!isValid) {
                    e.preventDefault();
                    showNotification('Please fill in all required fields', 'error');
                }
            });
        });
    }

    // Accessibility enhancements
    function initializeAccessibility() {
        // ARIA live region announcements for HTMX updates
        const announcer = document.getElementById('announcer');
        if (!announcer) {
            const div = document.createElement('div');
            div.id = 'announcer';
            div.setAttribute('aria-live', 'polite');
            div.setAttribute('aria-atomic', 'true');
            div.className = 'sr-only';
            document.body.appendChild(div);
        }

        // Enhanced keyboard navigation
        document.addEventListener('keydown', function(e) {
            // Escape key closes modals/dropdowns
            if (e.key === 'Escape') {
                document.querySelectorAll('.modal.open, .dropdown.open').forEach(function(el) {
                    el.classList.remove('open');
                });
            }

            // Enter key activates buttons
            if (e.key === 'Enter' && e.target.matches('button:not([type="submit"])')) {
                e.target.click();
            }
        });

        // Focus management for HTMX updates
        document.addEventListener('htmx:afterSwap', function(e) {
            const target = e.detail.target;
            const focusTarget = target.querySelector('[autofocus]') || target.querySelector('input, button, select, textarea');
            if (focusTarget) {
                focusTarget.focus();
            }
        });
    }

    // Performance measurement and reporting
    function measurePerformance() {
        window.addEventListener('load', function() {
            performanceMetrics.loadComplete = performance.now();

            // Get paint timings
            const paintEntries = performance.getEntriesByType('paint');
            paintEntries.forEach(function(entry) {
                if (entry.name === 'first-paint') {
                    performanceMetrics.firstPaint = entry.startTime;
                } else if (entry.name === 'first-contentful-paint') {
                    performanceMetrics.firstContentfulPaint = entry.startTime;
                }
            });

            // Calculate resource size
            const resourceSize = calculateResourceSize();
            
            // Report metrics
            console.log('Alchemorsel Performance Metrics:', {
                ...performanceMetrics,
                resourceSize: resourceSize,
                firstPacketOptimization: resourceSize.critical <= 14336 // 14KB in bytes
            });

            // Send metrics to server (optional)
            if (window.location.search.includes('debug=performance')) {
                fetch('/performance', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(performanceMetrics)
                });
            }
        });
    }

    // Calculate resource sizes for 14KB validation
    function calculateResourceSize() {
        const resources = performance.getEntriesByType('resource');
        let critical = 0;
        let total = 0;

        resources.forEach(function(resource) {
            const size = resource.transferSize || resource.encodedBodySize || 0;
            total += size;

            // Critical resources for first packet
            if (resource.name.includes('critical.css') || 
                resource.name.includes('htmx.min.js') ||
                resource.name === location.href) {
                critical += size;
            }
        });

        return {
            critical: critical,
            total: total,
            criticalKB: Math.round(critical / 1024 * 100) / 100,
            totalKB: Math.round(total / 1024 * 100) / 100
        };
    }

    // Utility function for notifications
    function showNotification(message, type = 'info') {
        const notification = document.createElement('div');
        notification.className = `alert alert-${type}`;
        notification.textContent = message;
        
        const container = document.querySelector('.notifications-container') || document.body;
        container.appendChild(notification);

        // Auto-remove after 5 seconds
        setTimeout(function() {
            notification.remove();
        }, 5000);

        // Announce to screen readers
        const announcer = document.getElementById('announcer');
        if (announcer) {
            announcer.textContent = message;
        }
    }

    // Real-time features with Server-Sent Events
    function initializeRealTimeFeatures() {
        if (typeof EventSource !== "undefined") {
            const eventSource = new EventSource('/htmx/notifications');
            
            eventSource.onmessage = function(event) {
                const data = JSON.parse(event.data);
                showNotification(data.message, data.type);
            };

            eventSource.onerror = function(event) {
                console.error('SSE connection error:', event);
            };
        }
    }

    // HTMX event handlers
    document.addEventListener('htmx:beforeRequest', function(e) {
        e.target.classList.add('htmx-loading');
        const indicator = e.target.querySelector('.htmx-indicator');
        if (indicator) {
            indicator.style.display = 'inline-block';
        }
    });

    document.addEventListener('htmx:afterRequest', function(e) {
        e.target.classList.remove('htmx-loading');
        const indicator = e.target.querySelector('.htmx-indicator');
        if (indicator) {
            indicator.style.display = 'none';
        }
    });

    document.addEventListener('htmx:responseError', function(e) {
        showNotification('An error occurred. Please try again.', 'error');
    });

    // Export utilities for global access
    window.AlchemorselApp = {
        showNotification: showNotification,
        performanceMetrics: performanceMetrics,
        measurePerformance: measurePerformance
    };

})();