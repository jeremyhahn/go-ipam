package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/jeremyhahn/go-ipam/pkg/ipam"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List allocations",
	Long:  `List all IP allocations, optionally filtered by network.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		networkID, _ := cmd.Flags().GetString("network-id")
		showAll, _ := cmd.Flags().GetBool("all")

		var allAllocations []*struct {
			allocation *ipam.IPAllocation
			network    *ipam.Network
		}

		if networkID != "" {
			network, err := pebbleStore.GetNetwork(networkID)
			if err != nil {
				return fmt.Errorf("failed to get network: %w", err)
			}

			allocations, err := pebbleStore.ListAllocations(networkID)
			if err != nil {
				return fmt.Errorf("failed to list allocations: %w", err)
			}

			for _, alloc := range allocations {
				if !showAll && alloc.ReleasedAt != nil {
					continue
				}
				allAllocations = append(allAllocations, &struct {
					allocation *ipam.IPAllocation
					network    *ipam.Network
				}{alloc, network})
			}
		} else {
			// List all allocations from all networks
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
					if !showAll && alloc.ReleasedAt != nil {
						continue
					}
					allAllocations = append(allAllocations, &struct {
						allocation *ipam.IPAllocation
						network    *ipam.Network
					}{alloc, network})
				}
			}
		}

		if len(allAllocations) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No allocations found.")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %-10s %-20s %-20s %s\n",
			"IP", "Network", "Status", "Hostname", "Description", "Allocated")
		fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 110))

		for _, item := range allAllocations {
			alloc := item.allocation
			network := item.network

			ipStr := alloc.IP
			if alloc.EndIP != "" {
				ipStr = fmt.Sprintf("%s-%s", alloc.IP, alloc.EndIP)
			}

			status := alloc.Status
			if alloc.ReleasedAt != nil {
				status = "released"
			} else if alloc.ExpiresAt != nil && alloc.ExpiresAt.Before(time.Now()) {
				status = "expired"
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %-10s %-20s %-20s %s\n",
				truncate(ipStr, 20),
				network.CIDR,
				status,
				truncate(alloc.Hostname, 20),
				truncate(alloc.Description, 20),
				alloc.AllocatedAt.Format("2006-01-02 15:04"),
			)
		}

		return nil
	},
}

func init() {
	listCmd.Flags().StringP("network-id", "n", "", "Filter by network ID")
	listCmd.Flags().BoolP("all", "a", false, "Show released allocations")
}
