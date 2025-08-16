package store

import (
	"encoding/gob"
	"fmt"
	"io"
	"sync"

	"github.com/jeremyhahn/go-ipam/pkg/ipam"
	sm "github.com/lni/dragonboat/v3/statemachine"
)

func init() {
	// Register types for gob encoding
	gob.Register(&saveNetworkCmd{})
	gob.Register(&deleteNetworkCmd{})
	gob.Register(&saveAllocationCmd{})
	gob.Register(&deleteAllocationCmd{})
	gob.Register(&saveAuditCmd{})
	gob.Register(&getNetworkQuery{})
	gob.Register(&getNetworkByCIDRQuery{})
	gob.Register(&listNetworksQuery{})
	gob.Register(&getAllocationQuery{})
	gob.Register(&getAllocationByIPQuery{})
	gob.Register(&listAllocationsQuery{})
	gob.Register(&listAuditQuery{})
}

// Command types
type commandType uint8

const (
	cmdSaveNetwork commandType = iota
	cmdDeleteNetwork
	cmdSaveAllocation
	cmdDeleteAllocation
	cmdSaveAudit
)

// Query types
type queryType uint8

const (
	queryGetNetwork queryType = iota
	queryGetNetworkByCIDR
	queryListNetworks
	queryGetAllocation
	queryGetAllocationByIP
	queryListAllocations
	queryListAudit
)

// Commands
type saveNetworkCmd struct {
	Network *ipam.Network
}

type deleteNetworkCmd struct {
	ID string
}

type saveAllocationCmd struct {
	Allocation *ipam.IPAllocation
}

type deleteAllocationCmd struct {
	ID string
}

type saveAuditCmd struct {
	Entry *ipam.AuditEntry
}

// Queries
type getNetworkQuery struct {
	ID string
}

type getNetworkByCIDRQuery struct {
	CIDR string
}

type listNetworksQuery struct{}

type getAllocationQuery struct {
	ID string
}

type getAllocationByIPQuery struct {
	NetworkID string
	IP        string
}

type listAllocationsQuery struct {
	NetworkID string
}

type listAuditQuery struct {
	Limit int
}

// ipamStateMachine implements the Raft state machine for IPAM
type ipamStateMachine struct {
	clusterID uint64
	nodeID    uint64

	mu          sync.RWMutex
	networks    map[string]*ipam.Network
	allocations map[string]*ipam.IPAllocation
	audit       []*ipam.AuditEntry

	// Indexes for fast lookup
	networkByCIDR    map[string]string   // CIDR -> Network ID
	allocationByIP   map[string]string   // NetworkID:IP -> Allocation ID
	allocationsByNet map[string][]string // Network ID -> Allocation IDs
}

func newIPAMStateMachine(clusterID, nodeID uint64) sm.IStateMachine {
	return &ipamStateMachine{
		clusterID:        clusterID,
		nodeID:           nodeID,
		networks:         make(map[string]*ipam.Network),
		allocations:      make(map[string]*ipam.IPAllocation),
		audit:            make([]*ipam.AuditEntry, 0),
		networkByCIDR:    make(map[string]string),
		allocationByIP:   make(map[string]string),
		allocationsByNet: make(map[string][]string),
	}
}

// Update applies a command to the state machine
func (s *ipamStateMachine) Update(data []byte) (sm.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.applyEntry(data)
	if err != nil {
		return sm.Result{
			Value: 0,
			Data:  []byte(err.Error()),
		}, err
	}

	return sm.Result{
		Value: 1,
		Data:  result,
	}, nil
}

