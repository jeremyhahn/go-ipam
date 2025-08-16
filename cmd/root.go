package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/jeremyhahn/go-ipam/pkg/ipam"
	"github.com/jeremyhahn/go-ipam/pkg/store"
	"github.com/spf13/cobra"
)

var (
	dbPath      string
	ipamClient  *ipam.IPAM
	pebbleStore *store.PebbleStore
	ipamStore   ipam.Store // Generic store interface for cluster mode
)

var rootCmd = &cobra.Command{
	Use:   "ipam",
	Short: "IP Address Management CLI",
	Long:  `A CLI tool for managing IP address allocations across IPv4 and IPv6 networks.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip initialization for cluster commands and server in cluster mode
		if cmd.Name() == "cluster" || (cmd.Name() == "server" && clusterMode) {
			return nil
		}

		// Only create a new store if we don't have one
		if pebbleStore == nil {
			var err error
			pebbleStore, err = store.NewPebbleStore(dbPath)
			if err != nil {
				return fmt.Errorf("failed to initialize store: %w", err)
			}
			ipamStore = pebbleStore
			ipamClient = ipam.New(ipamStore)
		}
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		// Don't close during tests - the test cleanup will handle it
		if pebbleStore != nil && !isTestMode() {
			pebbleStore.Close()
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

// isTestMode returns true if we're running under go test
func isTestMode() bool {
	for _, arg := range os.Args {
		if strings.HasSuffix(arg, ".test") || strings.Contains(arg, "go-build") {
			return true
		}
	}
	return false
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "ipam-data", "Path to database directory")

	// Add subcommands
	rootCmd.AddCommand(networkCmd)
	rootCmd.AddCommand(allocateCmd)
	rootCmd.AddCommand(releaseCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(clusterCmd)
}
