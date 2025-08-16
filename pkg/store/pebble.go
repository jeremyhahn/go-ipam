package store

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/cockroachdb/pebble"
	"github.com/jeremyhahn/go-ipam/pkg/ipam"
)

// PebbleStore implements the Store interface using PebbleDB
type PebbleStore struct {
	db *pebble.DB
	mu sync.RWMutex
}

// Key prefixes for different data types
const (
	prefixNetwork    = "network:"
	prefixAllocation = "allocation:"
	prefixAudit      = "audit:"
	prefixIndex      = "index:"
)

// NewPebbleStore creates a new PebbleDB-based store
func NewPebbleStore(path string) (*PebbleStore, error) {
	opts := &pebble.Options{
		// Optimize for our use case
		L0CompactionThreshold: 2,
		L0StopWritesThreshold: 12,
		LBaseMaxBytes:         64 << 20, // 64 MB
		Levels: []pebble.LevelOptions{
			{TargetFileSize: 2 << 20}, // 2 MB
		},
	}

	db, err := pebble.Open(filepath.Join(path, "ipam.pebble"), opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open PebbleDB: %w", err)
	}

	return &PebbleStore{
		db: db,
	}, nil
}

// Close closes the database
func (s *PebbleStore) Close() error {
	return s.db.Close()
}

// Network operations

func (s *PebbleStore) SaveNetwork(network *ipam.Network) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(network)
	if err != nil {
		return err
	}

	batch := s.db.NewBatch()
	defer batch.Close()

	// Save network
	if err := batch.Set([]byte(prefixNetwork+network.ID), data, nil); err != nil {
		return err
	}

	// Create CIDR index
	if err := batch.Set([]byte(prefixIndex+"cidr:"+network.CIDR), []byte(network.ID), nil); err != nil {
		return err
	}

	return batch.Commit(nil)
}

func (s *PebbleStore) GetNetwork(id string) (*ipam.Network, error) {
	value, closer, err := s.db.Get([]byte(prefixNetwork + id))
	if err == pebble.ErrNotFound {
		return nil, ipam.ErrNetworkNotFound
	}
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	var network ipam.Network
	if err := json.Unmarshal(value, &network); err != nil {
		return nil, err
	}

	return &network, nil
}

func (s *PebbleStore) GetNetworkByCIDR(cidr string) (*ipam.Network, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Look up network ID from CIDR index
	value, closer, err := s.db.Get([]byte(prefixIndex + "cidr:" + cidr))
	if err == pebble.ErrNotFound {
		return nil, ipam.ErrNetworkNotFound
	}
	if err != nil {
		return nil, err
	}
	networkID := string(value)
	closer.Close()

	// Get the network
	return s.GetNetwork(networkID)
}

func (s *PebbleStore) ListNetworks() ([]*ipam.Network, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var networks []*ipam.Network
	iter := s.db.NewIter(&pebble.IterOptions{
		LowerBound: []byte(prefixNetwork),
		UpperBound: []byte(prefixNetwork + "\xff"),
	})
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		var network ipam.Network
		if err := json.Unmarshal(iter.Value(), &network); err != nil {
			return nil, err
		}
		networks = append(networks, &network)
	}

	if err := iter.Error(); err != nil {
		return nil, err
	}

	return networks, nil
}

func (s *PebbleStore) DeleteNetwork(id string) error {
	// Get network to find CIDR for index deletion first (before locking)
	network, err := s.GetNetwork(id)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	batch := s.db.NewBatch()
	defer batch.Close()

	// Delete network
	if err := batch.Delete([]byte(prefixNetwork+id), nil); err != nil {
		return err
	}

	// Delete CIDR index
	if err := batch.Delete([]byte(prefixIndex+"cidr:"+network.CIDR), nil); err != nil {
		return err
	}

	// Delete all allocations for this network
	iter := s.db.NewIter(&pebble.IterOptions{
		LowerBound: []byte(prefixAllocation),
		UpperBound: []byte(prefixAllocation + "\xff"),
	})
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		var allocation ipam.IPAllocation
		if err := json.Unmarshal(iter.Value(), &allocation); err != nil {
			continue
		}
		if allocation.NetworkID == id {
			if err := batch.Delete(iter.Key(), nil); err != nil {
				return err
			}
			// Delete IP index
			indexKey := fmt.Sprintf("%sip:%s:%s", prefixIndex, allocation.NetworkID, allocation.IP)
			if err := batch.Delete([]byte(indexKey), nil); err != nil {
				return err
			}
		}
	}

	return batch.Commit(nil)
}

