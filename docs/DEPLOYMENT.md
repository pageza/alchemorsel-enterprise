# Alchemorsel v3 Production Deployment Guide

This guide covers production deployment strategies for Alchemorsel v3, including cloud platforms, security considerations, and operational best practices.

## 🏗️ Deployment Architecture

Alchemorsel v3 follows a microservices architecture suitable for various deployment strategies:

```
┌─────────────────┐    ┌─────────────────┐
│   Load Balancer │    │   Web Frontend  │
│   (Nginx/ALB)   │◄──►│   (Port 8080)   │
└─────────────────┘    └─────────────────┘
          │                       │
          │                       ▼
          │            ┌─────────────────┐
          └───────────►│   API Backend   │
                       │   (Port 3000)   │
                       └─────────────────┘
                                │
                                ▼
                    ┌─────────────────────┐
                    │    PostgreSQL       │
                    │    Redis Cache      │
                    │    File Storage     │
                    └─────────────────────┘
```

## 🚀 Deployment Options

### Option 1: Docker Swarm

**Best for:** Small to medium deployments, existing Docker infrastructure

```bash
# Initialize swarm
docker swarm init

# Deploy stack
docker stack deploy -c docker-compose.production.yml alchemorsel

# Scale services
docker service scale alchemorsel_api-backend=3
docker service scale alchemorsel_web-frontend=2
```

### Option 2: Kubernetes

**Best for:** Large scale deployments, cloud-native environments

```yaml
# k8s/api-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: alchemorsel-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: alchemorsel-api
  template:
    metadata:
      labels:
        app: alchemorsel-api
    spec:
      containers:
      - name: api
        image: alchemorsel/api:latest
        ports:
        - containerPort: 3000
        env:
        - name: ALCHEMORSEL_DATABASE_HOST
          value: "postgres-service"
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
```

### Option 3: Cloud Platforms

#### AWS Deployment

**Using AWS ECS + Fargate:**

```json
{
  "family": "alchemorsel-api",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "512",
  "memory": "1024",
  "executionRoleArn": "arn:aws:iam::ACCOUNT:role/ecsTaskExecutionRole",
  "containerDefinitions": [
    {
      "name": "alchemorsel-api",
      "image": "your-registry/alchemorsel-api:latest",
      "portMappings": [
        {
          "containerPort": 3000,
          "protocol": "tcp"
        }
      ],
      "environment": [
        {
          "name": "ALCHEMORSEL_DATABASE_HOST",
          "value": "your-rds-endpoint"
        }
      ]
    }
  ]
}
```

**Infrastructure as Code (Terraform):**

```hcl
# main.tf
resource "aws_ecs_cluster" "alchemorsel" {
  name = "alchemorsel-cluster"
}

resource "aws_ecs_service" "api" {
  name            = "alchemorsel-api"
  cluster         = aws_ecs_cluster.alchemorsel.id
  task_definition = aws_ecs_task_definition.api.arn
  desired_count   = 3
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [aws_security_group.api.id]
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.api.arn
    container_name   = "alchemorsel-api"
    container_port   = 3000
  }
}
```

#### Google Cloud Run

```yaml
# cloudbuild.yaml
steps:
  - name: 'gcr.io/cloud-builders/docker'
    args: ['build', '-f', 'Dockerfile.api', '-t', 'gcr.io/$PROJECT_ID/alchemorsel-api', '.']
  - name: 'gcr.io/cloud-builders/docker'
    args: ['push', 'gcr.io/$PROJECT_ID/alchemorsel-api']
  - name: 'gcr.io/cloud-builders/gcloud'
    args: ['run', 'deploy', 'alchemorsel-api', '--image', 'gcr.io/$PROJECT_ID/alchemorsel-api', '--region', 'us-central1']
```

#### Azure Container Instances

```bash
# Deploy API service
az container create \
  --resource-group alchemorsel-rg \
  --name alchemorsel-api \
  --image yourregistry.azurecr.io/alchemorsel-api:latest \
  --cpu 2 \
  --memory 4 \
  --ports 3000 \
  --environment-variables ALCHEMORSEL_DATABASE_HOST=your-postgres-server
```

## 🔐 Security Configuration

### SSL/TLS Configuration

**Nginx SSL Setup:**

```nginx
server {
    listen 443 ssl http2;
    server_name api.alchemorsel.com;
    
    ssl_certificate /etc/ssl/certs/alchemorsel.pem;
    ssl_certificate_key /etc/ssl/private/alchemorsel.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES256-GCM-SHA384:ECDHE-RSA-AES128-GCM-SHA256;
    ssl_prefer_server_ciphers off;
    
    # HSTS
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    
    location / {
        proxy_pass http://api-backend:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Environment Security

**Production .env template:**

```bash
# Security settings
ALCHEMORSEL_APP_ENVIRONMENT=production
ALCHEMORSEL_JWT_SECRET=$(openssl rand -hex 32)
ALCHEMORSEL_SESSION_SECRET=$(openssl rand -hex 32)
ALCHEMORSEL_SESSION_SECURE=true

