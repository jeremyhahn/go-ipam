package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jeremyhahn/go-ipam/pkg/config"
	"github.com/jeremyhahn/go-ipam/pkg/store"
	"github.com/spf13/cobra"
)

var (
	clusterMode      bool
	nodeID           uint64
	clusterID        uint64
	raftAddr         string
	dataDir          string
	joinCluster      bool
	initialMembers   string
	enableSingleNode bool
)

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Cluster management commands",
	Long:  `Commands for managing IPAM cluster nodes and configuration.`,
}

var clusterInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new cluster",
	Long: `Initialize a new IPAM cluster. This creates the initial configuration
and starts the first node. Other nodes can then join this cluster.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse initial members
		members, err := config.ParseInitialMembers(initialMembers)
		if err != nil {
			return fmt.Errorf("failed to parse initial members: %w", err)
		}

		// For single-node clusters, ensure this node is in the members list
		if enableSingleNode && len(members) == 0 {
			members = map[uint64]string{nodeID: raftAddr}
		}

		// Create cluster config
		cfg := &config.ClusterConfig{
			NodeID:           nodeID,
			ClusterID:        clusterID,
			RaftAddr:         raftAddr,
			APIAddr:          "",
			DataDir:          dataDir,
			Join:             false,
			InitialMembers:   members,
			EnableSingleNode: enableSingleNode,
		}

		// Validate configuration
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("invalid configuration: %w", err)
		}

		// Create data directory
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}

		// Save configuration
		configPath := filepath.Join(dataDir, "cluster.json")
		configData, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal configuration: %w", err)
		}

		if err := os.WriteFile(configPath, configData, 0644); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Cluster initialized successfully:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Cluster ID:  %d\n", cfg.ClusterID)
		fmt.Fprintf(cmd.OutOrStdout(), "  Node ID:     %d\n", cfg.NodeID)
		fmt.Fprintf(cmd.OutOrStdout(), "  Raft Addr:   %s\n", cfg.RaftAddr)
		fmt.Fprintf(cmd.OutOrStdout(), "  Data Dir:    %s\n", cfg.DataDir)
		fmt.Fprintf(cmd.OutOrStdout(), "  Config File: %s\n", configPath)
		if len(cfg.InitialMembers) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Initial Members:\n")
			for nid, addr := range cfg.InitialMembers {
				fmt.Fprintf(cmd.OutOrStdout(), "    Node %d: %s\n", nid, addr)
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\nTo start this node, run:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  ipam server --cluster --config %s\n", configPath)

		return nil
	},
}

var clusterJoinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join an existing cluster",
	Long:  `Join this node to an existing IPAM cluster.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse initial members
		members, err := config.ParseInitialMembers(initialMembers)
		if err != nil {
			return fmt.Errorf("failed to parse initial members: %w", err)
		}

		if len(members) == 0 {
			return fmt.Errorf("initial members are required when joining a cluster")
		}

		// Create cluster config
		cfg := &config.ClusterConfig{
			NodeID:         nodeID,
			ClusterID:      clusterID,
			RaftAddr:       raftAddr,
			APIAddr:        "",
			DataDir:        dataDir,
			Join:           true,
			InitialMembers: members,
		}

		// Validate configuration
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("invalid configuration: %w", err)
		}

		// Create data directory
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}

		// Save configuration
		configPath := filepath.Join(dataDir, "cluster.json")
		configData, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal configuration: %w", err)
		}

		if err := os.WriteFile(configPath, configData, 0644); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Node configured to join cluster:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Cluster ID:  %d\n", cfg.ClusterID)
		fmt.Fprintf(cmd.OutOrStdout(), "  Node ID:     %d\n", cfg.NodeID)
		fmt.Fprintf(cmd.OutOrStdout(), "  Raft Addr:   %s\n", cfg.RaftAddr)
		fmt.Fprintf(cmd.OutOrStdout(), "  Data Dir:    %s\n", cfg.DataDir)
		fmt.Fprintf(cmd.OutOrStdout(), "  Config File: %s\n", configPath)
		fmt.Fprintf(cmd.OutOrStdout(), "\nTo start this node and join the cluster, run:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  ipam server --cluster --config %s\n", configPath)

		return nil
	},
}

var clusterStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cluster status",
	Long:  `Display the current status of the IPAM cluster.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load cluster configuration
		configPath := filepath.Join("ipam-cluster-data", "cluster.json")
		if configFile != "" {
			configPath = configFile
		}

		configData, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read cluster config: %w (try specifying --config)", err)
		}

		var clusterConfig config.ClusterConfig
		if err := json.Unmarshal(configData, &clusterConfig); err != nil {
			return fmt.Errorf("failed to parse cluster config: %w", err)
		}

		// Initialize Raft store temporarily to get status
		raftStore, err := store.NewRaftStore(
			clusterConfig.NodeID,
			clusterConfig.ClusterID,
			clusterConfig.RaftAddr,
			clusterConfig.Join,
			clusterConfig.InitialMembers,
			clusterConfig.DataDir,
		)
		if err != nil {
			return fmt.Errorf("failed to connect to cluster: %w", err)
		}
		defer raftStore.Close()

		info, err := raftStore.GetClusterInfo()
		if err != nil {
			return fmt.Errorf("failed to get cluster info: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Cluster Status:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Cluster ID:        %d\n", info.ClusterID)
		fmt.Fprintf(cmd.OutOrStdout(), "  Leader Node:       %d\n", info.LeaderID)
		fmt.Fprintf(cmd.OutOrStdout(), "  Has Leader:        %v\n", info.HasLeader)
		fmt.Fprintf(cmd.OutOrStdout(), "  Config Change ID:  %d\n", info.ConfigChangeID)
		fmt.Fprintf(cmd.OutOrStdout(), "\nCluster Nodes:\n")

		for _, node := range info.Nodes {
			leaderMark := ""
			if node.IsLeader {
				leaderMark = " (LEADER)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  Node %d: %s%s\n", node.NodeID, node.RaftAddr, leaderMark)
		}

		return nil
	},
}

var clusterAddNodeCmd = &cobra.Command{
	Use:   "add-node [nodeID] [address]",
	Short: "Add a node to the cluster",
	Long:  `Add a node to the cluster. This command must be run through the API on a running cluster node.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("cluster node management must be done through the API on a running node:\n" +
			"  curl -X POST http://<node-address>/api/v1/cluster/nodes \\\n" +
			"    -H 'Content-Type: application/json' \\\n" +
			"    -d '{\"node_id\": <id>, \"addr\": \"<raft-address>\"}'")
	},
}

var clusterRemoveNodeCmd = &cobra.Command{
	Use:   "remove-node [nodeID]",
	Short: "Remove a node from the cluster",
	Long:  `Remove a node from the cluster. This command must be run through the API on a running cluster node.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("cluster node management must be done through the API on a running node:\n" +
			"  curl -X DELETE http://<node-address>/api/v1/cluster/nodes/<node-id>")
	},
}

func init() {
	// Add cluster subcommands
	clusterCmd.AddCommand(clusterInitCmd)
	clusterCmd.AddCommand(clusterJoinCmd)
	clusterCmd.AddCommand(clusterStatusCmd)
	clusterCmd.AddCommand(clusterAddNodeCmd)
	clusterCmd.AddCommand(clusterRemoveNodeCmd)

	// Cluster init flags
	clusterInitCmd.Flags().Uint64Var(&nodeID, "node-id", 1, "Unique node ID (must be > 0)")
	clusterInitCmd.Flags().Uint64Var(&clusterID, "cluster-id", 1, "Cluster ID")
	clusterInitCmd.Flags().StringVar(&raftAddr, "raft-addr", "localhost:5000", "Raft communication address")
	clusterInitCmd.Flags().StringVar(&dataDir, "data-dir", "ipam-cluster-data", "Directory for cluster data")
	clusterInitCmd.Flags().StringVar(&initialMembers, "initial-members", "", "Initial cluster members (e.g., '1:host1:5000,2:host2:5000')")
	clusterInitCmd.Flags().BoolVar(&enableSingleNode, "single-node", false, "Enable single-node cluster mode")

	// Cluster join flags
	clusterJoinCmd.Flags().Uint64Var(&nodeID, "node-id", 0, "Unique node ID (must be > 0)")
	clusterJoinCmd.Flags().Uint64Var(&clusterID, "cluster-id", 1, "Cluster ID to join")
	clusterJoinCmd.Flags().StringVar(&raftAddr, "raft-addr", "", "Raft communication address for this node")
	clusterJoinCmd.Flags().StringVar(&dataDir, "data-dir", "ipam-cluster-data", "Directory for cluster data")
	clusterJoinCmd.Flags().StringVar(&initialMembers, "initial-members", "", "Existing cluster members (e.g., '1:host1:5000,2:host2:5000')")

	clusterJoinCmd.MarkFlagRequired("node-id")
	clusterJoinCmd.MarkFlagRequired("raft-addr")
	clusterJoinCmd.MarkFlagRequired("initial-members")

	// Add persistent flag for cluster mode
	rootCmd.PersistentFlags().BoolVar(&clusterMode, "cluster", false, "Enable cluster mode")

	// Add config flag to status command
	clusterStatusCmd.Flags().StringVar(&configFile, "config", "", "Path to cluster configuration file")
}

func parseNodeID(s string) (uint64, error) {
	id, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid node ID: %s", s)
	}
	if id == 0 {
		return 0, fmt.Errorf("node ID must be greater than 0")
	}
	return id, nil
}
