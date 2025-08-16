package cmd

import (
	"fmt"
	"strings"

	"github.com/jeremyhahn/go-ipam/pkg/ipam"
	"github.com/spf13/cobra"
)

var allocateCmd = &cobra.Command{
	Use:   "allocate",
	Short: "Allocate IP addresses",
	Long:  `Allocate one or more IP addresses from a network pool.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		networkID, _ := cmd.Flags().GetString("network-id")
		cidr, _ := cmd.Flags().GetString("cidr")
		count, _ := cmd.Flags().GetInt("count")
		description, _ := cmd.Flags().GetString("description")
		hostname, _ := cmd.Flags().GetString("hostname")
		tagsStr, _ := cmd.Flags().GetString("tags")
		ttl, _ := cmd.Flags().GetInt("ttl")

		// Validate count
		if count < 1 {
			return fmt.Errorf("count must be at least 1")
		}

		var tags []string
		if tagsStr != "" {
			tags = strings.Split(tagsStr, ",")
		}

		req := &ipam.AllocationRequest{
			NetworkID:   networkID,
			CIDR:        cidr,
			Count:       count,
			Description: description,
			Hostname:    hostname,
			Tags:        tags,
			TTL:         ttl,
		}

		allocation, err := ipamClient.AllocateIP(req)
		if err != nil {
			return fmt.Errorf("failed to allocate IP: %w", err)
		}

		if allocation.EndIP != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "IP range allocated successfully:\n")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "IP allocated successfully:\n")
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  ID:          %s\n", allocation.ID)
		fmt.Fprintf(cmd.OutOrStdout(), "  Network ID:  %s\n", allocation.NetworkID)
		if allocation.EndIP != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  IP Range:    %s - %s\n", allocation.IP, allocation.EndIP)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  IP:          %s\n", allocation.IP)
		}
		if allocation.Description != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Description: %s\n", allocation.Description)
		}
		if allocation.Hostname != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Hostname:    %s\n", allocation.Hostname)
		}
		if len(allocation.Tags) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Tags:        %s\n", strings.Join(allocation.Tags, ", "))
		}
		if allocation.ExpiresAt != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  Expires:     %s\n", allocation.ExpiresAt.Format("2006-01-02 15:04:05"))
		}

		return nil
	},
}

func init() {
	allocateCmd.Flags().StringP("network-id", "n", "", "Network ID to allocate from")
	allocateCmd.Flags().StringP("cidr", "c", "", "Network CIDR to allocate from")
	allocateCmd.Flags().IntP("count", "k", 1, "Number of IPs to allocate")
	allocateCmd.Flags().StringP("description", "d", "", "Description for the allocation")
	allocateCmd.Flags().StringP("hostname", "H", "", "Hostname for the allocation")
	allocateCmd.Flags().StringP("tags", "t", "", "Comma-separated tags")
	allocateCmd.Flags().IntP("ttl", "T", 0, "Time to live in seconds")
}
