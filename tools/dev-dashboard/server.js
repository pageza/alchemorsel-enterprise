// Alchemorsel v3 Development Dashboard Server
// Real-time monitoring and control interface for hot reload development

const express = require('express');
const http = require('http');
const socketIo = require('socket.io');
const axios = require('axios');
const cors = require('cors');
const helmet = require('helmet');
const morgan = require('morgan');
const compression = require('compression');
const path = require('path');

const app = express();
const server = http.createServer(app);
const io = socketIo(server, {
  cors: {
    origin: "*",
    methods: ["GET", "POST"]
  }
});

// Configuration
const config = {
  port: process.env.DASHBOARD_PORT || 3030,
  apiBaseUrl: process.env.API_BASE_URL || 'http://api-dev:8080',
  webBaseUrl: process.env.WEB_BASE_URL || 'http://web-dev:8081',
  proxyUrl: process.env.PROXY_URL || 'http://dev-proxy:80',
  liveReloadUrl: process.env.LIVERELOAD_URL || 'http://livereload:35729',
  refreshInterval: 5000, // 5 seconds
  healthCheckTimeout: 3000
};

// Middleware
app.use(helmet({
  contentSecurityPolicy: false // Relaxed for development
}));
app.use(cors());
app.use(compression());
app.use(morgan('dev'));
app.use(express.json());
app.use(express.urlencoded({ extended: true }));
app.use(express.static(path.join(__dirname, 'public')));

// View engine
app.set('view engine', 'ejs');
app.set('views', path.join(__dirname, 'views'));

// Global state
let dashboardState = {
  services: {
    api: { status: 'unknown', lastCheck: null, error: null, stats: null },
    web: { status: 'unknown', lastCheck: null, error: null, stats: null },
    postgres: { status: 'unknown', lastCheck: null, error: null },
    redis: { status: 'unknown', lastCheck: null, error: null },
    ollama: { status: 'unknown', lastCheck: null, error: null },
    livereload: { status: 'unknown', lastCheck: null, error: null },
    proxy: { status: 'unknown', lastCheck: null, error: null, stats: null }
  },
  metrics: {
    totalRequests: 0,
    errorRate: 0,
    avgResponseTime: 0,
    uptime: 0
  },
  logs: [],
  hotReload: {
    enabled: true,
    lastReload: null,
    reloadCount: 0,
    connectedClients: 0
  }
};

// Routes

// Main dashboard
app.get('/', (req, res) => {
  res.render('dashboard', {
    title: 'Alchemorsel v3 - Development Dashboard',
    config: config,
    state: dashboardState
  });
});

// API endpoints
app.get('/api/status', (req, res) => {
  res.json(dashboardState);
});

