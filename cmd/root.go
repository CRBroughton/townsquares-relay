package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "townsquares-relay",
	Short: "Townsquares relay - Powering decentralised community focused social networks ",
	Long: `Townsquares Relay is a nostr relay designed for hyper-localised communities.
Townsquares supports Tailscale networking for secure, private relay mesh networks.

You can run the relay server or manage Tailscale authentication separately.`,
	Version: "1.0.0",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
