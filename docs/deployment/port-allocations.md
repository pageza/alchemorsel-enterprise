# Alchemorsel v3 Port Allocations

**Document**: Port Allocation Registry  
**Version**: 1.0  
**Date**: August 19, 2025  
**Status**: Active  

## Overview

This document maintains the official port allocation registry for Alchemorsel v3 as per ADR-0005 (portscan Port Allocation). All ports are allocated through the `portscan` system to prevent conflicts and ensure proper resource management.

## Allocated Ports

### Development Environment Ports

| Port | Service | Description | Allocation ID | Expires |
|------|---------|-------------|---------------|---------|
| 3010 | alchemorsel-api | Alchemorsel v3 API Server | 90f48b05 | 2025-08-20T19:09:52 |
| 3011 | alchemorsel-web | Alchemorsel v3 Web Server | c3287854 | 2025-08-20T19:09:55 |
| 3012 | alchemorsel-metrics | Alchemorsel v3 Metrics/Monitoring | b717e38a | 2025-08-20T19:09:58 |

### Infrastructure Ports (Standard)

| Port | Service | Description | Status |
|------|---------|-------------|--------|
| 5432 | postgres | PostgreSQL Database | Standard |
| 6379 | redis | Redis Cache | Standard |
| 9090 | prometheus | Prometheus Metrics | Standard |
| 3013 | grafana | Grafana Dashboard | Adjusted to avoid conflicts |
| 16686 | jaeger | Jaeger UI | Standard |
| 14268 | jaeger-http | Jaeger HTTP | Standard |

## Docker Compose Configuration

The port allocations are reflected in the following Docker Compose files:

- **`docker-compose.services.yml`**: Main service configuration
  - API service: `3010:3010` (portscan allocated)
  - Web service: `3011:3011` (portscan allocated)  
  - Metrics endpoint: `3012:3012` (portscan allocated)

## Service Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Alchemorsel v3 Services                   │
├─────────────────────────────────────────────────────────────┤
│  Frontend (HTMX)     │  API Backend        │  Metrics       │
│  Port: 3011         │  Port: 3010         │  Port: 3012    │
│  (Web Interface)    │  (JSON API)         │  (Prometheus)  │
└─────────────────────┴─────────────────────┴─────────────────┘
                               │
                    ┌─────────────────────┐
                    │   Infrastructure    │
                    │  PostgreSQL: 5432   │
                    │  Redis: 6379        │
                    │  Prometheus: 9090   │
                    │  Grafana: 3013      │
                    └─────────────────────┘
```

## Port Management Commands

### Allocate New Port
```bash
portscan ports allocate <service-name> "<description>" development <port>
```

### List Current Allocations
```bash
portscan ports list
```

### Check Available Ports
```bash
portscan ports suggest development <count>
```

## Configuration Updates Made

### Docker Compose Services (`docker-compose.services.yml`)

1. **API Service Updates**:
   - Changed `PORT: 3000` → `PORT: 3010`
   - Added `ALCHEMORSEL_MONITORING_METRICS_PORT: 3012`
   - Updated port mapping: `"3000:3000"` → `"3010:3010"`
   - Added metrics port mapping: `"3012:3012"`
   - Updated health check URL: `localhost:3000` → `localhost:3010`

2. **Web Service Updates**:
   - Changed `PORT: 8080` → `PORT: 3011`
   - Updated API connection: `http://api:3000` → `http://api:3010`
   - Updated port mapping: `"8080:8080"` → `"3011:3011"`
   - Updated health check URL: `localhost:8080` → `localhost:3011`

3. **Grafana Service Updates**:
   - Updated port mapping: `"3001:3000"` → `"3013:3000"`
   - Resolved conflict with previously allocated ports

## Environment Variables

The following environment variables reflect the new port allocations:

```bash
# API Service
PORT=3010
ALCHEMORSEL_MONITORING_METRICS_PORT=3012

# Web Service  
PORT=3011
API_URL=http://api:3010
```

## Compliance

- ✅ **ADR-0005**: All application ports allocated through portscan system
- ✅ **ADR-0003**: Docker Compose architecture maintained
- ✅ **ADR-0002**: PostgreSQL standard port maintained (5432)
- ✅ **Conflict Resolution**: No port conflicts with existing allocations

## Renewal Process

Port allocations expire after 24 hours. To renew:

1. Check expiration with `portscan ports list`
2. Renew before expiration with `portscan ports renew <allocation-id>`
3. Update this document with new expiration times

## Troubleshooting

### Port Conflicts
- Check `portscan ports list` for current allocations
- Use `portscan ports suggest development N` for alternatives
- Update Docker Compose configuration accordingly

### Service Communication
- Ensure inter-service URLs use internal Docker network names
- API service accessible at `http://api:3010` from web service
- External access via host ports (3010, 3011, 3012)

---

**Last Updated**: August 19, 2025  
**Next Review**: August 20, 2025 (before port expiration)