# Database with SSL
ALCHEMORSEL_DATABASE_SSL_MODE=require
ALCHEMORSEL_DATABASE_SSL_CERT=/etc/ssl/postgresql/client-cert.pem
ALCHEMORSEL_DATABASE_SSL_KEY=/etc/ssl/postgresql/client-key.pem
ALCHEMORSEL_DATABASE_SSL_ROOT_CERT=/etc/ssl/postgresql/ca-cert.pem

# CORS restrictions
ALCHEMORSEL_CORS_ALLOWED_ORIGINS=https://alchemorsel.com,https://app.alchemorsel.com
ALCHEMORSEL_CORS_ALLOW_CREDENTIALS=true

# Rate limiting
ALCHEMORSEL_SECURITY_RATE_LIMIT_ENABLED=true
ALCHEMORSEL_SECURITY_RATE_LIMIT_REQUESTS_PER_MINUTE=60
```

### Secrets Management

**Using Docker Secrets:**

```yaml
version: '3.8'
services:
  api-backend:
    image: alchemorsel/api:latest
    secrets:
      - jwt_secret
      - db_password
    environment:
      ALCHEMORSEL_JWT_SECRET_FILE: /run/secrets/jwt_secret
      ALCHEMORSEL_DATABASE_PASSWORD_FILE: /run/secrets/db_password

secrets:
  jwt_secret:
    external: true
  db_password:
    external: true
```

**Using Kubernetes Secrets:**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: alchemorsel-secrets
type: Opaque
data:
  jwt-secret: <base64-encoded-secret>
  db-password: <base64-encoded-password>
```

## 📊 Monitoring Setup

### Prometheus Configuration

```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'alchemorsel-api'
    static_configs:
      - targets: ['api-backend:9091']
    metrics_path: /metrics

  - job_name: 'alchemorsel-web'
    static_configs:
      - targets: ['web-frontend:9092']
    metrics_path: /metrics

rule_files:
  - "alchemorsel-alerts.yml"

alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - alertmanager:9093
```

### Grafana Dashboards

**API Performance Dashboard:**

```json
{
  "dashboard": {
    "title": "Alchemorsel API Performance",
    "panels": [
      {
        "title": "Request Rate",
        "targets": [
          {
            "expr": "rate(http_requests_total[5m])",
            "legendFormat": "{{method}} {{status}}"
          }
        ]
      },
      {
        "title": "Response Time",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))",
            "legendFormat": "95th percentile"
          }
        ]
      }
    ]
  }
}
```

### Alerting Rules

```yaml
# alchemorsel-alerts.yml
groups:
  - name: alchemorsel-api
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.1
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value }} requests per second"

      - alert: HighLatency
        expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 0.5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High latency detected"
          description: "95th percentile latency is {{ $value }}s"
```

## 🗃️ Database Management

### PostgreSQL Production Setup

**High Availability with Streaming Replication:**

```bash
# Primary server postgresql.conf
wal_level = replica
max_wal_senders = 3
wal_keep_segments = 64
archive_mode = on
archive_command = 'cp %p /var/lib/postgresql/wal_archive/%f'

# Replica server recovery.conf
standby_mode = 'on'
primary_conninfo = 'host=primary-db port=5432 user=replicator'
```

**Connection Pooling with PgBouncer:**

```ini
# pgbouncer.ini
[databases]
alchemorsel_prod = host=postgres-primary port=5432 dbname=alchemorsel_prod

[pgbouncer]
listen_port = 6432
listen_addr = 0.0.0.0
auth_type = md5
auth_file = /etc/pgbouncer/userlist.txt
pool_mode = transaction
max_client_conn = 100
default_pool_size = 25
```

### Database Migrations

**Production Migration Strategy:**

```bash
# Pre-deployment checks
./scripts/migrate-validate.sh

# Zero-downtime migration
./scripts/migrate-up.sh --safe-mode

# Post-deployment verification
./scripts/migrate-verify.sh
```

## 📈 Scaling Strategies

### Horizontal Scaling

**Load Balancer Configuration:**

```nginx
upstream api_backend {
    least_conn;
    server api-backend-1:3000 weight=1 max_fails=3 fail_timeout=30s;
    server api-backend-2:3000 weight=1 max_fails=3 fail_timeout=30s;
    server api-backend-3:3000 weight=1 max_fails=3 fail_timeout=30s;
}

upstream web_frontend {
    ip_hash;  # Sticky sessions for web frontend
    server web-frontend-1:8080 weight=1 max_fails=3 fail_timeout=30s;
    server web-frontend-2:8080 weight=1 max_fails=3 fail_timeout=30s;
}
```

**Auto-scaling with Kubernetes HPA:**

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: alchemorsel-api-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: alchemorsel-api
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

### Vertical Scaling

**Resource Optimization:**

