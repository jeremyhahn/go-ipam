# IPAM Documentation

Complete documentation for the IPAM (IP Address Management) system.

## Getting Started

- **[README.md](../README.md)** - Project overview and quick start guide
- **[Installation](../README.md#installation)** - Build and installation instructions
- **[Basic Usage](../README.md#basic-usage)** - CLI and API examples

## Deployment Guides

- **[Deployment Guide](DEPLOYMENT.md)** - Production deployment with security and monitoring
- **[Cluster Setup](3-node-cluster-setup.md)** - Multi-node cluster configuration
- **[Docker Compose](../examples/cluster/)** - Containerized cluster deployment

## API Reference

- **[REST API](API.md)** - Complete API endpoint reference
- **[Examples](API.md#examples)** - Code examples and workflows

## Architecture

### Core Components

- **CLI Interface** - Command-line tool for network and IP management
- **REST API Server** - HTTP/JSON API for programmatic access
- **Storage Layer** - PebbleDB (standalone) or Raft consensus (cluster)
- **IPAM Engine** - Bitmap-based IP allocation algorithms

### Deployment Modes

1. **Standalone Mode**
   - Single node with PebbleDB storage
   - Perfect for development and small deployments
   - No high availability

2. **Single-Node Cluster**
   - Development testing of cluster features
   - Raft consensus with single member
   - For testing failover scenarios

3. **Multi-Node Cluster**
   - Production high-availability deployment
   - Distributed Raft consensus
   - Automatic leader election and failover

## Feature Overview

### Network Management
- IPv4 and IPv6 support
- CIDR network allocation
- Network tagging and metadata
- Utilization statistics

### IP Allocation
- Automatic IP assignment
- Bulk allocation support
- TTL-based expiration
- Hostname and description metadata
- Range allocation for load balancers

### Cluster Features
- Multi-node Raft consensus
- Automatic leader election
- Dynamic node management
- Load balancer integration
- Fault tolerance

### Monitoring and Audit
- Health check endpoints
- Comprehensive audit logging
- Cluster status monitoring
- Performance metrics

## Configuration Reference

### Command Line Options

```bash
# Global flags
--db string      Database directory path (default: "ipam-data")
--cluster        Enable cluster mode

# Server flags  
--host string    Server host (default: "0.0.0.0")
--port int       Server port (default: 8080)
--config string  Cluster configuration file path

# Cluster initialization
--node-id uint64        Unique node identifier
--cluster-id uint64     Cluster identifier
--raft-addr string      Raft communication address
--data-dir string       Data directory path
--initial-members       Comma-separated member list
--single-node          Initialize as single-node cluster
```

### Environment Variables

```bash
IPAM_DB_PATH     # Database path (overrides --db)
IPAM_HOST        # Server host (overrides --host)  
IPAM_PORT        # Server port (overrides --port)
```

### Configuration Files

Cluster configuration is stored in JSON format:

```json
{
  "node_id": 1,
  "cluster_id": 100,
  "raft_addr": "localhost:5001",
  "data_dir": "ipam-cluster-data",
  "initial_members": [
    {"node_id": 1, "addr": "localhost:5001"},
    {"node_id": 2, "addr": "localhost:5002"},
    {"node_id": 3, "addr": "localhost:5003"}
  ]
}
```

## Development

### Building from Source

```bash
git clone https://github.com/jeremyhahn/go-ipam.git
cd go-ipam
make build
```

### Running Tests

```bash
make test           # Unit tests
make test-cli       # CLI tests  
make test-race      # Race condition detection
make bench          # Performance benchmarks
make coverage       # Test coverage report
```

### Development Workflow

```bash
# Format and lint
make fmt
make vet
make lint

# Build and test
make build
make test

# Run locally
./ipam server
```

## Performance Characteristics

### Benchmarks

Tested on Intel Core Ultra 9 285K (24 cores, 93GB RAM, NVMe SSD):

| Mode | Throughput | Memory Usage | Notes |
|------|------------|--------------|-------|
| CLI | 52-55 ops/sec | 22KB/1000 IPs | Consistent performance |
| API | 351 ops/sec | 22KB/1000 IPs | 6.8x faster than CLI |
| Cluster | 280+ ops/sec | 22KB/1000 IPs | With Raft consensus |
| Batch | 590 ops/sec | Optimized | Bulk operations |

### Scaling Limits

- **Networks**: Unlimited (limited by storage)
- **IPs per network**: Up to 16M+ IPs (/8 networks)
- **Allocations**: Unlimited (limited by storage)
- **Cluster nodes**: Recommended 3-5 nodes for optimal performance

## Security Considerations

### Production Security

- **Authentication**: Implement at reverse proxy level
- **TLS**: Use HTTPS for all external access
- **Firewall**: Secure Raft ports between cluster nodes
- **User isolation**: Run with dedicated system user
- **File permissions**: Restrict database directory access

### Network Security

- Raft communication ports (5000-5003) should only be accessible between cluster nodes
- API port (8080) should be behind authentication and TLS
- Consider network segmentation for cluster communication

## Troubleshooting

### Common Issues

1. **Database permission errors**
   - Check file ownership and permissions
   - Ensure adequate disk space

2. **Cluster connectivity issues**
   - Verify Raft port accessibility
   - Check initial member configuration consistency
   - Ensure unique node IDs

3. **API request failures**
   - Verify cluster leader status
   - Check network connectivity
   - Review error logs

4. **Performance issues**
   - Monitor memory and CPU usage
   - Check disk I/O performance
   - Review network latency between nodes

### Debug Commands

```bash
# Check system health
curl http://localhost:8080/api/v1/health

# Cluster status
curl http://localhost:8080/api/v1/cluster/status

# View logs
journalctl -u ipam -f

# Database statistics
./ipam stats
```

## Support and Contributing

### Getting Help

- Check this documentation for common solutions
- Review the troubleshooting section
- Examine application logs for error details

### Contributing

- Follow Go coding standards
- Add tests for new features
- Update documentation for changes
- Ensure all tests pass before submitting

## License

This project is licensed under the MIT License - see the LICENSE file for details.