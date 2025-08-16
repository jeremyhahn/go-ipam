# IPAM Cluster Deployment Guide

This guide covers deploying IPAM in cluster mode for high availability and distributed consensus.

## Deployment Options

### Option 1: Docker Compose (Recommended)

The easiest way to deploy a production 3-node cluster:

```bash
# Start 3-node cluster with load balancer
cd examples/cluster
docker-compose up -d
```

This creates:
- 3 IPAM nodes with Raft consensus (ports 8081-8083)
- NGINX load balancer on port 8080
- Automatic leader election and failover
- Data persistence with Docker volumes

**Access the cluster:**
```bash
# Via load balancer
curl http://localhost:8080/api/v1/health

# Direct to nodes
curl http://localhost:8081/api/v1/cluster/status
curl http://localhost:8082/api/v1/cluster/status  
curl http://localhost:8083/api/v1/cluster/status
```

### Option 2: Automated Script

Use the provided script for local testing:

```bash
./scripts/3-node-cluster.sh
```

### Option 3: Manual Setup

For custom deployments or understanding the process:

#### Single-Node Cluster (Development)

Perfect for testing cluster features:

```bash
# Initialize single-node cluster
./ipam cluster init --node-id 1 --cluster-id 100 --raft-addr localhost:5001 --single-node

# Start cluster server
./ipam server --cluster --config ipam-cluster-data/cluster.json
```

#### Multi-Node Cluster (Production)

**Step 1: Initialize the first node**
```bash
./ipam cluster init \
  --node-id 1 \
  --cluster-id 100 \
  --raft-addr node1.example.com:5001 \
  --data-dir node1-data \
  --initial-members "1:node1.example.com:5001,2:node2.example.com:5002,3:node3.example.com:5003"
```

**Step 2: Join additional nodes**
```bash
# Node 2
./ipam cluster join \
  --node-id 2 \
  --cluster-id 100 \
  --raft-addr node2.example.com:5002 \
  --data-dir node2-data \
  --initial-members "1:node1.example.com:5001,2:node2.example.com:5002,3:node3.example.com:5003"

# Node 3  
./ipam cluster join \
  --node-id 3 \
  --cluster-id 100 \
  --raft-addr node3.example.com:5003 \
  --data-dir node3-data \
  --initial-members "1:node1.example.com:5001,2:node2.example.com:5002,3:node3.example.com:5003"
```

**Step 3: Start the servers**
```bash
# Node 1
./ipam server --cluster --config node1-data/cluster.json --port 8080

# Node 2  
./ipam server --cluster --config node2-data/cluster.json --port 8080

# Node 3
./ipam server --cluster --config node3-data/cluster.json --port 8080
```

## Cluster Management

### Dynamic Node Management

Add or remove nodes using the cluster management API:

```bash
# Add a new node to the cluster
curl -X POST http://localhost:8080/api/v1/cluster/nodes \
  -H "Content-Type: application/json" \
  -d '{"node_id": 4, "addr": "node4.example.com:5004"}'

# Remove a node from the cluster
curl -X DELETE http://localhost:8080/api/v1/cluster/nodes/4
```

### Cluster Status

Check cluster health and member status:

```bash
# Get cluster information
curl http://localhost:8080/api/v1/cluster/status

# Response includes:
# - Node ID and role (leader/follower)
# - Cluster membership
# - Raft state information
```

## API Usage

The cluster provides the same REST API as standalone mode with automatic leader forwarding:

```bash
# Add a network (request goes to leader automatically)
curl -X POST http://localhost:8080/api/v1/networks \
  -H "Content-Type: application/json" \
  -d '{"cidr": "10.0.0.0/16", "description": "Production network"}'

# Allocate an IP
curl -X POST http://localhost:8080/api/v1/allocations \
  -H "Content-Type: application/json" \
  -d '{"network_id": "net-123", "hostname": "web-server"}'

# List allocations (can read from any node)
curl http://localhost:8080/api/v1/allocations
```

## Load Balancing

For production deployments, use a load balancer to distribute requests:

### NGINX Configuration

See `examples/cluster/nginx.conf` for a complete configuration:

```nginx
upstream ipam_cluster {
    least_conn;
    server node1.example.com:8080 max_fails=2 fail_timeout=30s;
    server node2.example.com:8080 max_fails=2 fail_timeout=30s;
    server node3.example.com:8080 max_fails=2 fail_timeout=30s;
}

server {
    listen 80;
    location /api/ {
        proxy_pass http://ipam_cluster;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## Production Considerations

### Security
- Use TLS for all external communications
- Secure Raft ports (5000-5003) with firewalls
- Implement authentication for API endpoints
- Use network policies in Kubernetes deployments

### Monitoring  
- Monitor cluster status via `/api/v1/cluster/status`
- Set up alerts for leader election events
- Monitor Raft log replication lag
- Use health checks: `/api/v1/health`

### Backup and Recovery
- Raft consensus provides automatic data replication
- For disaster recovery, backup one node's data directory
- Test restoration procedures regularly

### Performance Tuning
- Use SSD storage for Raft logs
- Ensure low-latency network between nodes
- Configure appropriate Raft timeouts for your network

## Testing on Single Machine

For local development and testing:

```bash
# Use different ports for each node
./ipam cluster init --node-id 1 --cluster-id 100 --raft-addr localhost:5001 --data-dir node1-data --initial-members "1:localhost:5001,2:localhost:5002,3:localhost:5003"
./ipam cluster join --node-id 2 --cluster-id 100 --raft-addr localhost:5002 --data-dir node2-data --initial-members "1:localhost:5001,2:localhost:5002,3:localhost:5003"  
./ipam cluster join --node-id 3 --cluster-id 100 --raft-addr localhost:5003 --data-dir node3-data --initial-members "1:localhost:5001,2:localhost:5002,3:localhost:5003"

# Start servers in separate terminals
./ipam server --cluster --config node1-data/cluster.json --port 8081
./ipam server --cluster --config node2-data/cluster.json --port 8082
./ipam server --cluster --config node3-data/cluster.json --port 8083
```

## Troubleshooting

### Common Issues

1. **Leader election fails**
   - Verify all nodes have identical `--initial-members` configuration
   - Check network connectivity between Raft ports
   - Ensure unique node IDs across the cluster

2. **Nodes can't join cluster**
   - Verify Raft addresses are accessible
   - Check firewall rules for ports 5000-5003
   - Ensure data directories have proper permissions

3. **API requests fail**
   - Use cluster status endpoint to verify leader
   - Check if nodes are properly joined to cluster
   - Verify API ports are accessible

4. **Data inconsistency**
   - Check Raft log synchronization
   - Verify no network partitions
   - Review cluster member status

### Debugging Commands

```bash
# Check cluster status
curl http://localhost:8080/api/v1/cluster/status

# Health check
curl http://localhost:8080/api/v1/health  

# View audit log for cluster events
curl http://localhost:8080/api/v1/audit

# CLI cluster status (if available)
./ipam cluster status --config node1-data/cluster.json
```