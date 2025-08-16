package cmd

import (
	"fmt"
	"strings"

	"github.com/jeremyhahn/go-ipam/pkg/ipam"
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show network statistics",
	Long:  `Display utilization statistics for networks.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		networkID, _ := cmd.Flags().GetString("network-id")

		var networks []*ipam.Network

		if networkID != "" {
			network, err := pebbleStore.GetNetwork(networkID)
			if err != nil {
				return fmt.Errorf("failed to get network: %w", err)
			}
			networks = append(networks, network)
		} else {
			var err error
			networks, err = pebbleStore.ListNetworks()
			if err != nil {
				return fmt.Errorf("failed to list networks: %w", err)
			}
		}

		if len(networks) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No networks found.")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-15s %-15s %-15s %-15s %s\n",
			"Network", "Total IPs", "Allocated", "Available", "Reserved", "Utilization")
		fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 95))

		for _, network := range networks {
			stats, err := ipamClient.GetNetworkStats(network.ID)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s Error: %v\n", network.CIDR, err)
				continue
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-15d %-15d %-15d %-15d %.1f%%\n",
				network.CIDR,
				stats.TotalIPs,
				stats.AllocatedIPs,
				stats.AvailableIPs,
				stats.ReservedIPs,
				stats.UtilizationPercent,
			)
		}

		return nil
	},
}

func init() {
	statsCmd.Flags().StringP("network-id", "n", "", "Show stats for specific network")
}
