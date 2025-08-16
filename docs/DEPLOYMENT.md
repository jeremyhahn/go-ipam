# IPAM Production Deployment Guide

This guide covers deploying IPAM in production environments with security, monitoring, and operational best practices.

## Overview

IPAM supports three deployment modes:

1. **Standalone Mode**: Single node with PebbleDB storage
2. **Single-Node Cluster**: Development cluster testing
3. **Multi-Node Cluster**: Production high-availability deployment

## Quick Start

### Standalone Deployment

Perfect for single-node deployments or development:

```bash
# Build and start
make build
./ipam server --host 0.0.0.0 --port 8080

# Or with custom database path
./ipam server --db /var/lib/ipam --host 0.0.0.0 --port 8080
```

### Cluster Deployment (Docker Compose)

Recommended for production:

```bash
cd examples/cluster
docker-compose up -d
```

## Production Configuration

### Environment Variables

Configure IPAM using environment variables:

```bash
# Database configuration
export IPAM_DB_PATH="/var/lib/ipam"

# Server configuration  
export IPAM_HOST="0.0.0.0"
export IPAM_PORT="8080"

# Cluster configuration
export IPAM_CLUSTER_MODE="true"
export IPAM_CONFIG_FILE="/etc/ipam/cluster.json"
```

### Systemd Service

Create `/etc/systemd/system/ipam.service`:

```ini
[Unit]
Description=IPAM Server
After=network.target
Wants=network.target

[Service]
Type=simple
User=ipam
Group=ipam
ExecStart=/usr/local/bin/ipam server
Environment=IPAM_DB_PATH=/var/lib/ipam
Environment=IPAM_HOST=0.0.0.0
Environment=IPAM_PORT=8080
Restart=always
RestartSec=5
KillMode=mixed
KillSignal=SIGTERM

# Security settings
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/ipam
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable ipam
sudo systemctl start ipam
sudo systemctl status ipam
```

## Security Hardening

### Reverse Proxy Setup

Use NGINX as a reverse proxy with TLS and authentication:

```nginx
server {
    listen 443 ssl http2;
    server_name ipam.example.com;

    # TLS configuration
    ssl_certificate /etc/ssl/certs/ipam.crt;
    ssl_certificate_key /etc/ssl/private/ipam.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # Authentication (example with basic auth)
    auth_basic "IPAM Access";
    auth_basic_user_file /etc/nginx/.htpasswd;

    # Rate limiting
    limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s;
    limit_req zone=api burst=20 nodelay;

    location /api/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Timeouts
        proxy_connect_timeout 30s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;
    }

    # Health check endpoint (no auth required)
    location /api/v1/health {
        auth_basic off;
        proxy_pass http://localhost:8080;
    }
}
```

### Firewall Configuration

Standalone mode:
```bash
# Allow API access
sudo ufw allow 8080/tcp

# If using NGINX
sudo ufw allow 443/tcp
sudo ufw allow 80/tcp
```

Cluster mode:
```bash
# API access
sudo ufw allow 8080/tcp

# Raft communication (between cluster nodes only)
sudo ufw allow from 10.0.1.0/24 to any port 5001
sudo ufw allow from 10.0.1.0/24 to any port 5002  
sudo ufw allow from 10.0.1.0/24 to any port 5003
```

### User and Permissions

Create dedicated user:
```bash
sudo useradd -r -s /bin/false -d /var/lib/ipam ipam
sudo mkdir -p /var/lib/ipam
sudo chown ipam:ipam /var/lib/ipam
sudo chmod 750 /var/lib/ipam
```

## Cluster Production Deployment

### Multi-Node Setup

**Prerequisites:**
- 3 or 5 nodes (odd number for quorum)
- Low-latency network between nodes
- Persistent storage on each node
- Load balancer

**Node Configuration:**