```yaml
# Production resource allocation
services:
  api-backend:
    deploy:
      resources:
        limits:
          memory: 1G
          cpus: '1.0'
        reservations:
          memory: 512M
          cpus: '0.5'

  web-frontend:
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: '0.5'
        reservations:
          memory: 256M
          cpus: '0.25'
```

## 🔄 CI/CD Pipeline

### GitHub Actions

```yaml
# .github/workflows/deploy.yml
name: Deploy to Production

on:
  push:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
        with:
          go-version: 1.23
      - run: go test ./...

  security-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: securecodewarrior/github-action-add-sarif@v1
        with:
          sarif-file: gosec-report.sarif

  build-and-deploy:
    needs: [test, security-scan]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Build Docker images
        run: |
          docker build -f Dockerfile.api -t ${{ secrets.REGISTRY }}/alchemorsel-api:${{ github.sha }} .
          docker build -f Dockerfile.web -t ${{ secrets.REGISTRY }}/alchemorsel-web:${{ github.sha }} .
      
      - name: Push to registry
        run: |
          docker push ${{ secrets.REGISTRY }}/alchemorsel-api:${{ github.sha }}
          docker push ${{ secrets.REGISTRY }}/alchemorsel-web:${{ github.sha }}
      
      - name: Deploy to production
        run: |
          kubectl set image deployment/alchemorsel-api api=${{ secrets.REGISTRY }}/alchemorsel-api:${{ github.sha }}
          kubectl set image deployment/alchemorsel-web web=${{ secrets.REGISTRY }}/alchemorsel-web:${{ github.sha }}
```

### Blue-Green Deployment

```bash
#!/bin/bash
# scripts/blue-green-deploy.sh

# Deploy to green environment
kubectl apply -f k8s/green-deployment.yaml

# Wait for green to be ready
kubectl wait --for=condition=available --timeout=300s deployment/alchemorsel-api-green

# Run health checks
if ./scripts/health-check.sh green; then
    # Switch traffic to green
    kubectl patch service alchemorsel-api -p '{"spec":{"selector":{"version":"green"}}}'
    
    # Clean up blue
    kubectl delete deployment alchemorsel-api-blue
else
    # Rollback - clean up failed green deployment
    kubectl delete deployment alchemorsel-api-green
    exit 1
fi
```

## 🔍 Troubleshooting

### Common Production Issues

#### High CPU Usage
```bash
# Check container resources
docker stats

# Profile API service
go tool pprof http://api-backend:3000/debug/pprof/profile

# Check for goroutine leaks
go tool pprof http://api-backend:3000/debug/pprof/goroutine
```

#### Memory Leaks
```bash
# Monitor memory usage
docker exec api-backend cat /proc/meminfo

# Heap profiling
go tool pprof http://api-backend:3000/debug/pprof/heap
```

#### Database Connection Issues
```bash
# Check connection pool
psql -h postgres -U postgres -c "SELECT * FROM pg_stat_activity;"

# Monitor slow queries
psql -h postgres -U postgres -c "SELECT query FROM pg_stat_statements ORDER BY total_time DESC LIMIT 10;"
```

### Log Analysis

**Centralized Logging with ELK Stack:**

```yaml
version: '3.8'
services:
  elasticsearch:
    image: elasticsearch:7.17.0
    environment:
      - discovery.type=single-node
    
  logstash:
    image: logstash:7.17.0
    volumes:
      - ./logstash.conf:/usr/share/logstash/pipeline/logstash.conf
    
  kibana:
    image: kibana:7.17.0
    ports:
      - "5601:5601"
    environment:
      ELASTICSEARCH_HOSTS: http://elasticsearch:9200
```

## 📋 Production Checklist

### Pre-deployment
- [ ] Security scan completed
- [ ] Load testing performed
- [ ] Database migrations tested
- [ ] Backup strategy verified
- [ ] Monitoring configured
- [ ] SSL certificates valid
- [ ] Environment variables updated
- [ ] Health checks functional

### Post-deployment
- [ ] Health checks passing
- [ ] Metrics flowing to monitoring
- [ ] Logs being collected
- [ ] SSL/TLS working
- [ ] Load balancer healthy
- [ ] Database connections stable
- [ ] API documentation accessible
- [ ] End-to-end tests passing

### Ongoing Maintenance
- [ ] Monitor resource usage
- [ ] Review security logs
- [ ] Update dependencies
- [ ] Backup verification
- [ ] Performance optimization
- [ ] Capacity planning
- [ ] Incident response testing

## 📞 Support and Maintenance

### Emergency Contacts
- **On-call Engineer:** +1-xxx-xxx-xxxx
- **Database Admin:** +1-xxx-xxx-xxxx
- **Security Team:** security@alchemorsel.com

### Runbooks
- [Database Failover](./runbooks/database-failover.md)
- [API Service Recovery](./runbooks/api-recovery.md)
- [Security Incident Response](./runbooks/security-incident.md)

### Maintenance Windows
- **Scheduled:** Every Sunday 2:00-4:00 AM UTC
- **Emergency:** As needed with 1-hour notice
- **Major Updates:** Monthly, first Sunday