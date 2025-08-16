package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "Manage networks",
	Long:  `Commands for managing network CIDRs in the IPAM system.`,
}

var networkAddCmd = &cobra.Command{
	Use:   "add [CIDR]",
	Short: "Add a new network CIDR",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cidr := args[0]
		description, _ := cmd.Flags().GetString("description")
		tagsStr, _ := cmd.Flags().GetString("tags")

		var tags []string
		if tagsStr != "" {
			tags = strings.Split(tagsStr, ",")
		}

		network, err := ipamClient.AddNetwork(cidr, description, tags)
		if err != nil {
			return fmt.Errorf("failed to add network: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Network added successfully:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  ID:          %s\n", network.ID)
		fmt.Fprintf(cmd.OutOrStdout(), "  CIDR:        %s\n", network.CIDR)
		fmt.Fprintf(cmd.OutOrStdout(), "  Description: %s\n", network.Description)
		if len(network.Tags) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Tags:        %s\n", strings.Join(network.Tags, ", "))
		}
		return nil
	},
}

var networkListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all networks",
	RunE: func(cmd *cobra.Command, args []string) error {
		networks, err := ipamStore.ListNetworks()
		if err != nil {
			return fmt.Errorf("failed to list networks: %w", err)
		}

		if len(networks) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No networks found.")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-20s %-30s %s\n", "ID", "CIDR", "Description", "Tags")
		fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 80))

		for _, network := range networks {
			tagsStr := strings.Join(network.Tags, ", ")
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-20s %-30s %s\n",
				network.ID,
				network.CIDR,
				truncate(network.Description, 30),
				tagsStr,
			)
		}
		return nil
	},
}

var networkDeleteCmd = &cobra.Command{
	Use:   "delete [ID]",
	Short: "Delete a network",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		// Check if there are any allocations
		allocations, err := ipamStore.ListAllocations(id)
		if err != nil {
			return fmt.Errorf("failed to check allocations: %w", err)
		}

		if len(allocations) > 0 {
			return fmt.Errorf("cannot delete network with active allocations")
		}

		if err := ipamStore.DeleteNetwork(id); err != nil {
			return fmt.Errorf("failed to delete network: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Network %s deleted successfully.\n", id)
		return nil
	},
}

func init() {
	networkCmd.AddCommand(networkAddCmd)
	networkCmd.AddCommand(networkListCmd)
	networkCmd.AddCommand(networkDeleteCmd)

	networkAddCmd.Flags().StringP("description", "d", "", "Network description")
	networkAddCmd.Flags().StringP("tags", "t", "", "Comma-separated tags")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