// Lookup performs a read-only query
func (s *ipamStateMachine) Lookup(query interface{}) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := query.([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid query type")
	}

	// Decode the query type
	if len(data) < 1 {
		return nil, fmt.Errorf("empty query")
	}

	queryType := queryType(data[0])
	queryData := data[1:]

	switch queryType {
	case queryGetNetwork:
		var q getNetworkQuery
		if err := decode(queryData, &q); err != nil {
			return nil, err
		}
		return s.networks[q.ID], nil

	case queryGetNetworkByCIDR:
		var q getNetworkByCIDRQuery
		if err := decode(queryData, &q); err != nil {
			return nil, err
		}
		if id, ok := s.networkByCIDR[q.CIDR]; ok {
			return s.networks[id], nil
		}
		return nil, nil

	case queryListNetworks:
		networks := make([]*ipam.Network, 0, len(s.networks))
		for _, n := range s.networks {
			networks = append(networks, n)
		}
		return networks, nil

	case queryGetAllocation:
		var q getAllocationQuery
		if err := decode(queryData, &q); err != nil {
			return nil, err
		}
		return s.allocations[q.ID], nil

	case queryGetAllocationByIP:
		var q getAllocationByIPQuery
		if err := decode(queryData, &q); err != nil {
			return nil, err
		}
		key := fmt.Sprintf("%s:%s", q.NetworkID, q.IP)
		if id, ok := s.allocationByIP[key]; ok {
			return s.allocations[id], nil
		}
		return nil, nil

	case queryListAllocations:
		var q listAllocationsQuery
		if err := decode(queryData, &q); err != nil {
			return nil, err
		}
		allocIDs := s.allocationsByNet[q.NetworkID]
		allocations := make([]*ipam.IPAllocation, 0, len(allocIDs))
		for _, id := range allocIDs {
			if alloc, ok := s.allocations[id]; ok {
				allocations = append(allocations, alloc)
			}
		}
		return allocations, nil

	case queryListAudit:
		var q listAuditQuery
		if err := decode(queryData, &q); err != nil {
			return nil, err
		}
		// Return most recent entries first
		start := len(s.audit) - q.Limit
		if start < 0 || q.Limit <= 0 {
			start = 0
		}
		result := make([]*ipam.AuditEntry, 0, q.Limit)
		for i := len(s.audit) - 1; i >= start && i >= 0; i-- {
			result = append(result, s.audit[i])
		}
		return result, nil

	default:
		return nil, fmt.Errorf("unknown query type: %d", queryType)
	}
}