Node 1 (`node1.example.com`):
```bash
./ipam cluster init \
  --node-id 1 \
  --cluster-id 100 \
  --raft-addr node1.example.com:5001 \
  --data-dir /var/lib/ipam \
  --initial-members "1:node1.example.com:5001,2:node2.example.com:5002,3:node3.example.com:5003"

./ipam server --cluster --config /var/lib/ipam/cluster.json --host 0.0.0.0 --port 8080
```

Node 2 (`node2.example.com`):
```bash
./ipam cluster join \
  --node-id 2 \
  --cluster-id 100 \
  --raft-addr node2.example.com:5002 \
  --data-dir /var/lib/ipam \
  --initial-members "1:node1.example.com:5001,2:node2.example.com:5002,3:node3.example.com:5003"

./ipam server --cluster --config /var/lib/ipam/cluster.json --host 0.0.0.0 --port 8080
```

Node 3 (`node3.example.com`):
```bash
./ipam cluster join \
  --node-id 3 \
  --cluster-id 100 \
  --raft-addr node3.example.com:5003 \
  --data-dir /var/lib/ipam \
  --initial-members "1:node1.example.com:5001,2:node2.example.com:5002,3:node3.example.com:5003"

./ipam server --cluster --config /var/lib/ipam/cluster.json --host 0.0.0.0 --port 8080
```

### Load Balancer Configuration

HAProxy configuration (`/etc/haproxy/haproxy.cfg`):

```
global
    daemon
    maxconn 4096

defaults
    mode http
    timeout connect 5s
    timeout client 30s
    timeout server 30s
    option httplog

frontend ipam_frontend
    bind *:80
    bind *:443 ssl crt /etc/ssl/certs/ipam.pem
    redirect scheme https if !{ ssl_fc }
    default_backend ipam_cluster

backend ipam_cluster
    balance roundrobin
    option httpchk GET /api/v1/health
    http-check expect status 200
    
    server node1 node1.example.com:8080 check
    server node2 node2.example.com:8080 check
    server node3 node3.example.com:8080 check

# Statistics (optional)
stats enable
stats uri /haproxy-stats
stats auth admin:secure_password
```

## Kubernetes Deployment

### Standalone Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ipam
  namespace: ipam-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ipam
  template:
    metadata:
      labels:
        app: ipam
    spec:
      containers:
      - name: ipam
        image: ipam:latest
        args: ["server", "--host", "0.0.0.0", "--port", "8080"]
        ports:
        - containerPort: 8080
        env:
        - name: IPAM_DB_PATH
          value: "/var/lib/ipam"
        volumeMounts:
        - name: data
          mountPath: /var/lib/ipam
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: ipam-data

---
apiVersion: v1
kind: Service
metadata:
  name: ipam-service
  namespace: ipam-system
spec:
  selector:
    app: ipam
  ports:
  - port: 8080
    targetPort: 8080
  type: ClusterIP

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ipam-data
  namespace: ipam-system
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```

### Cluster Deployment

Use StatefulSet for cluster deployment:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ipam-cluster
  namespace: ipam-system
spec:
  serviceName: ipam-cluster
  replicas: 3
  selector:
    matchLabels:
      app: ipam-cluster
  template:
    metadata:
      labels:
        app: ipam-cluster
    spec:
      containers:
      - name: ipam
        image: ipam:latest
        command: ["/bin/sh"]
        args: ["-c", "ipam cluster init --node-id $((${HOSTNAME##*-} + 1)) --cluster-id 100 --raft-addr ${HOSTNAME}.ipam-cluster.ipam-system.svc.cluster.local:5001 --data-dir /var/lib/ipam --initial-members '1:ipam-cluster-0.ipam-cluster.ipam-system.svc.cluster.local:5001,2:ipam-cluster-1.ipam-cluster.ipam-system.svc.cluster.local:5002,3:ipam-cluster-2.ipam-cluster.ipam-system.svc.cluster.local:5003' && ipam server --cluster --config /var/lib/ipam/cluster.json --host 0.0.0.0 --port 8080"]
        ports:
        - containerPort: 8080
          name: api
        - containerPort: 5001
          name: raft
        volumeMounts:
        - name: data
          mountPath: /var/lib/ipam
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
```

