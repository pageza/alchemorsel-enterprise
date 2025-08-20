# Emergency Procedures Runbook

This runbook provides step-by-step procedures for handling critical incidents and emergencies in the Alchemorsel v3 production environment.

## 🚨 Incident Severity Levels

### Severity 1 (Critical) - 15 minute response time
- Complete service outage
- Data corruption or loss
- Security breach
- Payment system failure

### Severity 2 (High) - 30 minute response time
- Partial service degradation
- Performance issues affecting >50% of users
- Failed deployments blocking releases
- Database connectivity issues

### Severity 3 (Medium) - 2 hour response time
- Minor feature issues
- Performance degradation affecting <25% of users
- Non-critical third-party service failures

### Severity 4 (Low) - Next business day
- Cosmetic issues
- Documentation problems
- Enhancement requests

## 🆘 Emergency Contacts

### Primary Escalation Path
1. **On-Call Engineer**: +1-555-0123 (PagerDuty)
2. **DevOps Lead**: +1-555-0124
3. **Engineering Manager**: +1-555-0125
4. **CTO**: +1-555-0126

### External Contacts
- **AWS Support**: 1-206-266-4064 (Enterprise Support)
- **GitHub Support**: enterprise@github.com
- **DataDog Support**: support@datadoghq.com

### Communication Channels
- **Slack**: #incident-response
- **PagerDuty**: https://alchemorsel.pagerduty.com
- **Status Page**: https://status.alchemorsel.com

## 🔥 Critical Incident Procedures

### 1. Complete Service Outage

#### Immediate Actions (0-5 minutes)
1. **Acknowledge the incident**
   ```bash
   # Check if you can reach the application
   curl -I https://alchemorsel.com/health
   
   # Check status page
   curl -I https://status.alchemorsel.com
   ```

2. **Update status page**
   ```bash
   # Update status page (if accessible)
   curl -X POST https://api.statuspage.io/v1/pages/PAGE_ID/incidents \
     -H "Authorization: OAuth TOKEN" \
     -d "incident[name]=Service Outage Investigation" \
     -d "incident[status]=investigating"
   ```

3. **Check monitoring dashboards**
   - Grafana: https://grafana.alchemorsel.com
   - DataDog: https://app.datadoghq.com
   - AWS CloudWatch: Check EKS cluster health

#### Investigation (5-15 minutes)
1. **Check infrastructure status**
   ```bash
   # Check EKS cluster status
   kubectl get nodes
   kubectl get pods -A
   
   # Check critical services
   kubectl get pods -n alchemorsel
   kubectl get svc -n alchemorsel
   kubectl get ingress -n alchemorsel
   ```

2. **Check recent deployments**
   ```bash
   # Check recent deployments
   kubectl rollout history deployment/alchemorsel-api -n alchemorsel
   
   # Check if deployment is in progress
   kubectl get deployment alchemorsel-api -n alchemorsel -o wide
   ```

3. **Check logs**
   ```bash
   # Check application logs
   kubectl logs -l app.kubernetes.io/name=alchemorsel-api -n alchemorsel --tail=100
   
   # Check ingress controller logs
   kubectl logs -l app.kubernetes.io/name=ingress-nginx -n ingress-nginx --tail=100
   ```

#### Resolution Actions
1. **If recent deployment caused the issue**
   ```bash
   # Rollback to previous version
   kubectl rollout undo deployment/alchemorsel-api -n alchemorsel
   
   # Wait for rollback to complete
   kubectl rollout status deployment/alchemorsel-api -n alchemorsel
   ```

2. **If infrastructure issue**
   ```bash
   # Scale up replicas if pods are failing
   kubectl scale deployment alchemorsel-api --replicas=6 -n alchemorsel
   
   # Restart deployment if needed
   kubectl rollout restart deployment/alchemorsel-api -n alchemorsel
   ```

3. **If database issue**
   ```bash
   # Check RDS instance status
   aws rds describe-db-instances --db-instance-identifier alchemorsel-prod
   
   # Check database connectivity
   kubectl run db-test --rm -i --tty --image=postgres:15 -- \
     psql $DATABASE_URL -c "SELECT version();"
   ```

### 2. Security Breach Response

#### Immediate Actions (0-5 minutes)
1. **Assess the scope**
   - Determine what systems are affected
   - Check for unauthorized access
   - Review security monitoring alerts

