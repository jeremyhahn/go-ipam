#!/bin/bash

set -e

echo "Setting up REAL 3-node IPAM cluster..."

# Clean up any existing data
echo "Cleaning up existing cluster data and processes..."
pkill -f "ipam server" || true
sleep 2
rm -rf node*-data *.log

# Build the binary if it doesn't exist
if [ ! -f "./ipam" ]; then
    echo "Building IPAM binary..."
    make build
fi

# Create configurations for all 3 nodes with the same initial members
echo "Creating node configurations..."

# Node 1 config
mkdir -p node1-data
cat > node1-data/cluster.json <<EOF
{
  "node_id": 1,
  "cluster_id": 100,
  "raft_addr": "localhost:5001",
  "api_addr": "",
  "data_dir": "node1-data",
  "join": false,
  "initial_members": {
    "1": "localhost:5001",
    "2": "localhost:5002",
    "3": "localhost:5003"
  },
  "enable_single_node": false
}
EOF

# Node 2 config
mkdir -p node2-data
cat > node2-data/cluster.json <<EOF
{
  "node_id": 2,
  "cluster_id": 100,
  "raft_addr": "localhost:5002",
  "api_addr": "",
  "data_dir": "node2-data",
  "join": false,
  "initial_members": {
    "1": "localhost:5001",
    "2": "localhost:5002",
    "3": "localhost:5003"
  },
  "enable_single_node": false
}
EOF

# Node 3 config
mkdir -p node3-data
cat > node3-data/cluster.json <<EOF
{
  "node_id": 3,
  "cluster_id": 100,
  "raft_addr": "localhost:5003",
  "api_addr": "",
  "data_dir": "node3-data",
  "join": false,
  "initial_members": {
    "1": "localhost:5001",
    "2": "localhost:5002",
    "3": "localhost:5003"
  },
  "enable_single_node": false
}
EOF

echo ""
echo "Starting all 3 nodes simultaneously..."
echo "This is required for initial cluster formation with Raft"
echo ""

# Start all nodes at roughly the same time
./ipam server --cluster --config node1-data/cluster.json --port 8081 > node1.log 2>&1 &
NODE1_PID=$!
echo "Started Node 1 (PID: $NODE1_PID)"

./ipam server --cluster --config node2-data/cluster.json --port 8082 > node2.log 2>&1 &
NODE2_PID=$!
echo "Started Node 2 (PID: $NODE2_PID)"

./ipam server --cluster --config node3-data/cluster.json --port 8083 > node3.log 2>&1 &
NODE3_PID=$!
echo "Started Node 3 (PID: $NODE3_PID)"

echo ""
echo "Waiting for cluster to form (this may take 10-30 seconds)..."
sleep 10

# Check if processes are still running
if ! kill -0 $NODE1_PID 2>/dev/null; then
    echo "ERROR: Node 1 crashed! Check node1.log"
    tail -20 node1.log
    exit 1
fi

if ! kill -0 $NODE2_PID 2>/dev/null; then
    echo "ERROR: Node 2 crashed! Check node2.log"
    tail -20 node2.log
    exit 1
fi

if ! kill -0 $NODE3_PID 2>/dev/null; then
    echo "ERROR: Node 3 crashed! Check node3.log"
    tail -20 node3.log
    exit 1
fi

echo ""
echo "3-node cluster is running!"
echo "  Node 1: http://localhost:8081 (PID: $NODE1_PID)"
echo "  Node 2: http://localhost:8082 (PID: $NODE2_PID)"
echo "  Node 3: http://localhost:8083 (PID: $NODE3_PID)"
echo ""
echo "Logs:"
echo "  tail -f node1.log"
echo "  tail -f node2.log"
echo "  tail -f node3.log"
echo ""
echo "Test the cluster:"
echo "  curl http://localhost:8081/health"
echo "  curl http://localhost:8082/health"
echo "  curl http://localhost:8083/health"
echo ""
echo "To stop: kill $NODE1_PID $NODE2_PID $NODE3_PID"
echo ""

# Create a stop script
cat > stop-cluster.sh <<EOF
#!/bin/bash
echo "Stopping cluster..."
kill $NODE1_PID $NODE2_PID $NODE3_PID 2>/dev/null || true
echo "Cluster stopped"
EOF
chmod +x stop-cluster.sh

echo "Created stop-cluster.sh to stop the cluster"
echo ""

# Keep script running
echo "Press Ctrl+C to stop the cluster"
trap "kill $NODE1_PID $NODE2_PID $NODE3_PID 2>/dev/null || true; exit" INT TERM
wait