## Monitoring and Observability

### Health Checks

Monitor using the health endpoint:

```bash
#!/bin/bash
# Health check script
HEALTH_URL="http://localhost:8080/api/v1/health"
if curl -sf "$HEALTH_URL" > /dev/null; then
    echo "IPAM healthy"
    exit 0
else
    echo "IPAM unhealthy"
    exit 1
fi
```

### Prometheus Monitoring

Add metrics collection (future enhancement):

```yaml
# Add to deployment
env:
- name: METRICS_ENABLED
  value: "true"
- name: METRICS_PORT
  value: "9090"
```

### Log Management

Configure structured logging:

```bash
# JSON log output
./ipam server --log-format json --log-level info

# Ship logs to centralized system
journalctl -u ipam -f | fluent-bit
```

## Backup and Recovery

### Standalone Mode

```bash
#!/bin/bash
# Backup script
BACKUP_DIR="/backup/ipam/$(date +%Y%m%d_%H%M%S)"
mkdir -p "$BACKUP_DIR"
cp -r /var/lib/ipam "$BACKUP_DIR/"
tar -czf "$BACKUP_DIR.tar.gz" -C "$(dirname $BACKUP_DIR)" "$(basename $BACKUP_DIR)"
rm -rf "$BACKUP_DIR"
```

### Cluster Mode

Raft provides automatic replication. For disaster recovery:

```bash
# Backup one node's data directory
tar -czf cluster-backup-$(date +%Y%m%d).tar.gz -C /var/lib/ipam .

# Restore process (if entire cluster lost)
# 1. Stop all nodes
# 2. Restore data to one node
# 3. Start as single-node cluster
# 4. Re-add other nodes
```

## Performance Tuning

### Database Optimization

```bash
# For high-performance storage
mount /var/lib/ipam -o noatime,barrier=0

# SSD-specific optimizations
echo noop > /sys/block/sdb/queue/scheduler
```

### Memory Settings

```bash
# Increase system limits for cluster mode
echo "ipam soft nofile 65536" >> /etc/security/limits.conf
echo "ipam hard nofile 65536" >> /etc/security/limits.conf
```

### Network Optimization

```bash
# Reduce Raft latency
sysctl -w net.core.rmem_max=16777216
sysctl -w net.core.wmem_max=16777216
sysctl -w net.ipv4.tcp_rmem="4096 65536 16777216"
sysctl -w net.ipv4.tcp_wmem="4096 65536 16777216"
```

## Troubleshooting

### Common Issues

1. **Permission denied accessing database**
   ```bash
   sudo chown -R ipam:ipam /var/lib/ipam
   sudo chmod -R 750 /var/lib/ipam
   ```

2. **Cluster nodes can't communicate**
   ```bash
   # Check connectivity
   telnet node2.example.com 5002
   
   # Check firewall
   sudo ufw status
   ```

3. **High memory usage**
   ```bash
   # Monitor memory usage
   ps aux | grep ipam
   
   # Check for memory leaks
   valgrind --leak-check=full ./ipam server
   ```

### Debugging Commands

```bash
# Check logs
journalctl -u ipam -f

# Test API connectivity
curl -v http://localhost:8080/api/v1/health

# Check cluster status
curl http://localhost:8080/api/v1/cluster/status | jq

# Database integrity check
./ipam stats --db /var/lib/ipam
```

## Maintenance

### Regular Tasks

1. **Log rotation** - Configure logrotate for application logs
2. **Backup verification** - Test backup restoration monthly
3. **Security updates** - Keep system and dependencies updated
4. **Capacity planning** - Monitor disk and memory usage
5. **Certificate renewal** - Automate TLS certificate updates

### Upgrade Process

1. **Backup** current data
2. **Test** in staging environment
3. **Rolling upgrade** for clusters (one node at a time)
4. **Verify** functionality after upgrade
5. **Rollback** plan if issues occur

This deployment guide provides a foundation for running IPAM reliably in production environments.