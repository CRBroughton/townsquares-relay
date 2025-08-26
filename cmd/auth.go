package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"tailscale.com/tsnet"
)

var (
	timeout    int
	configFile string
)

var tailscaleCmd = &cobra.Command{
	Use:   "tailscale",
	Short: "Tailscale management commands",
	Long:  `Commands for managing Tailscale authentication and connectivity`,
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Generate Tailscale authentication URL",
	Long: `Generate a Tailscale authentication URL without starting the full relay server.
This is useful for initial setup or re-authentication of devices.

The command will start a Tailscale instance and display the authentication URL.
Press Ctrl+C to stop after you've copied the URL.`,
	Run: runAuth,
}

func init() {
	rootCmd.AddCommand(tailscaleCmd)
	tailscaleCmd.AddCommand(authCmd)

	// Also add auth as a direct command for convenience
	rootCmd.AddCommand(authCmd)

	authCmd.Flags().StringVarP(&configFile, "config", "c", "config.json", "Config file to read Tailscale settings from")
	authCmd.Flags().IntVarP(&timeout, "timeout", "t", 60, "Timeout in seconds before giving up (0 = no timeout)")
}

func runAuth(cmd *cobra.Command, args []string) {
	config, err := loadConfigForAuth()
	if err != nil {
		fmt.Printf("❌ Error loading config file '%s': %v\n", configFile, err)
		fmt.Println("💡 Make sure the config file exists and contains valid JSON")
		os.Exit(1)
	}

	if !config.TailscaleEnabled {
		fmt.Println("❌ Error: tailscale_enabled is false in config file")
		fmt.Println("💡 Set 'tailscale_enabled': true in your config file to enable Tailscale")
		os.Exit(1)
	}

	hostname := config.TailscaleHostname
	if hostname == "" {
		hostname = "townsquares-relay"
		fmt.Printf("🏷️  Using default hostname (no tailscale_hostname in config): %s\n", hostname)
	} else {
		fmt.Printf("🏷️  Using hostname from config: %s\n", hostname)
	}

	if config.TailscaleStateDir != "" {
		fmt.Printf("📁 Using state directory from config: %s\n", config.TailscaleStateDir)
	}

	fmt.Printf("🔗 Generating Tailscale authentication URL for hostname: %s\n", hostname)

	srv := &tsnet.Server{
		Hostname: hostname,
	}

	if config.TailscaleStateDir != "" {
		srv.Dir = config.TailscaleStateDir
	}

	fmt.Println("🚀 Starting Tailscale authentication process...")
	fmt.Println("💡 The authentication URL will appear below. Press Ctrl+C after copying it.")
	fmt.Println("")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil {
			fmt.Printf("❌ Error starting Tailscale server: %v\n", err)
		}
	}()

	// Set up timeout if specified
	if timeout > 0 {
		go func() {
			time.Sleep(time.Duration(timeout) * time.Second)
			fmt.Printf("\n⏰ Timeout reached after %d seconds\n", timeout)
			fmt.Println("💡 If no URL appeared, try with a clean state directory")
			os.Exit(0)
		}()
	}

	// Wait for user to interrupt
	<-sigChan
	fmt.Println("\n🛑 Stopping...")

	// Clean shutdown
	if err := srv.Close(); err != nil {
		log.Printf("Warning: error closing Tailscale server: %v", err)
	}

	fmt.Println("✅ Done!")
}

type AuthConfig struct {
	TailscaleEnabled  bool   `json:"tailscale_enabled,omitempty"`
	TailscaleHostname string `json:"tailscale_hostname,omitempty"`
	TailscaleStateDir string `json:"tailscale_state_dir,omitempty"`
	TailscaleHTTPS    bool   `json:"tailscale_https,omitempty"`
}

func loadConfigForAuth() (*AuthConfig, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var config AuthConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
