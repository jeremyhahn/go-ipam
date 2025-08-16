package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// ClusterConfig holds configuration for cluster mode
type ClusterConfig struct {
	// NodeID is the unique identifier for this node (1-based)
	NodeID uint64 `json:"node_id"`

	// ClusterID identifies the IPAM cluster
	ClusterID uint64 `json:"cluster_id"`

	// RaftAddr is the address for Raft communication (e.g., "localhost:5000")
	RaftAddr string `json:"raft_addr"`

	// APIAddr is the address for the API server (e.g., "localhost:8080")
	APIAddr string `json:"api_addr"`

	// DataDir is the directory for storing Raft data
	DataDir string `json:"data_dir"`

	// Join indicates whether this node is joining an existing cluster
	Join bool `json:"join"`

	// InitialMembers is a map of nodeID -> raftAddr for initial cluster members
	// Required when starting a new cluster or joining an existing one
	InitialMembers map[uint64]string `json:"initial_members"`

	// EnableSingleNode allows running a single-node cluster for testing
	EnableSingleNode bool `json:"enable_single_node"`
}

// Validate checks if the cluster configuration is valid
func (c *ClusterConfig) Validate() error {
	if c.NodeID == 0 {
		return fmt.Errorf("node ID must be greater than 0")
	}

	if c.ClusterID == 0 {
		return fmt.Errorf("cluster ID must be greater than 0")
	}

	if c.RaftAddr == "" {
		return fmt.Errorf("raft address is required")
	}

	// Validate Raft address format
	host, port, err := net.SplitHostPort(c.RaftAddr)
	if err != nil {
		return fmt.Errorf("invalid raft address format: %w", err)
	}

	if host == "" {
		return fmt.Errorf("raft address must include hostname or IP")
	}

	portNum, err := strconv.Atoi(port)
	if err != nil || portNum <= 0 || portNum > 65535 {
		return fmt.Errorf("invalid raft port number")
	}

	if c.DataDir == "" {
		return fmt.Errorf("data directory is required")
	}

	// Validate initial members
	if !c.EnableSingleNode && len(c.InitialMembers) == 0 {
		return fmt.Errorf("initial members are required for cluster mode")
	}

	if c.Join && len(c.InitialMembers) == 0 {
		return fmt.Errorf("initial members are required when joining a cluster")
	}

	// Ensure this node is in the initial members list
	if len(c.InitialMembers) > 0 {
		if _, ok := c.InitialMembers[c.NodeID]; !ok && !c.Join {
			return fmt.Errorf("this node (ID %d) must be in the initial members list", c.NodeID)
		}
	}

	// Validate all member addresses
	for nodeID, addr := range c.InitialMembers {
		if nodeID == 0 {
			return fmt.Errorf("node ID in initial members must be greater than 0")
		}
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return fmt.Errorf("invalid address for node %d: %w", nodeID, err)
		}
		if host == "" || port == "" {
			return fmt.Errorf("invalid address for node %d: %s", nodeID, addr)
		}
	}

	return nil
}

// ParseInitialMembers parses a comma-separated list of nodeID:address pairs
// Example: "1:localhost:5000,2:localhost:5001,3:localhost:5002"
func ParseInitialMembers(membersStr string) (map[uint64]string, error) {
	if membersStr == "" {
		return nil, nil
	}

	members := make(map[uint64]string)
	pairs := strings.Split(membersStr, ",")

	for _, pair := range pairs {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid member format: %s (expected nodeID:address)", pair)
		}

		nodeID, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid node ID in %s: %w", pair, err)
		}

		address := strings.TrimSpace(parts[1])
		if address == "" {
			return nil, fmt.Errorf("empty address for node %d", nodeID)
		}

		members[nodeID] = address
	}

	return members, nil
}

// DefaultClusterConfig returns a default cluster configuration for single-node testing
func DefaultClusterConfig() *ClusterConfig {
	return &ClusterConfig{
		NodeID:           1,
		ClusterID:        1,
		RaftAddr:         "localhost:5000",
		APIAddr:          "localhost:8080",
		DataDir:          "ipam-cluster-data",
		Join:             false,
		InitialMembers:   map[uint64]string{1: "localhost:5000"},
		EnableSingleNode: true,
	}
}
