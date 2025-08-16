package store

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/jeremyhahn/go-ipam/pkg/ipam"
	"github.com/lni/dragonboat/v3"
	"github.com/lni/dragonboat/v3/config"
	"github.com/lni/dragonboat/v3/logger"
	sm "github.com/lni/dragonboat/v3/statemachine"
)

// ClusterInfo contains information about the Raft cluster
type ClusterInfo struct {
	ClusterID      uint64     `json:"cluster_id"`
	LeaderID       uint64     `json:"leader_id"`
	HasLeader      bool       `json:"has_leader"`
	Nodes          []NodeInfo `json:"nodes"`
	ConfigChangeID uint64     `json:"config_change_id"`
}

// NodeInfo contains information about a cluster node
type NodeInfo struct {
	NodeID   uint64 `json:"node_id"`
	RaftAddr string `json:"raft_addr"`
	IsLeader bool   `json:"is_leader"`
}

// RaftStore implements the Store interface using Dragonboat Raft
type RaftStore struct {
	nodeID    uint64
	clusterID uint64
	nh        *dragonboat.NodeHost
	mu        sync.RWMutex
}

// NewRaftStore creates a new Raft-based store
func NewRaftStore(nodeID, clusterID uint64, nodeAddr string, join bool, initialMembers map[uint64]string, dataDir string) (*RaftStore, error) {
	// Configure Dragonboat
	nhc := config.NodeHostConfig{
		NodeHostDir:    filepath.Join(dataDir, fmt.Sprintf("node-%d", nodeID)),
		RTTMillisecond: 200,
		RaftAddress:    nodeAddr,
	}

	// Disable default logger to reduce noise
	logger.GetLogger("raft").SetLevel(logger.ERROR)
	logger.GetLogger("rsm").SetLevel(logger.ERROR)
	logger.GetLogger("transport").SetLevel(logger.ERROR)
	logger.GetLogger("grpc").SetLevel(logger.ERROR)

	nh, err := dragonboat.NewNodeHost(nhc)
	if err != nil {
		return nil, fmt.Errorf("failed to create NodeHost: %w", err)
	}

	// Configure the Raft cluster
	rc := config.Config{
		NodeID:             nodeID,
		ClusterID:          clusterID,
		ElectionRTT:        10,
		HeartbeatRTT:       1,
		CheckQuorum:        true,
		SnapshotEntries:    10000,
		CompactionOverhead: 5000,
	}

	// Create the state machine factory
	factory := func(clusterID, nodeID uint64) sm.IStateMachine {
		return newIPAMStateMachine(clusterID, nodeID)
	}

	// Start or join the cluster
	if join {
		if err := nh.StartCluster(initialMembers, join, factory, rc); err != nil {
			nh.Stop()
			return nil, fmt.Errorf("failed to join cluster: %w", err)
		}
	} else {
		if err := nh.StartCluster(initialMembers, false, factory, rc); err != nil {
			nh.Stop()
			return nil, fmt.Errorf("failed to start cluster: %w", err)
		}
	}

	return &RaftStore{
		nodeID:    nodeID,
		clusterID: clusterID,
		nh:        nh,
	}, nil
}

// Close shuts down the Raft store
func (s *RaftStore) Close() error {
	if s.nh != nil {
		s.nh.Stop()
	}
	return nil
}

// executeCommand submits a command to the Raft cluster
func (s *RaftStore) executeCommand(cmdType commandType, cmd interface{}) error {
	cmdData, err := encode(cmd)
	if err != nil {
		return err
	}

	// Prepend command type
	data := append([]byte{byte(cmdType)}, cmdData...)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	session := s.nh.GetNoOPSession(s.clusterID)
	_, err = s.nh.SyncPropose(ctx, session, data)
	return err
}

// executeQuery performs a read-only query
func (s *RaftStore) executeQuery(queryType queryType, query interface{}) (interface{}, error) {
	queryData, err := encode(query)
	if err != nil {
		return nil, err
	}

	// Prepend query type
	data := append([]byte{byte(queryType)}, queryData...)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := s.nh.SyncRead(ctx, s.clusterID, data)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Network operations

func (s *RaftStore) SaveNetwork(network *ipam.Network) error {
	cmd := &saveNetworkCmd{Network: network}
	return s.executeCommand(cmdSaveNetwork, cmd)
}

func (s *RaftStore) GetNetwork(id string) (*ipam.Network, error) {
	query := &getNetworkQuery{ID: id}
	result, err := s.executeQuery(queryGetNetwork, query)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, ipam.ErrNetworkNotFound
	}

	return result.(*ipam.Network), nil
}

