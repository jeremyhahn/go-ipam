# go-ipam

High-performance IP Address Management (IPAM) system in Go with support for standalone and distributed cluster deployments.

## Features

- **IPv4/IPv6 Support**: Complete support for classful and classless networks
- **High Performance**: Bitmap-based allocation with 350+ ops/sec throughput
- **Dual Storage**: PebbleDB (standalone) and Raft consensus (cluster)
- **Multiple Interfaces**: CLI and REST API
- **Production Ready**: >90% test coverage, comprehensive validation
- **Cluster Support**: Multi-node distributed consensus with automatic leader election

## Quick Start

### Installation

```bash
git clone https://github.com/jeremyhahn/go-ipam.git
cd go-ipam
make build
```

### Standalone Mode (PebbleDB)

Perfect for development, testing, and single-node deployments:

```bash
# Start standalone server
./ipam server

# Add a network
./ipam network add 192.168.1.0/24 -d "Office network"

# Allocate IPs
./ipam allocate -c 192.168.1.0/24 --hostname web-server
./ipam allocate -c 192.168.1.0/24 -k 5 -d "Load balancer pool"

# List allocations
./ipam list

# View statistics
./ipam stats

# Release an IP
./ipam release 192.168.1.1
```

### Single-Node Cluster (Development)

For testing cluster features in development:

```bash
# Initialize single-node cluster
./ipam cluster init --node-id 1 --cluster-id 100 --raft-addr localhost:5001 --single-node

# Start cluster server
./ipam server --cluster --config ipam-cluster-data/cluster.json
```

### Production Cluster Deployment

#### Option 1: Using Docker Compose (Recommended)

```bash
# Start 3-node cluster with load balancer
make cluster-3-node

# Or manually:
cd examples/cluster
docker-compose up -d
```

This creates:
- 3 IPAM nodes with Raft consensus
- NGINX load balancer on port 8080
- Automatic leader election and failover

#### Option 2: Manual 3-Node Setup

```bash
# Use the automated script
./scripts/3-node-cluster.sh
```

### REST API Usage

```bash
# Add network
curl -X POST http://localhost:8080/api/v1/networks \
  -H "Content-Type: application/json" \
  -d '{"cidr": "10.0.0.0/16", "description": "Production network"}'

# Allocate IP
curl -X POST http://localhost:8080/api/v1/allocations \
  -H "Content-Type: application/json" \
  -d '{"network_id": "net-123", "hostname": "app-server"}'

# Get cluster status (cluster mode only)
curl http://localhost:8080/api/v1/cluster/status

# Health check
curl http://localhost:8080/api/v1/health
```

## Deployment Modes

### 1. Standalone Mode
- **Use case**: Development, testing, single-node deployments
- **Storage**: PebbleDB (embedded key-value store)
- **High availability**: None
- **Performance**: Excellent for single-node workloads

### 2. Single-Node Cluster
- **Use case**: Development testing of cluster features
- **Storage**: Raft consensus (single member)
- **High availability**: None (development only)
- **Performance**: Good for testing

### 3. Multi-Node Cluster
- **Use case**: Production deployments requiring high availability
- **Storage**: Raft consensus across multiple nodes
- **High availability**: Automatic leader election, fault tolerance
- **Performance**: Excellent with load balancing

## Development

### Build and Test

```bash
# Build binary
make build

# Run unit tests
make test

# Run CLI tests
make test-cli

# Run integration tests
make test-integration

# Run tests with race detection
make test-race
```

### Performance Benchmarks

```bash
# Run performance benchmarks
make bench

# Generate test coverage
make coverage
```

## Configuration

### CLI Flags

```bash
# Global flags
--db string      Path to database directory (default "ipam-data")
--cluster        Enable cluster mode

# Server flags
--host string    Server host (default "0.0.0.0")
--port int       Server port (default 8080)
--config string  Path to cluster configuration file
```

### Environment Variables

- `IPAM_DB_PATH`: Database path (overrides --db)
- `IPAM_HOST`: Server host (overrides --host)
- `IPAM_PORT`: Server port (overrides --port)

## API Endpoints

### Networks
- `GET /api/v1/networks` - List networks
- `POST /api/v1/networks` - Create network
- `GET /api/v1/networks/{id}` - Get network
- `DELETE /api/v1/networks/{id}` - Delete network
- `GET /api/v1/networks/{id}/stats` - Network statistics

### Allocations
- `GET /api/v1/allocations` - List allocations
- `POST /api/v1/allocations` - Allocate IP
- `GET /api/v1/allocations/{id}` - Get allocation
- `POST /api/v1/allocations/{id}/release` - Release IP

### Cluster (Cluster mode only)
- `GET /api/v1/cluster/status` - Cluster status
- `POST /api/v1/cluster/nodes` - Add node
- `DELETE /api/v1/cluster/nodes/{nodeID}` - Remove node

### System
- `GET /api/v1/health` - Health check
- `GET /api/v1/audit` - Audit log

## Performance

Benchmarks on Intel Core Ultra 9 285K (24 cores, 93GB RAM, NVMe SSD):

| Mode | Throughput | Memory Usage | Notes |
|------|------------|--------------|-------|
| **CLI** | 52-55 ops/sec | 22KB/1000 IPs | Consistent performance |
| **API** | **351 ops/sec** | 22KB/1000 IPs | **6.8x faster** |
| **Cluster** | 280+ ops/sec | 22KB/1000 IPs | With Raft consensus |
| **Batch** | 590 ops/sec | Optimized | Bulk operations |

## Network Support

- **IPv4**: Classes A-E, all CIDR ranges (/8-/32)
- **IPv6**: All ranges including /128 host routes
- **Special Cases**: /31 point-to-point, /32 host routes
- **Large Networks**: Supports /8 networks (16M+ IPs)

## Production Considerations

### Security
- All API endpoints should be behind authentication in production
- Use TLS/HTTPS for external access
- Secure Raft communication ports (5000-5003) between cluster nodes

### Monitoring
- Health endpoint: `/api/v1/health`
- Cluster status: `/api/v1/cluster/status`
- Audit logging available via API and CLI

### Backup
- **Standalone**: Backup `ipam-data/` directory
- **Cluster**: Backup handled automatically by Raft consensus

## Architecture

- **Core Engine**: Bitmap-based allocation algorithms
- **Storage**: PebbleDB (standalone) or Raft consensus (cluster)
- **CLI**: Cobra-based command line interface
- **API**: RESTful HTTP server with JSON
- **Clustering**: Dragonboat Raft implementation

## License

MIT License