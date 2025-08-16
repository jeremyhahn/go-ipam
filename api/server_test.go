package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jeremyhahn/go-ipam/pkg/ipam"
	"github.com/jeremyhahn/go-ipam/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestServer(t *testing.T) (*Server, func()) {
	// Create test store
	tempDir := t.TempDir()
	dbPath := fmt.Sprintf("%s/test.db", tempDir)

	pebbleStore, err := store.NewPebbleStore(dbPath)
	require.NoError(t, err)

	ipamClient := ipam.New(pebbleStore)
	server := NewServer(ipamClient, pebbleStore)

	cleanup := func() {
		pebbleStore.Close()
	}

	return server, cleanup
}

func TestHealthEndpoint(t *testing.T) {
	server, cleanup := createTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
	assert.Equal(t, "ipam", response["service"])
	assert.Equal(t, false, response["cluster_mode"])
}

func TestNetworkEndpoints(t *testing.T) {
	server, cleanup := createTestServer(t)
	defer cleanup()

	// Test create network
	networkData := map[string]interface{}{
		"cidr":        "192.168.1.0/24",
		"description": "Test network",
		"tags":        []string{"test", "api"},
	}
	body, _ := json.Marshal(networkData)

	req := httptest.NewRequest("POST", "/api/v1/networks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var network ipam.Network
	err := json.NewDecoder(w.Body).Decode(&network)
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.0/24", network.CIDR)
	assert.Equal(t, "Test network", network.Description)
	assert.Equal(t, []string{"test", "api"}, network.Tags)

	// Test list networks
	req = httptest.NewRequest("GET", "/api/v1/networks", nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var networks []*ipam.Network
	err = json.NewDecoder(w.Body).Decode(&networks)
	require.NoError(t, err)
	assert.Len(t, networks, 1)

	// Test get specific network
	req = httptest.NewRequest("GET", fmt.Sprintf("/api/v1/networks/%s", network.ID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var retrieved ipam.Network
	err = json.NewDecoder(w.Body).Decode(&retrieved)
	require.NoError(t, err)
	assert.Equal(t, network.ID, retrieved.ID)

	// Test get network stats
	req = httptest.NewRequest("GET", fmt.Sprintf("/api/v1/networks/%s/stats", network.ID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var stats ipam.NetworkStats
	err = json.NewDecoder(w.Body).Decode(&stats)
	require.NoError(t, err)
	assert.Equal(t, uint64(256), stats.TotalIPs)
	assert.Equal(t, uint64(0), stats.AllocatedIPs)

	// Test delete network
	req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/networks/%s", network.ID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestAllocationEndpoints(t *testing.T) {
	server, cleanup := createTestServer(t)
	defer cleanup()

	// Create a network first
	networkData := map[string]interface{}{
		"cidr": "10.0.0.0/24",
	}
	body, _ := json.Marshal(networkData)

	req := httptest.NewRequest("POST", "/api/v1/networks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var network ipam.Network
	json.NewDecoder(w.Body).Decode(&network)

	// Test allocate IP
	allocationData := map[string]interface{}{
		"network_id":  network.ID,
		"count":       1,
		"description": "Test allocation",
		"hostname":    "test-host",
		"tags":        []string{"test"},
	}
	body, _ = json.Marshal(allocationData)

	req = httptest.NewRequest("POST", "/api/v1/allocations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var allocation ipam.IPAllocation
	err := json.NewDecoder(w.Body).Decode(&allocation)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", allocation.IP)
	assert.Equal(t, "Test allocation", allocation.Description)
	assert.Equal(t, "test-host", allocation.Hostname)

	// Test list allocations
	req = httptest.NewRequest("GET", "/api/v1/allocations", nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var allocations []*ipam.IPAllocation
	err = json.NewDecoder(w.Body).Decode(&allocations)
	require.NoError(t, err)
	assert.Len(t, allocations, 1)

	// Test list allocations for specific network
	req = httptest.NewRequest("GET", fmt.Sprintf("/api/v1/allocations?network_id=%s", network.ID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	err = json.NewDecoder(w.Body).Decode(&allocations)
	require.NoError(t, err)
	assert.Len(t, allocations, 1)

	// Test get specific allocation
	req = httptest.NewRequest("GET", fmt.Sprintf("/api/v1/allocations/%s", allocation.ID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var retrieved ipam.IPAllocation
	err = json.NewDecoder(w.Body).Decode(&retrieved)
	require.NoError(t, err)
	assert.Equal(t, allocation.ID, retrieved.ID)

	// Test release IP
	req = httptest.NewRequest("POST", fmt.Sprintf("/api/v1/allocations/%s/release", allocation.ID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify IP was released
	req = httptest.NewRequest("GET", fmt.Sprintf("/api/v1/allocations/%s", allocation.ID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var released ipam.IPAllocation
	json.NewDecoder(w.Body).Decode(&released)
	assert.NotNil(t, released.ReleasedAt)
}

func TestAllocationWithTTL(t *testing.T) {
	server, cleanup := createTestServer(t)
	defer cleanup()

	// Create network
	networkData := map[string]interface{}{
		"cidr": "172.16.0.0/24",
	}
	body, _ := json.Marshal(networkData)

	req := httptest.NewRequest("POST", "/api/v1/networks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var network ipam.Network
	json.NewDecoder(w.Body).Decode(&network)

	// Allocate with TTL
	allocationData := map[string]interface{}{
		"network_id": network.ID,
		"ttl":        3600, // 1 hour
	}
	body, _ = json.Marshal(allocationData)

	req = httptest.NewRequest("POST", "/api/v1/allocations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var allocation ipam.IPAllocation
	err := json.NewDecoder(w.Body).Decode(&allocation)
	require.NoError(t, err)
	assert.NotNil(t, allocation.ExpiresAt)
	assert.True(t, allocation.ExpiresAt.After(time.Now()))
}

func TestRangeAllocation(t *testing.T) {
	server, cleanup := createTestServer(t)
	defer cleanup()

	// Create network
	networkData := map[string]interface{}{
		"cidr": "10.1.0.0/24",
	}
	body, _ := json.Marshal(networkData)

	req := httptest.NewRequest("POST", "/api/v1/networks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var network ipam.Network
	json.NewDecoder(w.Body).Decode(&network)

	// Allocate range
	allocationData := map[string]interface{}{
		"network_id": network.ID,
		"count":      5,
	}
	body, _ = json.Marshal(allocationData)

	req = httptest.NewRequest("POST", "/api/v1/allocations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var allocation ipam.IPAllocation
	err := json.NewDecoder(w.Body).Decode(&allocation)
	require.NoError(t, err)
	assert.Equal(t, "10.1.0.1", allocation.IP)
	assert.Equal(t, "10.1.0.5", allocation.EndIP)
}

func TestAuditEndpoint(t *testing.T) {
	server, cleanup := createTestServer(t)
	defer cleanup()

	// Perform some operations to generate audit entries
	networkData := map[string]interface{}{
		"cidr": "192.168.0.0/24",
	}
	body, _ := json.Marshal(networkData)

	req := httptest.NewRequest("POST", "/api/v1/networks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var network ipam.Network
	json.NewDecoder(w.Body).Decode(&network)

	// Allocate IP
	allocationData := map[string]interface{}{
		"network_id": network.ID,
	}
	body, _ = json.Marshal(allocationData)

	req = httptest.NewRequest("POST", "/api/v1/allocations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	// Get audit log
	req = httptest.NewRequest("GET", "/api/v1/audit", nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var entries []*ipam.AuditEntry
	err := json.NewDecoder(w.Body).Decode(&entries)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(entries), 2)

	// Test with limit
	req = httptest.NewRequest("GET", "/api/v1/audit?limit=1", nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	err = json.NewDecoder(w.Body).Decode(&entries)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestErrorHandling(t *testing.T) {
	server, cleanup := createTestServer(t)
	defer cleanup()

	// Test invalid CIDR
	networkData := map[string]interface{}{
		"cidr": "invalid-cidr",
	}
	body, _ := json.Marshal(networkData)

	req := httptest.NewRequest("POST", "/api/v1/networks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Test allocation from non-existent network
	allocationData := map[string]interface{}{
		"network_id": "non-existent",
	}
	body, _ = json.Marshal(allocationData)

	req = httptest.NewRequest("POST", "/api/v1/allocations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Test delete network with active allocations
	// First create network and allocation
	networkData = map[string]interface{}{
		"cidr": "10.2.0.0/24",
	}
	body, _ = json.Marshal(networkData)

	req = httptest.NewRequest("POST", "/api/v1/networks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var network ipam.Network
	json.NewDecoder(w.Body).Decode(&network)

	allocationData = map[string]interface{}{
		"network_id": network.ID,
	}
	body, _ = json.Marshal(allocationData)

	req = httptest.NewRequest("POST", "/api/v1/allocations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	// Try to delete network with active allocation
	req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/networks/%s", network.ID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestConcurrentRequests(t *testing.T) {
	server, cleanup := createTestServer(t)
	defer cleanup()

	// Create network
	networkData := map[string]interface{}{
		"cidr": "10.3.0.0/24",
	}
	body, _ := json.Marshal(networkData)

	req := httptest.NewRequest("POST", "/api/v1/networks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var network ipam.Network
	json.NewDecoder(w.Body).Decode(&network)

	// Simulate concurrent allocations
	done := make(chan bool, 10)
	allocatedIPs := make(map[string]bool)
	ipChan := make(chan string, 10)

	for i := 0; i < 10; i++ {
		go func() {
			allocationData := map[string]interface{}{
				"network_id": network.ID,
			}
			body, _ := json.Marshal(allocationData)

			req := httptest.NewRequest("POST", "/api/v1/allocations", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)

			if w.Code == http.StatusCreated {
				var allocation ipam.IPAllocation
				json.NewDecoder(w.Body).Decode(&allocation)
				ipChan <- allocation.IP
			}
			done <- true
		}()
	}

	// Wait for all requests
	for i := 0; i < 10; i++ {
		<-done
	}
	close(ipChan)

	// Verify no duplicate IPs
	for ip := range ipChan {
		assert.False(t, allocatedIPs[ip], "Duplicate IP allocated: %s", ip)
		allocatedIPs[ip] = true
	}
}