func (s *RaftStore) GetNetworkByCIDR(cidr string) (*ipam.Network, error) {
	query := &getNetworkByCIDRQuery{CIDR: cidr}
	result, err := s.executeQuery(queryGetNetworkByCIDR, query)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, ipam.ErrNetworkNotFound
	}

	return result.(*ipam.Network), nil
}

func (s *RaftStore) ListNetworks() ([]*ipam.Network, error) {
	query := &listNetworksQuery{}
	result, err := s.executeQuery(queryListNetworks, query)
	if err != nil {
		return nil, err
	}

	return result.([]*ipam.Network), nil
}

func (s *RaftStore) DeleteNetwork(id string) error {
	cmd := &deleteNetworkCmd{ID: id}
	return s.executeCommand(cmdDeleteNetwork, cmd)
}

// Allocation operations

func (s *RaftStore) SaveAllocation(allocation *ipam.IPAllocation) error {
	cmd := &saveAllocationCmd{Allocation: allocation}
	return s.executeCommand(cmdSaveAllocation, cmd)
}

func (s *RaftStore) GetAllocation(id string) (*ipam.IPAllocation, error) {
	query := &getAllocationQuery{ID: id}
	result, err := s.executeQuery(queryGetAllocation, query)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, ipam.ErrIPNotAllocated
	}

	return result.(*ipam.IPAllocation), nil
}

func (s *RaftStore) GetAllocationByIP(networkID, ip string) (*ipam.IPAllocation, error) {
	query := &getAllocationByIPQuery{NetworkID: networkID, IP: ip}
	result, err := s.executeQuery(queryGetAllocationByIP, query)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, ipam.ErrIPNotAllocated
	}

	return result.(*ipam.IPAllocation), nil
}

func (s *RaftStore) ListAllocations(networkID string) ([]*ipam.IPAllocation, error) {
	query := &listAllocationsQuery{NetworkID: networkID}
	result, err := s.executeQuery(queryListAllocations, query)
	if err != nil {
		return nil, err
	}

	return result.([]*ipam.IPAllocation), nil
}

func (s *RaftStore) DeleteAllocation(id string) error {
	cmd := &deleteAllocationCmd{ID: id}
	return s.executeCommand(cmdDeleteAllocation, cmd)
}

// Audit operations

func (s *RaftStore) SaveAuditEntry(entry *ipam.AuditEntry) error {
	cmd := &saveAuditCmd{Entry: entry}
	return s.executeCommand(cmdSaveAudit, cmd)
}

func (s *RaftStore) ListAuditEntries(limit int) ([]*ipam.AuditEntry, error) {
	query := &listAuditQuery{Limit: limit}
	result, err := s.executeQuery(queryListAudit, query)
	if err != nil {
		return nil, err
	}

	return result.([]*ipam.AuditEntry), nil
}

// GetClusterInfo returns information about the Raft cluster
func (s *RaftStore) GetClusterInfo() (*ClusterInfo, error) {
	leader, ok, err := s.nh.GetLeaderID(s.clusterID)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	membership, err := s.nh.SyncGetClusterMembership(ctx, s.clusterID)
	if err != nil {
		return nil, err
	}

	nodes := make([]NodeInfo, 0, len(membership.Nodes))
	for nodeID, addr := range membership.Nodes {
		nodes = append(nodes, NodeInfo{
			NodeID:   nodeID,
			RaftAddr: addr,
			IsLeader: nodeID == leader,
		})
	}

	return &ClusterInfo{
		ClusterID:      s.clusterID,
		LeaderID:       leader,
		HasLeader:      ok,
		Nodes:          nodes,
		ConfigChangeID: membership.ConfigChangeID,
	}, nil
}

// AddNode adds a new node to the cluster
func (s *RaftStore) AddNode(nodeID uint64, addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.nh.SyncRequestAddNode(ctx, s.clusterID, nodeID, addr, 0)
}

// RemoveNode removes a node from the cluster
func (s *RaftStore) RemoveNode(nodeID uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.nh.SyncRequestDeleteNode(ctx, s.clusterID, nodeID, 0)
}

// Helper functions

func encode(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decode(data []byte, v interface{}) error {
	dec := gob.NewDecoder(bytes.NewReader(data))
	return dec.Decode(v)
}
