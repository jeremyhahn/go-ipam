package store

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jeremyhahn/go-ipam/pkg/ipam"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestRaftStore(t *testing.T, nodeID uint64) (*RaftStore, func()) {
	tempDir := t.TempDir()

	// Create single node cluster for testing
	members := map[uint64]string{
		nodeID: fmt.Sprintf("localhost:%d", 5000+nodeID),
	}

	store, err := NewRaftStore(
		nodeID,
		1, // cluster ID
		members[nodeID],
		false, // not joining
		members,
		tempDir,
	)
	require.NoError(t, err)

	// Wait for cluster to be ready
	var clusterReady bool
	for i := 0; i < 10; i++ {
		info, err := store.GetClusterInfo()
		if err == nil && info.HasLeader {
			clusterReady = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !clusterReady {
		t.Fatal("cluster failed to elect leader")
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tempDir)
	}

	return store, cleanup
}

func TestRaftStoreNetworkOperations(t *testing.T) {
	t.Skip("Skipping Raft integration test for now")
	store, cleanup := createTestRaftStore(t, 1)
	defer cleanup()

	// Test SaveNetwork
	network := &ipam.Network{
		ID:          "net1",
		CIDR:        "192.168.1.0/24",
		Description: "Test network",
		Tags:        []string{"test", "raft"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := store.SaveNetwork(network)
	require.NoError(t, err)

	// Test GetNetwork
	retrieved, err := store.GetNetwork("net1")
	require.NoError(t, err)
	assert.Equal(t, network.ID, retrieved.ID)
	assert.Equal(t, network.CIDR, retrieved.CIDR)
	assert.Equal(t, network.Description, retrieved.Description)

	// Test GetNetworkByCIDR
	byCIDR, err := store.GetNetworkByCIDR("192.168.1.0/24")
	require.NoError(t, err)
	assert.Equal(t, network.ID, byCIDR.ID)

	// Test ListNetworks
	networks, err := store.ListNetworks()
	require.NoError(t, err)
	assert.Len(t, networks, 1)

	// Test DeleteNetwork
	err = store.DeleteNetwork("net1")
	require.NoError(t, err)

	_, err = store.GetNetwork("net1")
	assert.Error(t, err)
}

func TestRaftStoreAllocationOperations(t *testing.T) {
	t.Skip("Skipping Raft integration test for now")
	store, cleanup := createTestRaftStore(t, 1)
	defer cleanup()

	// First create a network
	network := &ipam.Network{
		ID:        "net1",
		CIDR:      "10.0.0.0/24",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := store.SaveNetwork(network)
	require.NoError(t, err)

	// Test SaveAllocation
	allocation := &ipam.IPAllocation{
		ID:          "alloc1",
		NetworkID:   "net1",
		IP:          "10.0.0.10",
		Description: "Test allocation",
		Status:      "allocated",
		AllocatedAt: time.Now(),
	}

	err = store.SaveAllocation(allocation)
	require.NoError(t, err)

	// Test GetAllocation
	retrieved, err := store.GetAllocation("alloc1")
	require.NoError(t, err)
	assert.Equal(t, allocation.ID, retrieved.ID)
	assert.Equal(t, allocation.IP, retrieved.IP)

	// Test GetAllocationByIP
	byIP, err := store.GetAllocationByIP("net1", "10.0.0.10")
	require.NoError(t, err)
	assert.Equal(t, allocation.ID, byIP.ID)

	// Test ListAllocations
	allocations, err := store.ListAllocations("net1")
	require.NoError(t, err)
	assert.Len(t, allocations, 1)

	// Test DeleteAllocation
	err = store.DeleteAllocation("alloc1")
	require.NoError(t, err)

	_, err = store.GetAllocation("alloc1")
	assert.ErrorIs(t, err, ipam.ErrIPNotAllocated)
}

func TestRaftStoreAuditOperations(t *testing.T) {
	store, cleanup := createTestRaftStore(t, 1)
	defer cleanup()

	// Add audit entries
	for i := 0; i < 5; i++ {
		entry := &ipam.AuditEntry{
			ID:        fmt.Sprintf("audit%d", i),
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Action:    "test_action",
			Resource:  fmt.Sprintf("resource%d", i),
			Details:   fmt.Sprintf("Test audit %d", i),
			User:      "test_user",
		}
		err := store.SaveAuditEntry(entry)
		require.NoError(t, err)
	}

	// Test ListAuditEntries
	entries, err := store.ListAuditEntries(3)
	require.NoError(t, err)
	assert.Len(t, entries, 3)

	// Verify order (most recent first)
	assert.Equal(t, "audit4", entries[0].ID)
	assert.Equal(t, "audit3", entries[1].ID)
	assert.Equal(t, "audit2", entries[2].ID)
}

func TestRaftStoreClusterInfo(t *testing.T) {
	store, cleanup := createTestRaftStore(t, 1)
	defer cleanup()

	info, err := store.GetClusterInfo()
	require.NoError(t, err)

	assert.Equal(t, uint64(1), info.ClusterID)
	assert.True(t, info.HasLeader)
	assert.Equal(t, uint64(1), info.LeaderID)
	assert.Len(t, info.Nodes, 1)
	assert.Equal(t, uint64(1), info.Nodes[0].NodeID)
	assert.True(t, info.Nodes[0].IsLeader)
}

func TestRaftStoreConsistency(t *testing.T) {
	store, cleanup := createTestRaftStore(t, 1)
	defer cleanup()

	// Create multiple networks rapidly
	for i := 0; i < 10; i++ {
		network := &ipam.Network{
			ID:        fmt.Sprintf("net%d", i),
			CIDR:      fmt.Sprintf("10.%d.0.0/24", i),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := store.SaveNetwork(network)
		require.NoError(t, err)
	}

	// Verify all were saved
	networks, err := store.ListNetworks()
	require.NoError(t, err)
	assert.Len(t, networks, 10)

	// Create allocations
	for i := 0; i < 10; i++ {
		allocation := &ipam.IPAllocation{
			ID:          fmt.Sprintf("alloc%d", i),
			NetworkID:   fmt.Sprintf("net%d", i),
			IP:          fmt.Sprintf("10.%d.0.1", i),
			Status:      "allocated",
			AllocatedAt: time.Now(),
		}
		err := store.SaveAllocation(allocation)
		require.NoError(t, err)
	}

	// Verify allocations
	for i := 0; i < 10; i++ {
		allocations, err := store.ListAllocations(fmt.Sprintf("net%d", i))
		require.NoError(t, err)
		assert.Len(t, allocations, 1)
		assert.Equal(t, fmt.Sprintf("10.%d.0.1", i), allocations[0].IP)
	}
}
