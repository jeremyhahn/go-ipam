# IPAM REST API Reference

Complete reference for the IPAM REST API endpoints.

## Base URL

- **Standalone**: `http://localhost:8080/api/v1`
- **Cluster**: `http://localhost:8080/api/v1` (load balanced)

## Authentication

Currently, the API does not implement authentication. For production deployments, implement authentication at the reverse proxy level.

## Response Format

All responses use JSON format with consistent error handling:

```json
// Success response
{
  "id": "net-123",
  "cidr": "192.168.1.0/24",
  "description": "Office network"
}

// Error response
{
  "error": "Network not found",
  "code": 404
}
```

## Network Management

### List Networks

List all configured networks.

**Request:**
```http
GET /api/v1/networks
```

**Response:**
```json
[
  {
    "id": "net-123",
    "cidr": "192.168.1.0/24",
    "description": "Office network",
    "tags": ["production", "office"],
    "created_at": "2024-01-15T10:30:00Z"
  }
]
```

### Create Network

Add a new network CIDR.

**Request:**
```http
POST /api/v1/networks
Content-Type: application/json

{
  "cidr": "10.0.0.0/16",
  "description": "Production network",
  "tags": ["prod", "web"]
}
```

**Response:**
```json
{
  "id": "net-456",
  "cidr": "10.0.0.0/16",
  "description": "Production network",
  "tags": ["prod", "web"],
  "created_at": "2024-01-15T10:30:00Z"
}
```

### Get Network

Retrieve details for a specific network.

**Request:**
```http
GET /api/v1/networks/{id}
```

**Response:**
```json
{
  "id": "net-123",
  "cidr": "192.168.1.0/24",
  "description": "Office network",
  "tags": ["production", "office"],
  "created_at": "2024-01-15T10:30:00Z"
}
```

### Delete Network

Remove a network. Fails if the network has active allocations.

**Request:**
```http
DELETE /api/v1/networks/{id}
```

**Response:**
```http
204 No Content
```

### Get Network Statistics

Get utilization statistics for a network.

**Request:**
```http
GET /api/v1/networks/{id}/stats
```

**Response:**
```json
{
  "network_id": "net-123",
  "total_ips": 254,
  "allocated_ips": 45,
  "available_ips": 209,
  "utilization_percent": 17.7,
  "first_available": "192.168.1.46",
  "last_allocated": "192.168.1.45"
}
```

## IP Allocation Management

### List Allocations

List IP allocations, optionally filtered by network.

**Request:**
```http
GET /api/v1/allocations
GET /api/v1/allocations?network_id=net-123
GET /api/v1/allocations?all=true
```

**Parameters:**
- `network_id` (optional): Filter by specific network
- `all` (optional): Include released IPs in results

**Response:**
```json
[
  {
    "id": "alloc-789",
    "network_id": "net-123",
    "ip": "192.168.1.10",
    "end_ip": null,
    "hostname": "web-server-01",
    "description": "Web server",
    "status": "allocated",
    "allocated_at": "2024-01-15T10:30:00Z",
    "expires_at": null,
    "released_at": null
  }
]
```

### Allocate IP Address

Allocate one or more IP addresses from a network.

**Request:**
```http
POST /api/v1/allocations
Content-Type: application/json

{
  "network_id": "net-123",
  "count": 1,
  "hostname": "web-server-02",
  "description": "New web server",
  "ttl_hours": 24
}
```

**Parameters:**
- `network_id` (required): Target network ID
- `count` (optional, default: 1): Number of IPs to allocate
- `hostname` (optional): Hostname for the allocation
- `description` (optional): Description of the allocation
- `ttl_hours` (optional): TTL in hours for automatic expiration

**Response:**
```json
{
  "id": "alloc-790",
  "network_id": "net-123",
  "ip": "192.168.1.11",
  "end_ip": null,
  "hostname": "web-server-02",
  "description": "New web server",
  "status": "allocated",
  "allocated_at": "2024-01-15T10:35:00Z",
  "expires_at": "2024-01-16T10:35:00Z",
  "released_at": null
}
```

### Get Allocation

Retrieve details for a specific allocation.

**Request:**
```http
GET /api/v1/allocations/{id}
```

**Response:**
```json
{
  "id": "alloc-789",
  "network_id": "net-123",
  "ip": "192.168.1.10",
  "end_ip": null,
  "hostname": "web-server-01",
  "description": "Web server",
  "status": "allocated",
  "allocated_at": "2024-01-15T10:30:00Z",
  "expires_at": null,
  "released_at": null
}
```