// SaveSnapshot saves the state machine's state
func (s *ipamStateMachine) SaveSnapshot(w io.Writer, fc sm.ISnapshotFileCollection, done <-chan struct{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create snapshot data
	snapshot := &snapshotData{
		Networks:    s.networks,
		Allocations: s.allocations,
		Audit:       s.audit,
	}

	// Encode and write
	enc := gob.NewEncoder(w)
	return enc.Encode(snapshot)
}

// RecoverFromSnapshot restores the state machine from a snapshot
func (s *ipamStateMachine) RecoverFromSnapshot(r io.Reader, files []sm.SnapshotFile, done <-chan struct{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Decode snapshot
	var snapshot snapshotData
	dec := gob.NewDecoder(r)
	if err := dec.Decode(&snapshot); err != nil {
		return err
	}

	// Restore state
	s.networks = snapshot.Networks
	s.allocations = snapshot.Allocations
	s.audit = snapshot.Audit

	// Rebuild indexes
	s.rebuildIndexes()

	return nil
}

// Close cleans up the state machine
func (s *ipamStateMachine) Close() error {
	return nil
}

// applyEntry applies a single command
func (s *ipamStateMachine) applyEntry(cmd []byte) ([]byte, error) {
	if len(cmd) < 1 {
		return nil, fmt.Errorf("empty command")
	}

	cmdType := commandType(cmd[0])
	cmdData := cmd[1:]

	switch cmdType {
	case cmdSaveNetwork:
		var c saveNetworkCmd
		if err := decode(cmdData, &c); err != nil {
			return nil, err
		}
		s.networks[c.Network.ID] = c.Network
		s.networkByCIDR[c.Network.CIDR] = c.Network.ID
		return nil, nil

	case cmdDeleteNetwork:
		var c deleteNetworkCmd
		if err := decode(cmdData, &c); err != nil {
			return nil, err
		}
		if network, ok := s.networks[c.ID]; ok {
			delete(s.networks, c.ID)
			delete(s.networkByCIDR, network.CIDR)
			// Also remove allocations for this network
			if allocIDs, ok := s.allocationsByNet[c.ID]; ok {
				for _, allocID := range allocIDs {
					if alloc, ok := s.allocations[allocID]; ok {
						delete(s.allocations, allocID)
						key := fmt.Sprintf("%s:%s", alloc.NetworkID, alloc.IP)
						delete(s.allocationByIP, key)
					}
				}
				delete(s.allocationsByNet, c.ID)
			}
		}
		return nil, nil

	case cmdSaveAllocation:
		var c saveAllocationCmd
		if err := decode(cmdData, &c); err != nil {
			return nil, err
		}
		alloc := c.Allocation
		s.allocations[alloc.ID] = alloc

		// Update indexes
		key := fmt.Sprintf("%s:%s", alloc.NetworkID, alloc.IP)
		s.allocationByIP[key] = alloc.ID

		// Add to network's allocation list
		if _, exists := s.allocationsByNet[alloc.NetworkID]; !exists {
			s.allocationsByNet[alloc.NetworkID] = []string{}
		}
		// Check if already in list
		found := false
		for _, id := range s.allocationsByNet[alloc.NetworkID] {
			if id == alloc.ID {
				found = true
				break
			}
		}
		if !found {
			s.allocationsByNet[alloc.NetworkID] = append(s.allocationsByNet[alloc.NetworkID], alloc.ID)
		}

		return nil, nil

	case cmdDeleteAllocation:
		var c deleteAllocationCmd
		if err := decode(cmdData, &c); err != nil {
			return nil, err
		}
		if alloc, ok := s.allocations[c.ID]; ok {
			delete(s.allocations, c.ID)
			key := fmt.Sprintf("%s:%s", alloc.NetworkID, alloc.IP)
			delete(s.allocationByIP, key)

			// Remove from network's allocation list
			if allocIDs, ok := s.allocationsByNet[alloc.NetworkID]; ok {
				newList := make([]string, 0, len(allocIDs))
				for _, id := range allocIDs {
					if id != c.ID {
						newList = append(newList, id)
					}
				}
				s.allocationsByNet[alloc.NetworkID] = newList
			}
		}
		return nil, nil

	case cmdSaveAudit:
		var c saveAuditCmd
		if err := decode(cmdData, &c); err != nil {
			return nil, err
		}
		s.audit = append(s.audit, c.Entry)
		// Keep only last 10000 entries
		if len(s.audit) > 10000 {
			s.audit = s.audit[len(s.audit)-10000:]
		}
		return nil, nil

	default:
		return nil, fmt.Errorf("unknown command type: %d", cmdType)
	}
}

// rebuildIndexes rebuilds the lookup indexes after snapshot recovery
func (s *ipamStateMachine) rebuildIndexes() {
	s.networkByCIDR = make(map[string]string)
	s.allocationByIP = make(map[string]string)
	s.allocationsByNet = make(map[string][]string)

	// Rebuild network index
	for id, network := range s.networks {
		s.networkByCIDR[network.CIDR] = id
	}

	// Rebuild allocation indexes
	for id, alloc := range s.allocations {
		key := fmt.Sprintf("%s:%s", alloc.NetworkID, alloc.IP)
		s.allocationByIP[key] = id

		if _, exists := s.allocationsByNet[alloc.NetworkID]; !exists {
			s.allocationsByNet[alloc.NetworkID] = []string{}
		}
		s.allocationsByNet[alloc.NetworkID] = append(s.allocationsByNet[alloc.NetworkID], id)
	}
}

// snapshotData holds the complete state for snapshots
type snapshotData struct {
	Networks    map[string]*ipam.Network
	Allocations map[string]*ipam.IPAllocation
	Audit       []*ipam.AuditEntry
}
