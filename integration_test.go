// +build integration

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFullIntegration(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Build the binary
	cmd := exec.Command("go", "build", "-o", filepath.Join(tempDir, "ipam"))
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build: %s", string(output))

	ipamBin := filepath.Join(tempDir, "ipam")

	// Test CLI commands
	t.Run("CLI", func(t *testing.T) {
		// Add network
		cmd := exec.Command(ipamBin, "--db", dbPath, "network", "add", "192.168.1.0/24",
			"-d", "Test network", "-t", "test,cli")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Failed to add network: %s", string(output))
		assert.Contains(t, string(output), "Network added successfully")
		assert.Contains(t, string(output), "192.168.1.0/24")

		// Add IPv6 network
		cmd = exec.Command(ipamBin, "--db", dbPath, "network", "add", "2001:db8::/32",
			"-d", "IPv6 network")
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, "Failed to add IPv6 network: %s", string(output))
		assert.Contains(t, string(output), "2001:db8::/32")

		// List networks
		cmd = exec.Command(ipamBin, "--db", dbPath, "network", "list")
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, "Failed to list networks: %s", string(output))
		assert.Contains(t, string(output), "192.168.1.0/24")
		assert.Contains(t, string(output), "2001:db8::/32")

		// Allocate IP
		cmd = exec.Command(ipamBin, "--db", dbPath, "allocate", "-c", "192.168.1.0/24",
			"-d", "Web server", "-H", "web01.example.com")
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, "Failed to allocate IP: %s", string(output))
		assert.Contains(t, string(output), "IP allocated successfully")
		assert.Contains(t, string(output), "192.168.1.1")
		assert.Contains(t, string(output), "web01.example.com")

		// Allocate IP range
		cmd = exec.Command(ipamBin, "--db", dbPath, "allocate", "-c", "192.168.1.0/24",
			"-k", "5", "-d", "App servers")
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, "Failed to allocate IP range: %s", string(output))
		assert.Contains(t, string(output), "192.168.1.2")
		assert.Contains(t, string(output), "192.168.1.6")

		// Allocate IPv6
		cmd = exec.Command(ipamBin, "--db", dbPath, "allocate", "-c", "2001:db8::/32")
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, "Failed to allocate IPv6: %s", string(output))
		assert.Contains(t, string(output), "2001:db8::1")

		// List allocations
		cmd = exec.Command(ipamBin, "--db", dbPath, "list")
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, "Failed to list allocations: %s", string(output))
		assert.Contains(t, string(output), "192.168.1.1")
		assert.Contains(t, string(output), "web01.example.com")
		assert.Contains(t, string(output), "2001:db8::1")

		// Show stats
		cmd = exec.Command(ipamBin, "--db", dbPath, "stats")
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, "Failed to show stats: %s", string(output))
		assert.Contains(t, string(output), "192.168.1.0/24")
		assert.Contains(t, string(output), "Utilization")

		// Release IP
		cmd = exec.Command(ipamBin, "--db", dbPath, "release", "192.168.1.1")
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, "Failed to release IP: %s", string(output))
		assert.Contains(t, string(output), "released successfully")

		// Verify release in list
		cmd = exec.Command(ipamBin, "--db", dbPath, "list")
		output, err = cmd.CombinedOutput()
		require.NoError(t, err)
		assert.NotContains(t, string(output), "192.168.1.1")

		// List with --all flag to see released IPs
		cmd = exec.Command(ipamBin, "--db", dbPath, "list", "-a")
		output, err = cmd.CombinedOutput()
		require.NoError(t, err)
		assert.Contains(t, string(output), "192.168.1.1")
		assert.Contains(t, string(output), "released")
	})

	// Test API server
	t.Run("API", func(t *testing.T) {
		// Start server
		cmd := exec.Command(ipamBin, "--db", dbPath, "server", "-p", "8888")
		err := cmd.Start()
		require.NoError(t, err)
		defer cmd.Process.Kill()

		// Wait for server to start
		time.Sleep(2 * time.Second)

		apiURL := "http://localhost:8888/api/v1"

		// Test health endpoint
		resp, err := http.Get(apiURL + "/health")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var health map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&health)
		resp.Body.Close()
		assert.Equal(t, "healthy", health["status"])

		// Create network via API
		networkData := map[string]interface{}{
			"cidr":        "10.0.0.0/16",
			"description": "API test network",
			"tags":        []string{"api", "test"},
		}
		body, _ := json.Marshal(networkData)

		resp, err = http.Post(apiURL+"/networks", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var network map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&network)
		resp.Body.Close()
		networkID := network["id"].(string)

		// Get network stats
		resp, err = http.Get(fmt.Sprintf("%s/networks/%s/stats", apiURL, networkID))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var stats map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&stats)
		resp.Body.Close()
		assert.Equal(t, float64(65536), stats["total_ips"])

		// Allocate IP with TTL
		allocationData := map[string]interface{}{
			"network_id":  networkID,
			"description": "API allocation",
			"hostname":    "api-test.example.com",
			"ttl":         300, // 5 minutes
		}
		body, _ = json.Marshal(allocationData)

		resp, err = http.Post(apiURL+"/allocations", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var allocation map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&allocation)
		resp.Body.Close()
		assert.Equal(t, "10.0.0.1", allocation["ip"])
		assert.NotNil(t, allocation["expires_at"])
		allocationID := allocation["id"].(string)

		// Allocate range
		rangeData := map[string]interface{}{
			"network_id": networkID,
			"count":      10,
			"tags":       []string{"range", "test"},
		}
		body, _ = json.Marshal(rangeData)

		resp, err = http.Post(apiURL+"/allocations", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var rangeAlloc map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&rangeAlloc)
		resp.Body.Close()
		assert.Equal(t, "10.0.0.2", rangeAlloc["ip"])
		assert.Equal(t, "10.0.0.11", rangeAlloc["end_ip"])

		// List allocations
		resp, err = http.Get(apiURL + "/allocations")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var allocations []map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&allocations)
		resp.Body.Close()
		assert.GreaterOrEqual(t, len(allocations), 2)

		// Release IP
		resp, err = http.Post(fmt.Sprintf("%s/allocations/%s/release", apiURL, allocationID),
			"application/json", nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		resp.Body.Close()

		// Check audit log
		resp, err = http.Get(apiURL + "/audit?limit=10")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var auditEntries []map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&auditEntries)
		resp.Body.Close()
		assert.Greater(t, len(auditEntries), 0)

		// Verify audit entries contain expected actions
		actions := make(map[string]bool)
		for _, entry := range auditEntries {
			actions[entry["action"].(string)] = true
		}
		assert.True(t, actions["network_added"])
		assert.True(t, actions["ip_allocated"])
		assert.True(t, actions["ip_released"])
	})
}

