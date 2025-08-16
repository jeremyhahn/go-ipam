#!/bin/bash
# Script to set up a single-node cluster for testing

set -e

echo "Setting up single-node IPAM cluster..."

# Clean up any existing data
echo "Cleaning up existing cluster data..."
pkill -f "ipam server" || true
sleep 1
rm -rf ipam-cluster-data *.log

# Build the binary if it doesn't exist
if [ ! -f "./ipam" ]; then
    echo "Building IPAM binary..."
    make build
fi

# Initialize single-node cluster
echo "Initializing single-node cluster..."
./ipam cluster init \
  --node-id 1 \
  --cluster-id 100 \
  --raft-addr localhost:5001 \
  --single-node

echo ""
echo "Cluster configuration complete!"
echo ""
echo "To start the cluster server, run:"
echo "  ./ipam server --cluster --config ipam-cluster-data/cluster.json"
echo ""
echo "Or use:"
echo "  make server-cluster"
echo ""