app.get('/api/services/:service/health', async (req, res) => {
  const { service } = req.params;
  try {
    const health = await checkServiceHealth(service);
    res.json(health);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

app.post('/api/services/:service/restart', async (req, res) => {
  const { service } = req.params;
  try {
    const result = await restartService(service);
    res.json({ success: true, message: `${service} restart initiated`, result });
  } catch (error) {
    res.status(500).json({ success: false, error: error.message });
  }
});

app.post('/api/hotreload/trigger', async (req, res) => {
  try {
    await triggerHotReload();
    res.json({ success: true, message: 'Hot reload triggered' });
  } catch (error) {
    res.status(500).json({ success: false, error: error.message });
  }
});

app.get('/api/logs', (req, res) => {
  const limit = parseInt(req.query.limit) || 100;
  const logs = dashboardState.logs.slice(-limit);
  res.json(logs);
});

// Service health checking
async function checkServiceHealth(serviceName) {
  const now = new Date();
  
  try {
    let response;
    let url;
    
    switch (serviceName) {
      case 'api':
        url = `${config.apiBaseUrl}/health`;
        break;
      case 'web':
        url = `${config.webBaseUrl}/health`;
        break;
      case 'proxy':
        url = `${config.proxyUrl}/dev-proxy/health`;
        break;
      case 'livereload':
        url = `${config.liveReloadUrl}/status`;
        break;
      default:
        throw new Error(`Unknown service: ${serviceName}`);
    }
    
    response = await axios.get(url, { timeout: config.healthCheckTimeout });
    
    const health = {
      status: 'healthy',
      lastCheck: now,
      error: null,
      response: response.data,
      responseTime: response.headers['x-response-time'] || 'unknown'
    };
    
    dashboardState.services[serviceName] = health;
    return health;
    
  } catch (error) {
    const health = {
      status: 'unhealthy',
      lastCheck: now,
      error: error.message,
      response: null
    };
    
    dashboardState.services[serviceName] = health;
    return health;
  }
}

// Service statistics collection
async function collectServiceStats(serviceName) {
  try {
    let statsUrl;
    
    switch (serviceName) {
      case 'api':
        statsUrl = `${config.apiBaseUrl}/metrics`;
        break;
      case 'proxy':
        statsUrl = `${config.proxyUrl}/dev-proxy/stats`;
        break;
      default:
        return null;
    }
    
    const response = await axios.get(statsUrl, { timeout: config.healthCheckTimeout });
    dashboardState.services[serviceName].stats = response.data;
    
    return response.data;
  } catch (error) {
    addLog('error', `Failed to collect stats for ${serviceName}: ${error.message}`);
    return null;
  }
}

// Service restart functionality
async function restartService(serviceName) {
  // In a real implementation, this would trigger Docker container restart
  // For now, we'll simulate it
  addLog('info', `Restart requested for service: ${serviceName}`);
  
  // Mark service as restarting
  dashboardState.services[serviceName].status = 'restarting';
  
  // Simulate restart delay
  setTimeout(() => {
    checkServiceHealth(serviceName);
  }, 3000);
  
  return { message: `${serviceName} restart initiated` };
}

// Hot reload trigger
async function triggerHotReload() {
  try {
    const response = await axios.post(`${config.liveReloadUrl}/trigger`, {
      path: 'manual',
      timestamp: Date.now()
    });
    
    dashboardState.hotReload.lastReload = new Date();
    dashboardState.hotReload.reloadCount++;
    
    addLog('info', 'Manual hot reload triggered');
    
    // Broadcast to connected clients
    io.emit('hot-reload-triggered', {
      timestamp: dashboardState.hotReload.lastReload,
      count: dashboardState.hotReload.reloadCount
    });
    
    return response.data;
  } catch (error) {
    addLog('error', `Failed to trigger hot reload: ${error.message}`);
    throw error;
  }
}

// Logging system
function addLog(level, message, metadata = {}) {
  const logEntry = {
    timestamp: new Date(),
    level,
    message,
    metadata
  };
  
  dashboardState.logs.push(logEntry);
  
  // Keep only last 1000 log entries
  if (dashboardState.logs.length > 1000) {
    dashboardState.logs = dashboardState.logs.slice(-1000);
  }
  
  // Emit to connected clients
  io.emit('new-log', logEntry);
  
  console.log(`[${logEntry.timestamp.toISOString()}] ${level.toUpperCase()}: ${message}`);
}

// Periodic health checks and stats collection
function startMonitoring() {
  const monitoringInterval = setInterval(async () => {
    const services = ['api', 'web', 'proxy', 'livereload'];
    
    for (const service of services) {
      await checkServiceHealth(service);
      await collectServiceStats(service);
    }
    
    // Update metrics
    updateDashboardMetrics();
    
    // Broadcast updates to connected clients
    io.emit('dashboard-update', dashboardState);
    
  }, config.refreshInterval);
  
  // Clean up on exit
  process.on('SIGINT', () => {
    clearInterval(monitoringInterval);
    process.exit(0);
  });
}

// Update dashboard metrics
function updateDashboardMetrics() {
  const proxyStats = dashboardState.services.proxy.stats;
  
  if (proxyStats && proxyStats.requests) {
    dashboardState.metrics.totalRequests = proxyStats.requests.total;
    dashboardState.metrics.errorRate = proxyStats.requests.errors > 0 
      ? (proxyStats.requests.errors / proxyStats.requests.total * 100).toFixed(2)
      : 0;
    dashboardState.metrics.avgResponseTime = proxyStats.timing?.avg_response_time_ms || 0;
  }
  
  // Calculate uptime from first service
  const apiService = dashboardState.services.api;
  if (apiService.stats && apiService.stats.timing?.uptime) {
    dashboardState.metrics.uptime = apiService.stats.timing.uptime;
  }
}

// WebSocket connection handling
io.on('connection', (socket) => {
  console.log('Dashboard client connected:', socket.id);
  
  // Send current state to new client
  socket.emit('dashboard-update', dashboardState);
  
  // Handle client requests
  socket.on('request-service-restart', async (serviceName) => {
    try {
      await restartService(serviceName);
      socket.emit('service-restart-result', { success: true, service: serviceName });
    } catch (error) {
      socket.emit('service-restart-result', { success: false, error: error.message });
    }
  });
  
  socket.on('request-hot-reload', async () => {
    try {
      await triggerHotReload();
      socket.emit('hot-reload-result', { success: true });
    } catch (error) {
      socket.emit('hot-reload-result', { success: false, error: error.message });
    }
  });
  
  socket.on('disconnect', () => {
    console.log('Dashboard client disconnected:', socket.id);
  });
});

// Start server
server.listen(config.port, () => {
  console.log(`🚀 Alchemorsel v3 Development Dashboard started on port ${config.port}`);
  console.log(`📊 Dashboard: http://localhost:${config.port}`);
  console.log(`🔧 Monitoring services every ${config.refreshInterval}ms`);
  
  addLog('info', 'Development dashboard started', { port: config.port });
  
  // Start monitoring
  startMonitoring();
  
  // Initial health check
  setTimeout(() => {
    const services = ['api', 'web', 'proxy', 'livereload'];
    services.forEach(service => checkServiceHealth(service));
  }, 2000);
});

// Graceful shutdown
process.on('SIGTERM', () => {
  console.log('Shutting down dashboard server...');
  server.close(() => {
    console.log('Dashboard server stopped');
    process.exit(0);
  });
});

module.exports = app;