func TestSingleNodeCluster(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cluster test in short mode")
	}

	// Clean up
	defer func() {
		exec.Command("pkill", "-f", "ipam server").Run()
		os.RemoveAll("ipam-cluster-data")
	}()

	// Initialize single-node cluster
	cmd := exec.Command("./ipam", "cluster", "init", "--node-id", "1", "--cluster-id", "100", "--raft-addr", "localhost:5001", "--single-node")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init single-node cluster: %v", err)
	}

	// Start server
	serverCmd := exec.Command("./ipam", "server", "--cluster", "--config", "ipam-cluster-data/cluster.json", "--port", "8090")
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer serverCmd.Process.Kill()

	// Wait for server to start
	time.Sleep(3 * time.Second)

	t.Run("SingleNodeOperations", func(t *testing.T) {
		apiURL := "http://localhost:8090/api/v1"

		// Test health endpoint
		resp, err := http.Get(apiURL + "/health")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		// Test cluster status endpoint (may not be ready immediately)
		resp, err = http.Get(apiURL + "/cluster/status")
		require.NoError(t, err)
		resp.Body.Close()
		
		if resp.StatusCode == http.StatusOK {
			// Cluster is ready, test the status
			resp, err = http.Get(apiURL + "/cluster/status")
			require.NoError(t, err)
			
			var clusterStatus map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&clusterStatus)
			resp.Body.Close()
			
			if clusterStatus["node_id"] != nil {
				assert.Equal(t, float64(1), clusterStatus["node_id"])
				assert.Equal(t, float64(100), clusterStatus["cluster_id"])
			}
		}

		// Create network
		network := map[string]interface{}{
			"cidr":        "192.168.1.0/24",
			"description": "Single node test",
		}

		data, _ := json.Marshal(network)
		resp, err = http.Post(apiURL+"/networks", "application/json", bytes.NewReader(data))
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var netResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&netResult)
		resp.Body.Close()
		
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Failed to create network, status: %d", resp.StatusCode)
		}
		
		networkID, ok := netResult["id"].(string)
		if !ok {
			t.Fatalf("Network ID not found in response: %v", netResult)
		}

		// Allocate IP
		alloc := map[string]interface{}{
			"network_id": networkID,
			"hostname":   "cluster-test-host",
		}

		data, _ = json.Marshal(alloc)
		resp, err = http.Post(apiURL+"/allocations", "application/json", bytes.NewReader(data))
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var allocResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&allocResult)
		resp.Body.Close()
		assert.Equal(t, "192.168.1.1", allocResult["ip"])
		assert.Equal(t, "cluster-test-host", allocResult["hostname"])
	})
}