2. **Isolate affected systems**
   ```bash
   # Scale down affected deployment
   kubectl scale deployment alchemorsel-api --replicas=0 -n alchemorsel
   
   # Block traffic at load balancer level
   kubectl patch ingress alchemorsel-ingress -n alchemorsel \
     -p '{"metadata":{"annotations":{"nginx.ingress.kubernetes.io/server-snippet":"return 503;"}}}'
   ```

3. **Notify security team**
   - Email: security@alchemorsel.com
   - Slack: #security-incidents
   - Follow security incident response plan

#### Investigation (5-30 minutes)
1. **Preserve evidence**
   ```bash
   # Create snapshot of affected systems
   kubectl get events --sort-by='.metadata.creationTimestamp' -A > events-$(date +%Y%m%d-%H%M%S).log
   
   # Export pod logs
   kubectl logs -l app.kubernetes.io/name=alchemorsel-api -n alchemorsel --since=1h > app-logs-$(date +%Y%m%d-%H%M%S).log
   ```

2. **Check for indicators of compromise**
   ```bash
   # Check for suspicious network activity
   kubectl logs -l app.kubernetes.io/name=ingress-nginx -n ingress-nginx --since=1h | grep -E "(404|403|500)"
   
   # Check authentication logs
   kubectl logs -l app.kubernetes.io/name=alchemorsel-api -n alchemorsel --since=1h | grep -i "auth"
   ```

3. **Review access logs**
   - Check AWS CloudTrail
   - Review application access logs
   - Check database audit logs

#### Recovery Actions
1. **Deploy clean version**
   ```bash
   # Deploy from known good image
   kubectl set image deployment/alchemorsel-api \
     alchemorsel-api=ghcr.io/alchemorsel/v3:KNOWN_GOOD_VERSION \
     -n alchemorsel
   ```

2. **Update secrets and credentials**
   ```bash
   # Rotate API keys
   kubectl create secret generic alchemorsel-secrets-new \
     --from-literal=jwt-secret="NEW_SECRET" \
     --from-literal=database-url="NEW_CONNECTION_STRING" \
     -n alchemorsel
   
   # Update deployment to use new secrets
   kubectl patch deployment alchemorsel-api -n alchemorsel \
     -p '{"spec":{"template":{"spec":{"containers":[{"name":"api","envFrom":[{"secretRef":{"name":"alchemorsel-secrets-new"}}]}]}}}}'
   ```

### 3. Database Failure

#### Immediate Actions (0-5 minutes)
1. **Check database status**
   ```bash
   # Check RDS instance
   aws rds describe-db-instances --db-instance-identifier alchemorsel-prod
   
   # Test connectivity
   kubectl run db-test --rm -i --tty --image=postgres:15 -- \
     psql $DATABASE_URL -c "SELECT NOW();"
   ```

2. **Check for recent changes**
   ```bash
   # Check recent migrations
   kubectl logs -l job-name=database-migration -n alchemorsel --tail=50
   
   # Check RDS events
   aws rds describe-events --source-identifier alchemorsel-prod --max-records 20
   ```

#### Recovery Actions
1. **If master database is down**
   ```bash
   # Check if read replica is available
   aws rds describe-db-instances --filters "Name=db-cluster-id,Values=alchemorsel-cluster"
   
   # Promote read replica if needed
   aws rds promote-read-replica --db-instance-identifier alchemorsel-replica
   ```

2. **If corruption detected**
   ```bash
   # Stop application to prevent further damage
   kubectl scale deployment alchemorsel-api --replicas=0 -n alchemorsel
   
   # Restore from latest backup
   aws rds restore-db-instance-from-db-snapshot \
     --db-instance-identifier alchemorsel-restore \
     --db-snapshot-identifier alchemorsel-backup-YYYYMMDD
   ```

### 4. Failed Deployment Recovery

#### Immediate Actions (0-5 minutes)
1. **Check deployment status**
   ```bash
   # Check rollout status
   kubectl rollout status deployment/alchemorsel-api -n alchemorsel --timeout=60s
   
   # Check pod status
   kubectl get pods -l app.kubernetes.io/name=alchemorsel-api -n alchemorsel
   ```

2. **Check for failing pods**
   ```bash
   # Get pod details
   kubectl describe pods -l app.kubernetes.io/name=alchemorsel-api -n alchemorsel
   
   # Check events
   kubectl get events -n alchemorsel --sort-by='.metadata.creationTimestamp'
   ```

#### Recovery Actions
1. **Rollback deployment**
   ```bash
   # Quick rollback to previous version
   kubectl rollout undo deployment/alchemorsel-api -n alchemorsel
   
   # Wait for rollback to complete
   kubectl rollout status deployment/alchemorsel-api -n alchemorsel
   ```