### Release IP Address

Release an allocated IP address back to the pool.

**Request:**
```http
POST /api/v1/allocations/{id}/release
```

**Response:**
```http
204 No Content
```

## Cluster Management

*Available only in cluster mode*

### Get Cluster Status

Get cluster information and member status.

**Request:**
```http
GET /api/v1/cluster/status
```

**Response:**
```json
{
  "node_id": 1,
  "cluster_id": 100,
  "role": "leader",
  "raft_state": "StateLeader",
  "members": [
    {
      "node_id": 1,
      "addr": "node1.example.com:5001",
      "role": "leader"
    },
    {
      "node_id": 2,
      "addr": "node2.example.com:5002",
      "role": "follower"
    },
    {
      "node_id": 3,
      "addr": "node3.example.com:5003",
      "role": "follower"
    }
  ],
  "leader_id": 1,
  "term": 1,
  "commit_index": 42
}
```

### Add Cluster Node

Add a new node to the cluster.

**Request:**
```http
POST /api/v1/cluster/nodes
Content-Type: application/json

{
  "node_id": 4,
  "addr": "node4.example.com:5004"
}
```

**Response:**
```http
204 No Content
```

### Remove Cluster Node

Remove a node from the cluster.

**Request:**
```http
DELETE /api/v1/cluster/nodes/{nodeID}
```

**Response:**
```http
204 No Content
```

## System Endpoints

### Health Check

Check system health and readiness.

**Request:**
```http
GET /api/v1/health
```

**Response:**
```json
{
  "status": "healthy",
  "service": "ipam",
  "cluster_mode": true,
  "version": "1.0.0",
  "uptime": "2h30m15s"
}
```

### Audit Log

Retrieve audit log entries.

**Request:**
```http
GET /api/v1/audit
GET /api/v1/audit?limit=50
```

**Parameters:**
- `limit` (optional, default: 100): Maximum number of entries to return

**Response:**
```json
[
  {
    "timestamp": "2024-01-15T10:30:00Z",
    "action": "network_created",
    "resource": "net-123",
    "details": "Created network 192.168.1.0/24",
    "user": "system"
  },
  {
    "timestamp": "2024-01-15T10:35:00Z",
    "action": "ip_allocated",
    "resource": "alloc-789",
    "details": "Allocated 192.168.1.10 to web-server-01",
    "user": "system"
  }
]
```

## Error Codes

Standard HTTP status codes are used:

- **200**: Success
- **201**: Created
- **204**: No Content
- **400**: Bad Request - Invalid parameters
- **404**: Not Found - Resource doesn't exist
- **409**: Conflict - Resource already exists or has dependencies
- **500**: Internal Server Error

## Rate Limiting

Currently no rate limiting is implemented. For production deployments, implement rate limiting at the reverse proxy level.

## Examples

### Complete Workflow Example

```bash
# 1. Create a network
NETWORK=$(curl -s -X POST http://localhost:8080/api/v1/networks \
  -H "Content-Type: application/json" \
  -d '{"cidr": "10.0.0.0/24", "description": "Test network"}')

NETWORK_ID=$(echo $NETWORK | jq -r '.id')

# 2. Allocate an IP
ALLOCATION=$(curl -s -X POST http://localhost:8080/api/v1/allocations \
  -H "Content-Type: application/json" \
  -d "{\"network_id\": \"$NETWORK_ID\", \"hostname\": \"test-server\"}")

ALLOCATION_ID=$(echo $ALLOCATION | jq -r '.id')
IP=$(echo $ALLOCATION | jq -r '.ip')

echo "Allocated IP: $IP"

# 3. Check network stats
curl -s http://localhost:8080/api/v1/networks/$NETWORK_ID/stats | jq

# 4. Release the IP
curl -s -X POST http://localhost:8080/api/v1/allocations/$ALLOCATION_ID/release

# 5. Clean up
curl -s -X DELETE http://localhost:8080/api/v1/networks/$NETWORK_ID
```

### Bulk Operations

```bash
# Allocate multiple IPs
curl -X POST http://localhost:8080/api/v1/allocations \
  -H "Content-Type: application/json" \
  -d '{"network_id": "net-123", "count": 10, "description": "Load balancer pool"}'
```

### Cluster Operations

```bash
# Check cluster health
curl http://localhost:8080/api/v1/cluster/status | jq '.members'

# Add a node
curl -X POST http://localhost:8080/api/v1/cluster/nodes \
  -H "Content-Type: application/json" \
  -d '{"node_id": 4, "addr": "node4.example.com:5004"}'
```