/* Service Worker for Alchemorsel v3 */

const CACHE_NAME = 'alchemorsel-v3-1.0.0';
const STATIC_CACHE_URLS = [
    '/',
    '/static/css/extended.css',
    '/static/css/accessibility.css',
    '/static/js/htmx.min.js',
    '/static/js/app.js',
    '/static/js/performance.js',
    '/static/js/accessibility.js'
];

// Install event - cache static assets
self.addEventListener('install', function(event) {
    console.log('Service Worker installing...');
    
    event.waitUntil(
        caches.open(CACHE_NAME).then(function(cache) {
            console.log('Caching static assets');
            return cache.addAll(STATIC_CACHE_URLS.map(url => new Request(url, {
                cache: 'reload'
            })));
        }).catch(function(error) {
            console.error('Failed to cache static assets:', error);
        })
    );
    
    // Force immediate activation
    self.skipWaiting();
});

// Activate event - clean up old caches
self.addEventListener('activate', function(event) {
    console.log('Service Worker activating...');
    
    event.waitUntil(
        caches.keys().then(function(cacheNames) {
            return Promise.all(
                cacheNames.map(function(cacheName) {
                    if (cacheName !== CACHE_NAME) {
                        console.log('Deleting old cache:', cacheName);
                        return caches.delete(cacheName);
                    }
                })
            );
        }).then(function() {
            // Take control of all pages immediately
            return self.clients.claim();
        })
    );
});

// Fetch event - serve from cache with network fallback
self.addEventListener('fetch', function(event) {
    // Skip non-GET requests
    if (event.request.method !== 'GET') {
        return;
    }
    
    // Skip requests with cache-control: no-cache
    if (event.request.headers.get('cache-control') === 'no-cache') {
        return;
    }
    
    event.respondWith(
        caches.match(event.request).then(function(cachedResponse) {
            // Return cached version if available
            if (cachedResponse) {
                console.log('Serving from cache:', event.request.url);
                return cachedResponse;
            }
            
            // Otherwise fetch from network
            return fetch(event.request).then(function(response) {
                // Only cache successful responses
                if (response.status === 200) {
                    const responseClone = response.clone();
                    
                    // Cache static assets and API responses
                    if (event.request.url.includes('/static/') || 
                        event.request.url.includes('/api/')) {
                        caches.open(CACHE_NAME).then(function(cache) {
                            cache.put(event.request, responseClone);
                        });
                    }
                }
                
                return response;
            }).catch(function(error) {
                console.error('Fetch failed:', error);
                
                // Return offline page for navigation requests
                if (event.request.mode === 'navigate') {
                    return caches.match('/').then(function(response) {
                        return response || new Response('Offline - Please check your connection', {
                            status: 503,
                            statusText: 'Service Unavailable',
                            headers: { 'Content-Type': 'text/plain' }
                        });
                    });
                }
                
                throw error;
            });
        })
    );
});

// Background sync for offline actions
self.addEventListener('sync', function(event) {
    console.log('Background sync triggered:', event.tag);
    
    if (event.tag === 'sync-recipes') {
        event.waitUntil(syncRecipes());
    }
});

function syncRecipes() {
    // Sync offline recipe data when connection is restored
    return new Promise(function(resolve) {
        console.log('Syncing recipes...');
        // Implementation would sync any offline changes
        resolve();
    });
}

// Push notifications (for future enhancement)
self.addEventListener('push', function(event) {
    console.log('Push notification received');
    
    const options = {
        body: event.data ? event.data.text() : 'New recipe available!',
        icon: '/static/img/icon-192.png',
        badge: '/static/img/badge-72.png',
        vibrate: [200, 100, 200],
        data: {
            url: '/'
        },
        actions: [
            {
                action: 'view',
                title: 'View Recipe',
                icon: '/static/img/view-icon.png'
            },
            {
                action: 'close',
                title: 'Close',
                icon: '/static/img/close-icon.png'
            }
        ]
    };
    
    event.waitUntil(
        self.registration.showNotification('Alchemorsel', options)
    );
});

// Handle notification clicks
self.addEventListener('notificationclick', function(event) {
    event.notification.close();
    
    if (event.action === 'view') {
        event.waitUntil(
            clients.openWindow(event.notification.data.url)
        );
    }
});

// Handle messages from main thread
self.addEventListener('message', function(event) {
    console.log('Service Worker received message:', event.data);
    
    if (event.data && event.data.type === 'CACHE_UPDATE') {
        // Update cache with new content
        event.waitUntil(
            caches.open(CACHE_NAME).then(function(cache) {
                return cache.addAll(event.data.urls);
            })
        );
    }
});

console.log('Alchemorsel Service Worker loaded');