// Allocation operations

func (s *PebbleStore) SaveAllocation(allocation *ipam.IPAllocation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(allocation)
	if err != nil {
		return err
	}

	batch := s.db.NewBatch()
	defer batch.Close()

	// Save allocation
	if err := batch.Set([]byte(prefixAllocation+allocation.ID), data, nil); err != nil {
		return err
	}

	// Create IP index
	indexKey := fmt.Sprintf("%sip:%s:%s", prefixIndex, allocation.NetworkID, allocation.IP)
	if err := batch.Set([]byte(indexKey), []byte(allocation.ID), nil); err != nil {
		return err
	}

	return batch.Commit(nil)
}

func (s *PebbleStore) GetAllocation(id string) (*ipam.IPAllocation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, closer, err := s.db.Get([]byte(prefixAllocation + id))
	if err == pebble.ErrNotFound {
		return nil, ipam.ErrIPNotAllocated
	}
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	var allocation ipam.IPAllocation
	if err := json.Unmarshal(value, &allocation); err != nil {
		return nil, err
	}

	return &allocation, nil
}

func (s *PebbleStore) GetAllocationByIP(networkID, ip string) (*ipam.IPAllocation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Look up allocation ID from IP index
	indexKey := fmt.Sprintf("%sip:%s:%s", prefixIndex, networkID, ip)
	value, closer, err := s.db.Get([]byte(indexKey))
	if err == pebble.ErrNotFound {
		return nil, ipam.ErrIPNotAllocated
	}
	if err != nil {
		return nil, err
	}
	allocationID := string(value)
	closer.Close()

	// Get the allocation
	return s.GetAllocation(allocationID)
}

func (s *PebbleStore) ListAllocations(networkID string) ([]*ipam.IPAllocation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var allocations []*ipam.IPAllocation
	iter := s.db.NewIter(&pebble.IterOptions{
		LowerBound: []byte(prefixAllocation),
		UpperBound: []byte(prefixAllocation + "\xff"),
	})
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		var allocation ipam.IPAllocation
		if err := json.Unmarshal(iter.Value(), &allocation); err != nil {
			return nil, err
		}
		if allocation.NetworkID == networkID {
			allocations = append(allocations, &allocation)
		}
	}

	if err := iter.Error(); err != nil {
		return nil, err
	}

	return allocations, nil
}

func (s *PebbleStore) DeleteAllocation(id string) error {
	// Get allocation to find IP for index deletion first (before locking)
	allocation, err := s.GetAllocation(id)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	batch := s.db.NewBatch()
	defer batch.Close()

	// Delete allocation
	if err := batch.Delete([]byte(prefixAllocation+id), nil); err != nil {
		return err
	}

	// Delete IP index
	indexKey := fmt.Sprintf("%sip:%s:%s", prefixIndex, allocation.NetworkID, allocation.IP)
	if err := batch.Delete([]byte(indexKey), nil); err != nil {
		return err
	}

	return batch.Commit(nil)
}

// Audit operations

func (s *PebbleStore) SaveAuditEntry(entry *ipam.AuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	// Use timestamp as part of key for natural ordering
	key := fmt.Sprintf("%s%d_%s", prefixAudit, entry.Timestamp.UnixNano(), entry.ID)
	return s.db.Set([]byte(key), data, nil)
}

func (s *PebbleStore) ListAuditEntries(limit int) ([]*ipam.AuditEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entries []*ipam.AuditEntry

	// Iterate in reverse order (most recent first)
	iter := s.db.NewIter(&pebble.IterOptions{
		LowerBound: []byte(prefixAudit),
		UpperBound: []byte(prefixAudit + "\xff"),
	})
	defer iter.Close()

	// Collect all entries first
	var allEntries []*ipam.AuditEntry
	for iter.First(); iter.Valid(); iter.Next() {
		var entry ipam.AuditEntry
		if err := json.Unmarshal(iter.Value(), &entry); err != nil {
			return nil, err
		}
		allEntries = append(allEntries, &entry)
	}

	if err := iter.Error(); err != nil {
		return nil, err
	}

	// Return the last 'limit' entries (most recent)
	start := len(allEntries) - limit
	if start < 0 {
		start = 0
	}

	// Reverse order to get most recent first
	for i := len(allEntries) - 1; i >= start; i-- {
		entries = append(entries, allEntries[i])
	}

	return entries, nil
}

// Helper method to get store statistics
func (s *PebbleStore) GetStats() (*pebble.Metrics, error) {
	return s.db.Metrics(), nil
}