2. **If rollback fails**
   ```bash
   # Force rollback to specific revision
   kubectl rollout undo deployment/alchemorsel-api --to-revision=2 -n alchemorsel
   
   # Or scale to 0 and back up
   kubectl scale deployment alchemorsel-api --replicas=0 -n alchemorsel
   kubectl scale deployment alchemorsel-api --replicas=3 -n alchemorsel
   ```

## 🔧 Common Recovery Commands

### Quick Health Checks
```bash
# Application health
curl -f https://alchemorsel.com/health

# API health
curl -f https://alchemorsel.com/api/health

# Database connectivity
kubectl run db-test --rm -i --tty --image=postgres:15 -- \
  psql $DATABASE_URL -c "SELECT version();"

# Redis connectivity
kubectl run redis-test --rm -i --tty --image=redis:7 -- \
  redis-cli -u $REDIS_URL ping
```

### Deployment Management
```bash
# Check deployment status
kubectl get deployments -n alchemorsel

# Rollback deployment
kubectl rollout undo deployment/alchemorsel-api -n alchemorsel

# Scale deployment
kubectl scale deployment alchemorsel-api --replicas=5 -n alchemorsel

# Restart deployment
kubectl rollout restart deployment/alchemorsel-api -n alchemorsel
```

### Log Analysis
```bash
# Application logs (last 100 lines)
kubectl logs -l app.kubernetes.io/name=alchemorsel-api -n alchemorsel --tail=100

# Application logs (last hour)
kubectl logs -l app.kubernetes.io/name=alchemorsel-api -n alchemorsel --since=1h

# Events (sorted by time)
kubectl get events -n alchemorsel --sort-by='.metadata.creationTimestamp'

# Failed pod descriptions
kubectl describe pods -l app.kubernetes.io/name=alchemorsel-api -n alchemorsel | grep -A 10 -B 10 "Failed"
```

### Traffic Management
```bash
# Block all traffic
kubectl patch ingress alchemorsel-ingress -n alchemorsel \
  -p '{"metadata":{"annotations":{"nginx.ingress.kubernetes.io/server-snippet":"return 503;"}}}'

# Restore traffic
kubectl patch ingress alchemorsel-ingress -n alchemorsel \
  -p '{"metadata":{"annotations":{"nginx.ingress.kubernetes.io/server-snippet":"# normal traffic"}}}'

# Enable maintenance mode
kubectl create configmap maintenance-mode --from-literal=enabled=true -n alchemorsel
```

## 📋 Post-Incident Procedures

### Immediate Post-Resolution (0-30 minutes)
1. **Verify service restoration**
   - Run health checks
   - Check monitoring dashboards
   - Confirm user reports

2. **Update status page**
   - Mark incident as resolved
   - Provide brief resolution summary

3. **Notify stakeholders**
   - Internal teams
   - Customer support
   - Leadership (for Sev 1/2)

### Short-term Follow-up (1-24 hours)
1. **Document incident timeline**
   - When incident started
   - Detection time
   - Response actions taken
   - Resolution time

2. **Gather logs and evidence**
   - Application logs
   - Infrastructure metrics
   - Error reports

3. **Initial impact assessment**
   - Affected users
   - Service downtime
   - Business impact

### Long-term Follow-up (1-7 days)
1. **Conduct post-mortem**
   - Root cause analysis
   - Timeline reconstruction
   - Contributing factors

2. **Create action items**
   - Preventive measures
   - Process improvements
   - Tool enhancements

3. **Update procedures**
   - Update runbooks
   - Improve monitoring
   - Training needs

## 📞 Communication Templates

### Initial Incident Notification
```
🚨 INCIDENT ALERT - Severity X
Service: Alchemorsel Production
Issue: [Brief description]
Started: [Time]
Impact: [Description of user impact]
Response: [Current actions being taken]
Next Update: [Time commitment]
```

### Resolution Notification
```
✅ INCIDENT RESOLVED - Severity X
Service: Alchemorsel Production
Issue: [Brief description]
Duration: [Total time]
Resolution: [What was done to fix]
Follow-up: [Post-mortem scheduled/completed]
```

### Status Page Update
```
We are currently investigating reports of [issue description]. 
Our team is actively working to resolve this issue. 
We will provide updates every 30 minutes until resolved.
```

---

Remember: During incidents, clear communication and methodical troubleshooting are key. Don't hesitate to escalate early if needed.