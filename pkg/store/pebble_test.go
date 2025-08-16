package store

import (
	"fmt"
	"testing"
	"time"

	"github.com/jeremyhahn/go-ipam/pkg/ipam"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestPebbleStore(t *testing.T) (*PebbleStore, func()) {
	tempDir := t.TempDir()
	store, err := NewPebbleStore(tempDir)
	require.NoError(t, err)

	cleanup := func() {
		store.Close()
	}

	return store, cleanup
}

func TestPebbleStoreNetworkOperations(t *testing.T) {
	store, cleanup := createTestPebbleStore(t)
	defer cleanup()

	// Test SaveNetwork
	network := &ipam.Network{
		ID:          "net1",
		CIDR:        "192.168.1.0/24",
		Description: "Test network",
		Tags:        []string{"test", "pebble"},
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

	// Test network update
	network.Description = "Updated network"
	err = store.SaveNetwork(network)
	require.NoError(t, err)

	retrieved, err = store.GetNetwork("net1")
	require.NoError(t, err)
	assert.Equal(t, "Updated network", retrieved.Description)

	// Test DeleteNetwork
	err = store.DeleteNetwork("net1")
	require.NoError(t, err)

	_, err = store.GetNetwork("net1")
	assert.ErrorIs(t, err, ipam.ErrNetworkNotFound)
}

func TestPebbleStoreAllocationOperations(t *testing.T) {
	store, cleanup := createTestPebbleStore(t)
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

	// Test allocation update
	allocation.Description = "Updated allocation"
	err = store.SaveAllocation(allocation)
	require.NoError(t, err)

	retrieved, err = store.GetAllocation("alloc1")
	require.NoError(t, err)
	assert.Equal(t, "Updated allocation", retrieved.Description)

	// Test DeleteAllocation
	err = store.DeleteAllocation("alloc1")
	require.NoError(t, err)

	_, err = store.GetAllocation("alloc1")
	assert.ErrorIs(t, err, ipam.ErrIPNotAllocated)
}

func TestPebbleStoreAuditOperations(t *testing.T) {
	store, cleanup := createTestPebbleStore(t)
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

func TestPebbleStoreDeleteNetworkCascade(t *testing.T) {
	store, cleanup := createTestPebbleStore(t)
	defer cleanup()

	// Create network
	network := &ipam.Network{
		ID:        "net1",
		CIDR:      "10.0.0.0/24",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := store.SaveNetwork(network)
	require.NoError(t, err)

	// Create allocations
	for i := 0; i < 5; i++ {
		allocation := &ipam.IPAllocation{
			ID:          fmt.Sprintf("alloc%d", i),
			NetworkID:   "net1",
			IP:          fmt.Sprintf("10.0.0.%d", i+10),
			Status:      "allocated",
			AllocatedAt: time.Now(),
		}
		err := store.SaveAllocation(allocation)
		require.NoError(t, err)
	}

	// Verify allocations exist
	allocations, err := store.ListAllocations("net1")
	require.NoError(t, err)
	assert.Len(t, allocations, 5)

	// Delete network (should cascade delete allocations)
	err = store.DeleteNetwork("net1")
	require.NoError(t, err)

	// Verify allocations are gone
	allocations, err = store.ListAllocations("net1")
	require.NoError(t, err)
	assert.Len(t, allocations, 0)

	// Verify IP indexes are cleaned up
	for i := 0; i < 5; i++ {
		_, err := store.GetAllocationByIP("net1", fmt.Sprintf("10.0.0.%d", i+10))
		assert.ErrorIs(t, err, ipam.ErrIPNotAllocated)
	}
}

func TestPebbleStoreConcurrentOperations(t *testing.T) {
	store, cleanup := createTestPebbleStore(t)
	defer cleanup()

	// Create a network
	network := &ipam.Network{
		ID:        "net1",
		CIDR:      "10.0.0.0/24",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := store.SaveNetwork(network)
	require.NoError(t, err)

	// Run concurrent allocations
	done := make(chan bool, 10)
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			allocation := &ipam.IPAllocation{
				ID:          fmt.Sprintf("alloc%d", idx),
				NetworkID:   "net1",
				IP:          fmt.Sprintf("10.0.0.%d", idx+10),
				Status:      "allocated",
				AllocatedAt: time.Now(),
			}
			if err := store.SaveAllocation(allocation); err != nil {
				errors <- err
			}
			done <- true
		}(i)
	}

	// Wait for all operations
	for i := 0; i < 10; i++ {
		<-done
	}
	close(errors)

	// Check for errors
	for err := range errors {
		assert.NoError(t, err)
	}

	// Verify all allocations were saved
	allocations, err := store.ListAllocations("net1")
	assert.NoError(t, err)
	assert.Len(t, allocations, 10)
}

func TestPebbleStoreStats(t *testing.T) {
	store, cleanup := createTestPebbleStore(t)
	defer cleanup()

	// Add some data
	for i := 0; i < 5; i++ {
		network := &ipam.Network{
			ID:        fmt.Sprintf("net%d", i),
			CIDR:      fmt.Sprintf("10.%d.0.0/24", i),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := store.SaveNetwork(network)
		require.NoError(t, err)
	}

	// Get stats
	stats, err := store.GetStats()
	require.NoError(t, err)
	assert.NotNil(t, stats)

	// Basic sanity checks - stats is a metrics object
	// Just verify we got it without errors
}

func BenchmarkPebbleStoreWrite(b *testing.B) {
	store, cleanup := createTestPebbleStore(&testing.T{})
	defer cleanup()

	network := &ipam.Network{
		ID:        "bench-net",
		CIDR:      "10.0.0.0/24",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.SaveNetwork(network)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		allocation := &ipam.IPAllocation{
			ID:          fmt.Sprintf("alloc%d", i),
			NetworkID:   "bench-net",
			IP:          fmt.Sprintf("10.0.0.%d", (i%250)+1),
			Status:      "allocated",
			AllocatedAt: time.Now(),
		}
		store.SaveAllocation(allocation)
	}
}

func BenchmarkPebbleStoreRead(b *testing.B) {
	store, cleanup := createTestPebbleStore(&testing.T{})
	defer cleanup()

	// Prepare data
	network := &ipam.Network{
		ID:        "bench-net",
		CIDR:      "10.0.0.0/24",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.SaveNetwork(network)

	for i := 0; i < 100; i++ {
		allocation := &ipam.IPAllocation{
			ID:          fmt.Sprintf("alloc%d", i),
			NetworkID:   "bench-net",
			IP:          fmt.Sprintf("10.0.0.%d", i+1),
			Status:      "allocated",
			AllocatedAt: time.Now(),
		}
		store.SaveAllocation(allocation)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.GetAllocationByIP("bench-net", fmt.Sprintf("10.0.0.%d", (i%100)+1))
	}
}
