package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var releaseCmd = &cobra.Command{
	Use:   "release [IP]",
	Short: "Release an allocated IP address",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ip := args[0]
		networkID, _ := cmd.Flags().GetString("network-id")

		if networkID == "" {
			// Try to find the network containing this IP
			networks, err := pebbleStore.ListNetworks()
			if err != nil {
				return fmt.Errorf("failed to list networks: %w", err)
			}

			for _, network := range networks {
				allocations, err := pebbleStore.ListAllocations(network.ID)
				if err != nil {
					continue
				}

				for _, alloc := range allocations {
					if alloc.IP == ip && alloc.ReleasedAt == nil {
						networkID = network.ID
						break
					}
				}

				if networkID != "" {
					break
				}
			}

			if networkID == "" {
				return fmt.Errorf("IP %s not found in any network", ip)
			}
		}

		if err := ipamClient.ReleaseIP(networkID, ip); err != nil {
			return fmt.Errorf("failed to release IP: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "IP %s released successfully.\n", ip)
		return nil
	},
}

func init() {
	releaseCmd.Flags().StringP("network-id", "n", "", "Network ID (optional, will auto-detect)")
}
