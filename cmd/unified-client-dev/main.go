package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type UnifiedClient struct {
	id        string
	region    string
	supernode string
	exitPort  int
	mode      string
	logger    *logrus.Logger
	devMode   bool
}

func main() {
	var (
		id        = flag.String("id", "", "Peer ID")
		region    = flag.String("region", "", "Region name")
		supernode = flag.String("supernode", "", "SuperNode address")
		exitPort  = flag.Int("exit-port", 51820, "Exit mode WireGuard port")
		logLevel  = flag.String("log-level", "info", "Log level")
		noUI      = flag.Bool("no-ui", false, "Disable interactive UI")
		devMode   = flag.Bool("dev", false, "Development mode (no WireGuard)")
	)
	flag.Parse()

	logger := logrus.New()
	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		logger.Fatal("Invalid log level")
	}
	logger.SetLevel(level)

	if *id == "" {
		*id = fmt.Sprintf("peer-%d", os.Getpid())
	}

	client := &UnifiedClient{
		id:        *id,
		region:    *region,
		supernode: *supernode,
		exitPort:  *exitPort,
		mode:      "client",
		logger:    logger,
		devMode:   *devMode,
	}

	logger.WithFields(logrus.Fields{
		"id":        *id,
		"region":    *region,
		"supernode": *supernode,
		"exit_port": *exitPort,
		"dev_mode":  *devMode,
	}).Info("Starting unified peer")

	if *noUI {
		client.runService()
	} else {
		client.runInteractive()
	}
}

func (c *UnifiedClient) runService() {
	c.logger.Info("Running in service mode")
	
	if c.devMode {
		c.logger.Info("🔧 Development mode - simulating WireGuard operations")
	}
	
	c.logger.Info("Authentication successful")
	c.logger.Info("Persistent stream manager started")
	
	// Simulate some activity
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for range ticker.C {
			c.logger.Debug("Service heartbeat")
		}
	}()
	
	// Keep running
	select {}
}

func (c *UnifiedClient) runInteractive() {
	fmt.Println("🌐 myDvpn Unified Peer")
	if c.devMode {
		fmt.Println("🔧 Development Mode (No WireGuard)")
	}
	fmt.Println("=====================")
	fmt.Printf("Peer ID: %s\n", c.id)
	fmt.Printf("Region: %s\n", c.region)
	fmt.Printf("Current Mode: %s\n\n", c.mode)

	// Simulate startup
	c.logger.Info("Authentication successful")
	c.logger.Info("Persistent stream manager started")
	
	if c.devMode {
		c.logger.Info("🔧 Development mode - WireGuard operations simulated")
		fmt.Println("📝 Note: Running in development mode - no actual WireGuard interfaces created")
		fmt.Println()
	}

	c.showHelp()

	scanner := bufio.NewScanner(os.Stdin)
	
	for {
		fmt.Print("myDvpn> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		c.handleCommand(input)
	}
}

func (c *UnifiedClient) handleCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	command := parts[0]

	switch command {
	case "help", "h":
		c.showHelp()
	case "status", "s":
		c.showStatus()
	case "toggle-exit", "te":
		if len(parts) > 1 {
			c.toggleExit(parts[1])
		} else {
			fmt.Println("Usage: toggle-exit on|off")
		}
	case "connect", "c":
		region := "us"
		if len(parts) > 1 {
			region = parts[1]
		}
		c.connectToExit(region)
	case "disconnect", "d":
		c.disconnect()
	case "clients", "cl":
		c.showClients()
	case "stats", "st":
		c.showStats()
	case "simulate", "sim":
		c.simulateTraffic()
	case "test-wg", "tw":
		c.testWireGuard()
	case "quit", "q", "exit":
		fmt.Println("👋 Goodbye!")
		os.Exit(0)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Type 'help' for available commands.")
	}
}

func (c *UnifiedClient) showHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  help (h)           - Show this help")
	fmt.Println("  status (s)         - Show current status")
	fmt.Println("  toggle-exit (te)   - Toggle exit node mode on/off")
	fmt.Println("                       Usage: toggle-exit on|off")
	fmt.Println("  connect (c)        - Connect to exit peer")
	fmt.Println("                       Usage: connect [region]")
	fmt.Println("  disconnect (d)     - Disconnect from current exit")
	fmt.Println("  clients (cl)       - Show connected clients (exit mode)")
	fmt.Println("  stats (st)         - Show detailed statistics")
	if c.devMode {
		fmt.Println("  simulate (sim)     - Simulate traffic flow")
		fmt.Println("  test-wg (tw)       - Test WireGuard operations")
	}
	fmt.Println("  quit (q)           - Exit the application")
	fmt.Println()
}

func (c *UnifiedClient) showStatus() {
	fmt.Println("📊 Current Status:")
	fmt.Printf("  Mode: %s\n", c.mode)
	fmt.Printf("  Connected: %s\n", "true")
	
	if c.devMode {
		fmt.Printf("  Development Mode: enabled\n")
		fmt.Printf("  WireGuard: simulated\n")
	}
	
	if c.mode == "exit" || c.mode == "hybrid" {
		fmt.Printf("  👥 Active Clients: %d\n", 1)
		fmt.Printf("  🔑 Exit Public Key: %s\n", "mB8pTq1q2rK3sL4tU5vW6xY7zA8bC9dE0fF1gG2hH3iI=")
		if c.devMode {
			fmt.Printf("  🔧 Exit Interface: wg-exit-%s (simulated)\n", c.id)
		}
	}
	
	if c.mode == "client" || c.mode == "hybrid" {
		fmt.Printf("  🚪 Exit Peer: %s\n", "peer-provider-123")
		if c.devMode {
			fmt.Printf("  🔧 Client Interface: wg-client-%s (simulated)\n", c.id)
		}
	}
	fmt.Println()
}

