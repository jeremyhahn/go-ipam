package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jeremyhahn/go-ipam/api"
	"github.com/jeremyhahn/go-ipam/pkg/config"
	"github.com/jeremyhahn/go-ipam/pkg/ipam"
	"github.com/jeremyhahn/go-ipam/pkg/store"
	"github.com/spf13/cobra"
)

var (
	configFile string
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the IPAM API server",
	Long:  `Start the REST API server for the IPAM system.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		host, _ := cmd.Flags().GetString("host")
		address, _ := cmd.Flags().GetString("address")

		// Parse address if provided
		if address != "" {
			var err error
			host, port, err = parseAddress(address)
			if err != nil {
				return fmt.Errorf("invalid address: %w", err)
			}
		}

		// Check if running in cluster mode
		if clusterMode {
			return runClusterServer(host, port)
		}

		// Standard mode - use PebbleDB
		return runStandardServer(host, port)
	},
}

func runStandardServer(host string, port int) error {
	// Initialize API server with PebbleDB store
	server := api.NewServer(ipamClient, pebbleStore)

	addr := fmt.Sprintf("%s:%d", host, port)
	fmt.Printf("Starting IPAM server (standalone mode) on %s\n", addr)
	fmt.Printf("API available at: http://%s/api/v1\n", addr)

	log.Fatal(http.ListenAndServe(addr, server))
	return nil
}

func runClusterServer(host string, port int) error {
	// Load cluster configuration
	if configFile == "" {
		// Try default location
		configFile = filepath.Join("ipam-cluster-data", "cluster.json")
	}

	configData, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read cluster config: %w", err)
	}

	var clusterConfig config.ClusterConfig
	if err := json.Unmarshal(configData, &clusterConfig); err != nil {
		return fmt.Errorf("failed to parse cluster config: %w", err)
	}

	// Override API address if specified
	if host != "" && port != 0 {
		clusterConfig.APIAddr = fmt.Sprintf("%s:%d", host, port)
	} else if clusterConfig.APIAddr == "" {
		clusterConfig.APIAddr = fmt.Sprintf("0.0.0.0:%d", port)
	}

	// Validate configuration
	if err := clusterConfig.Validate(); err != nil {
		return fmt.Errorf("invalid cluster configuration: %w", err)
	}

	// Initialize Raft store
	raftStore, err := store.NewRaftStore(
		clusterConfig.NodeID,
		clusterConfig.ClusterID,
		clusterConfig.RaftAddr,
		clusterConfig.Join,
		clusterConfig.InitialMembers,
		clusterConfig.DataDir,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize Raft store: %w", err)
	}
	defer raftStore.Close()

	// Create IPAM client with Raft store
	ipamClient := ipam.New(raftStore)

	// Initialize API server with Raft store
	server := api.NewServer(ipamClient, raftStore)

	// Use the provided address or fall back to configured one
	addr := fmt.Sprintf("%s:%d", host, port)
	fmt.Printf("Starting IPAM server (cluster mode) on %s\n", addr)
	fmt.Printf("  Node ID:     %d\n", clusterConfig.NodeID)
	fmt.Printf("  Cluster ID:  %d\n", clusterConfig.ClusterID)
	fmt.Printf("  Raft Addr:   %s\n", clusterConfig.RaftAddr)
	fmt.Printf("  API Addr:    %s\n", addr)
	fmt.Printf("API available at: http://%s/api/v1\n", addr)

	log.Fatal(http.ListenAndServe(addr, server))
	return nil
}

func parseAddress(address string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port: %w", err)
	}

	return host, port, nil
}

func init() {
	serverCmd.Flags().IntP("port", "p", 8080, "Server port")
	serverCmd.Flags().StringP("host", "H", "0.0.0.0", "Server host")
	serverCmd.Flags().StringP("address", "a", "", "Server address (host:port)")
	serverCmd.Flags().StringVar(&configFile, "config", "", "Path to cluster configuration file")
}