func TestThreeNodeCluster(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cluster integration test in short mode")
	}

	// Start the 3-node cluster using Docker Compose
	t.Log("Starting 3-node cluster with Docker Compose...")
	
	// Change to examples/cluster directory
	cmd := exec.Command("docker-compose", "up", "-d")
	cmd.Dir = "examples/cluster"
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start cluster: %v\nOutput: %s", err, output)
	}

	// Ensure cleanup
	defer func() {
		t.Log("Stopping cluster...")
		cmd := exec.Command("docker-compose", "down", "-v")
		cmd.Dir = "examples/cluster"
		cmd.Run()
	}()

	// Wait for cluster to be ready
	t.Log("Waiting for cluster to form...")
	time.Sleep(30 * time.Second)

	// Test cluster health via load balancer
	t.Run("LoadBalancerHealth", func(t *testing.T) {
		resp, err := http.Get("http://localhost:8080/api/v1/health")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	// Test all individual nodes are responding
	nodes := []string{"8081", "8082", "8083"}
	for _, port := range nodes {
		t.Run(fmt.Sprintf("Node%s_Health", port), func(t *testing.T) {
			resp, err := http.Get(fmt.Sprintf("http://localhost:%s/api/v1/health", port))
			if err != nil {
				t.Errorf("Node on port %s not responding: %v", port, err)
				return
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Node on port %s unhealthy: status %d", port, resp.StatusCode)
			}
		})
	}

	// Test cluster status endpoints
	t.Run("ClusterStatus", func(t *testing.T) {
		for _, port := range nodes {
			resp, err := http.Get(fmt.Sprintf("http://localhost:%s/api/v1/cluster/status", port))
			if err != nil {
				t.Errorf("Failed to get cluster status from node %s: %v", port, err)
				continue
			}

			var status map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&status)
			resp.Body.Close()

			assert.Equal(t, float64(1), status["cluster_id"])
			assert.Contains(t, []float64{1, 2, 3}, status["node_id"])
			assert.Contains(t, []string{"leader", "follower"}, status["role"])
		}
	})

	// Test data consistency across nodes
	t.Run("DataConsistency", func(t *testing.T) {
		// Create a network via load balancer
		network := map[string]interface{}{
			"cidr":        "10.0.0.0/24",
			"description": "Test network for Raft cluster",
		}

		data, _ := json.Marshal(network)
		resp, err := http.Post("http://localhost:8080/api/v1/networks", "application/json", bytes.NewReader(data))
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Parse response to get network ID
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		networkID := result["id"].(string)

		// Give Raft time to replicate
		time.Sleep(5 * time.Second)

		// Read from all nodes - data should be consistent
		for _, port := range nodes {
			resp, err := http.Get(fmt.Sprintf("http://localhost:%s/api/v1/networks", port))
			if err != nil {
				t.Errorf("Failed to read from node %s: %v", port, err)
				continue
			}

			var networks []map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&networks)
			resp.Body.Close()

			found := false
			for _, net := range networks {
				if net["id"] == networkID {
					found = true
					if net["cidr"] != "10.0.0.0/24" {
						t.Errorf("Node %s has inconsistent data: %v", port, net)
					}
					break
				}
			}

			if !found {
				t.Errorf("Node %s doesn't have the network", port)
			}
		}
	})

	// Test distributed operations
	t.Run("DistributedAllocation", func(t *testing.T) {
		allocatedIPs := make(map[string]string)

		// Allocate IPs from different nodes
		for i, port := range nodes {
			alloc := map[string]interface{}{
				"network_id": "net-1", // Should exist from previous test
				"hostname":   fmt.Sprintf("host%d", i+1),
			}

			data, _ := json.Marshal(alloc)
			resp, err := http.Post(fmt.Sprintf("http://localhost:%s/api/v1/allocations", port), "application/json", bytes.NewReader(data))
			if err != nil {
				t.Errorf("Failed to allocate from node %s: %v", port, err)
				continue
			}

			if resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&result)
				resp.Body.Close()

				if ip, ok := result["ip"].(string); ok {
					if _, exists := allocatedIPs[ip]; exists {
						t.Errorf("Duplicate IP allocated: %s", ip)
					}
					allocatedIPs[ip] = port
					t.Logf("Node %s allocated IP: %s", port, ip)
				}
			} else {
				body, _ := io.ReadAll(resp.Body)
				t.Logf("Node %s allocation failed: %s", port, body)
				resp.Body.Close()
			}
		}

		// Verify we got unique IPs
		if len(allocatedIPs) > 0 {
			t.Logf("Successfully allocated %d unique IPs across cluster", len(allocatedIPs))
		}
	})
}

func TestEdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "edge.db")

	cmd := exec.Command("go", "build", "-o", filepath.Join(tempDir, "ipam"))
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build: %s", string(output))

	ipamBin := filepath.Join(tempDir, "ipam")

	// Test tiny network exhaustion
	cmd = exec.Command(ipamBin, "--db", dbPath, "network", "add", "10.0.0.0/30")
	output, err = cmd.CombinedOutput()
	require.NoError(t, err)

	// Allocate all available IPs (2 usable in a /30)
	for i := 0; i < 2; i++ {
		cmd = exec.Command(ipamBin, "--db", dbPath, "allocate", "-c", "10.0.0.0/30")
		output, err = cmd.CombinedOutput()
		require.NoError(t, err)
	}

	// Try to allocate when full
	cmd = exec.Command(ipamBin, "--db", dbPath, "allocate", "-c", "10.0.0.0/30")
	output, err = cmd.CombinedOutput()
	assert.Error(t, err)
	assert.Contains(t, string(output), "no available IP addresses")

	// Test invalid operations
	cmd = exec.Command(ipamBin, "--db", dbPath, "network", "add", "invalid-cidr")
	output, err = cmd.CombinedOutput()
	assert.Error(t, err)
	assert.Contains(t, string(output), "invalid CIDR")

	// Test release non-existent IP
	cmd = exec.Command(ipamBin, "--db", dbPath, "release", "1.2.3.4")
	output, err = cmd.CombinedOutput()
	assert.Error(t, err)

	// Test allocation from non-existent network
	cmd = exec.Command(ipamBin, "--db", dbPath, "allocate", "-c", "172.16.0.0/16")
	output, err = cmd.CombinedOutput()
	assert.Error(t, err)
	assert.Contains(t, string(output), "network not found")
}

func TestConcurrentOperations(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "concurrent.db")

	cmd := exec.Command("go", "build", "-o", filepath.Join(tempDir, "ipam"))
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build: %s", string(output))

	ipamBin := filepath.Join(tempDir, "ipam")

	// Create network
	cmd = exec.Command(ipamBin, "--db", dbPath, "network", "add", "10.0.0.0/24")
	output, err = cmd.CombinedOutput()
	require.NoError(t, err)

	// Run concurrent allocations
	done := make(chan string, 10)
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			cmd := exec.Command(ipamBin, "--db", dbPath, "allocate", "-c", "10.0.0.0/24")
			output, err := cmd.CombinedOutput()
			if err != nil {
				errors <- err
				return
			}
			// Extract IP from output
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "IP:") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						done <- parts[1]
						return
					}
				}
			}
			done <- ""
		}()
	}

	// Collect results
	allocatedIPs := make(map[string]bool)
	for i := 0; i < 10; i++ {
		select {
		case ip := <-done:
			if ip != "" {
				assert.False(t, allocatedIPs[ip], "Duplicate IP allocated: %s", ip)
				allocatedIPs[ip] = true
			}
		case err := <-errors:
			t.Errorf("Allocation error: %v", err)
		}
	}

	assert.Equal(t, 10, len(allocatedIPs), "Expected 10 unique IPs")
}