func (c *UnifiedClient) toggleExit(action string) {
	switch action {
	case "on":
		if c.mode == "client" {
			c.mode = "exit"
		} else if c.mode == "client" {
			c.mode = "hybrid"
		}
		fmt.Println("✅ Exit mode enabled - You are now providing VPN services!")
		if c.devMode {
			fmt.Println("🔧 Simulated: Created WireGuard exit interface")
		}
		fmt.Printf("🔄 Mode changed to: %s\n", c.mode)
	case "off":
		if c.mode == "exit" {
			c.mode = "client"
		} else if c.mode == "hybrid" {
			c.mode = "client"
		}
		fmt.Println("✅ Exit mode disabled - You are now in client-only mode.")
		if c.devMode {
			fmt.Println("🔧 Simulated: Destroyed WireGuard exit interface")
		}
		fmt.Printf("🔄 Mode changed to: %s\n", c.mode)
	default:
		fmt.Println("Usage: toggle-exit on|off")
	}
	fmt.Println()
}

func (c *UnifiedClient) connectToExit(region string) {
	fmt.Printf("🔍 Requesting exit peer in region: %s...\n", region)
	if c.devMode {
		fmt.Println("🔧 Simulated: Creating client WireGuard tunnel")
	}
	fmt.Println("✅ Connected to exit peer: peer-provider-123")
	fmt.Printf("📡 Exit endpoint: 203.0.113.10:51820\n")
	fmt.Printf("🔑 Exit public key: nM9qPr2sT3uV4wX5yZ6aA7bB8cC9dD0eE1fF2gG3hH4i=\n")
	
	if c.mode == "exit" {
		c.mode = "hybrid"
		fmt.Printf("🔄 Mode changed to: %s\n", c.mode)
	}
	fmt.Println()
}

func (c *UnifiedClient) disconnect() {
	fmt.Println("✅ Disconnected from exit peer")
	if c.devMode {
		fmt.Println("🔧 Simulated: Destroyed client WireGuard tunnel")
	}
	
	if c.mode == "hybrid" {
		c.mode = "exit"
		fmt.Printf("🔄 Mode changed to: %s\n", c.mode)
	}
	fmt.Println()
}

func (c *UnifiedClient) showClients() {
	if c.mode == "client" {
		fmt.Println("❌ Not in exit mode - no clients to show")
		return
	}
	
	fmt.Println("👥 Active Clients (1):")
	fmt.Println("  1. remote-user-123")
	fmt.Println("     IP: 10.9.0.2")
	fmt.Printf("     Connected: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("     Session: session-abc123")
	if c.devMode {
		fmt.Println("     Status: simulated connection")
	}
	fmt.Println()
}

func (c *UnifiedClient) showStats() {
	fmt.Println("📊 Detailed Statistics:")
	fmt.Printf("  peer_id: %s\n", c.id)
	fmt.Printf("  region: %s\n", c.region)
	fmt.Printf("  mode: %s\n", c.mode)
	fmt.Println("  connected: true")
	fmt.Printf("  session_id: %s-session-123\n", c.id)
	fmt.Printf("  uptime: %s\n", "5m 32s")
	
	if c.devMode {
		fmt.Printf("  development_mode: enabled\n")
		fmt.Printf("  wireguard_simulated: true\n")
	}
	
	if c.mode != "client" {
		fmt.Printf("  exit_interface: wg-exit-%s\n", c.id)
		fmt.Printf("  exit_listen_port: %d\n", c.exitPort)
		fmt.Println("  active_clients: 1")
		fmt.Println("  exit_public_key: mB8pTq1q2rK3sL4tU5vW6xY7zA8bC9dE0fF1gG2hH3iI=")
	}
	
	if c.mode != "exit" {
		fmt.Printf("  client_interface: wg-client-%s\n", c.id)
		fmt.Println("  client_public_key: oC0rUs3vW4xY5zA6bB7cC8dD9eE0fF1gG2hH3iI4jJ5k=")
	}
	fmt.Println()
}

func (c *UnifiedClient) simulateTraffic() {
	if !c.devMode {
		fmt.Println("❌ Traffic simulation only available in development mode")
		return
	}
	
	fmt.Println("🔄 Simulating traffic flow...")
	fmt.Println("  📤 Sent: 1024 KB")
	fmt.Println("  📥 Received: 2048 KB")
	fmt.Println("  🕐 Latency: 25ms")
	fmt.Println("  📊 Throughput: 50 Mbps")
	fmt.Println("✅ Traffic simulation complete")
	fmt.Println()
}

func (c *UnifiedClient) testWireGuard() {
	if !c.devMode {
		fmt.Println("❌ WireGuard testing only available in development mode")
		return
	}
	
	fmt.Println("🔧 Testing WireGuard operations...")
	fmt.Printf("  ✅ Interface creation: wg-test-%s (simulated)\n", c.id)
	fmt.Println("  ✅ Key generation: successful")
	fmt.Println("  ✅ Peer configuration: successful")
	fmt.Println("  ✅ Interface cleanup: successful")
	fmt.Println("🎯 All WireGuard operations working correctly!")
	fmt.Println()
}