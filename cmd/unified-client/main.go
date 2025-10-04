package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"myDvpn/clientPeer/client"
	"github.com/sirupsen/logrus"
)

// UIInterface represents the simple text-based UI
type UIInterface struct {
	peer   *client.UnifiedPeer
	logger *logrus.Logger
	scanner *bufio.Scanner
}

func main() {
	// Parse command line flags
	id := flag.String("id", "peer-1", "Peer ID")
	region := flag.String("region", "us-east-1", "Region")
	supernodeAddr := flag.String("supernode", "localhost:50052", "SuperNode address")
	exitPort := flag.Int("exit-port", 51820, "WireGuard listen port for exit mode")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	noUI := flag.Bool("no-ui", false, "Disable interactive UI")
	flag.Parse()

	// Setup logger
	logger := logrus.New()
	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		logger.Fatal("Invalid log level")
	}
	logger.SetLevel(level)

	// Create unified peer
	peer, err := client.NewUnifiedPeer(*id, *region, *supernodeAddr, *exitPort, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create unified peer")
	}

	// Setup UI callbacks
	peer.SetModeChangedCallback(func(mode client.PeerMode) {
		fmt.Printf("\n🔄 Mode changed to: %s\n", mode)
		printPrompt()
	})

	peer.SetClientConnectedCallback(func(config *client.UnifiedExitConfig) {
		fmt.Printf("\n✅ Connected to exit peer: %s (endpoint: %s)\n", 
			config.ExitPeerID, config.Endpoint)
		printPrompt()
	})

	peer.SetExitClientAddedCallback(func(clientInfo *client.ClientInfo) {
		fmt.Printf("\n👤 New client connected: %s (IP: %s)\n", 
			clientInfo.ClientID, clientInfo.AllocatedIP)
		printPrompt()
	})

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start peer in goroutine
	go func() {
		logger.WithFields(logrus.Fields{
			"id":        *id,
			"region":    *region,
			"supernode": *supernodeAddr,
			"exit_port": *exitPort,
		}).Info("Starting unified peer")
		
		if err := peer.Start(); err != nil {
			logger.WithError(err).Fatal("Unified peer failed")
		}
	}()

	// Start UI if enabled
	if !*noUI {
		ui := &UIInterface{
			peer:    peer,
			logger:  logger,
			scanner: bufio.NewScanner(os.Stdin),
		}
		go ui.runInteractiveUI()
	}

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\n🛑 Shutting down...")
	peer.Stop()
}

func (ui *UIInterface) runInteractiveUI() {
	fmt.Println("🌐 myDvpn Unified Peer")
	fmt.Println("=====================")
	fmt.Printf("Peer ID: %s\n", ui.peer.GetStats()["peer_id"])
	fmt.Printf("Region: %s\n", ui.peer.GetStats()["region"])
	fmt.Printf("Current Mode: %s\n", ui.peer.GetCurrentMode())
	fmt.Println()
	
	ui.printHelp()
	
	for {
		printPrompt()
		
		if !ui.scanner.Scan() {
			break
		}
		
		input := strings.TrimSpace(ui.scanner.Text())
		if input == "" {
			continue
		}
		
		ui.handleCommand(input)
	}
}

func (ui *UIInterface) handleCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}
	
	command := strings.ToLower(parts[0])
	
	switch command {
	case "help", "h":
		ui.printHelp()
		
	case "status", "s":
		ui.printStatus()
		
	case "toggle-exit", "te":
		ui.handleToggleExit(parts)
		
	case "connect", "c":
		ui.handleConnect(parts)
		
	case "disconnect", "d":
		ui.handleDisconnect()
		
	case "clients", "cl":
		ui.printActiveClients()
		
	case "stats", "st":
		ui.printDetailedStats()
		
	case "quit", "q", "exit":
		fmt.Println("👋 Goodbye!")
		os.Exit(0)
		
	default:
		fmt.Printf("❌ Unknown command: %s\n", command)
		fmt.Println("Type 'help' for available commands.")
	}
}

func (ui *UIInterface) printHelp() {
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
	fmt.Println("  quit (q)           - Exit the application")
	fmt.Println()
}

func (ui *UIInterface) printStatus() {
	stats := ui.peer.GetStats()
	mode := ui.peer.GetCurrentMode()
	
	fmt.Println("📊 Current Status:")
	fmt.Printf("  Mode: %s\n", mode)
	fmt.Printf("  Connected: %v\n", stats["connected"])
	
	if mode == client.ModeClient || mode == client.ModeHybrid {
		if exit := ui.peer.GetCurrentExit(); exit != nil {
			fmt.Printf("  🚪 Exit Peer: %s (%s)\n", exit.ExitPeerID, exit.Endpoint)
		} else {
			fmt.Println("  🚪 Exit Peer: Not connected")
		}
	}
	
	if mode == client.ModeExit || mode == client.ModeHybrid {
		clients := ui.peer.GetActiveClients()
		fmt.Printf("  👥 Active Clients: %d\n", len(clients))
		if stats["exit_public_key"] != nil {
			fmt.Printf("  🔑 Exit Public Key: %s\n", stats["exit_public_key"])
		}
	}
	fmt.Println()
}

func (ui *UIInterface) handleToggleExit(parts []string) {
	if len(parts) < 2 {
		fmt.Println("❌ Usage: toggle-exit on|off")
		return
	}
	
	enabled := strings.ToLower(parts[1]) == "on"
	
	if err := ui.peer.ToggleExitMode(enabled); err != nil {
		fmt.Printf("❌ Failed to toggle exit mode: %v\n", err)
		return
	}
	
	if enabled {
		fmt.Println("✅ Exit mode enabled - You are now providing VPN services!")
		fmt.Println("   Other peers can connect through you.")
	} else {
		fmt.Println("✅ Exit mode disabled - You are now in client-only mode.")
	}
}

func (ui *UIInterface) handleConnect(parts []string) {
	currentMode := ui.peer.GetCurrentMode()
	if currentMode != client.ModeClient && currentMode != client.ModeHybrid {
		fmt.Println("❌ Cannot connect: peer is not in client mode")
		fmt.Println("   Use 'toggle-exit off' to enable client mode")
		return
	}
	
	if ui.peer.GetCurrentExit() != nil {
		fmt.Println("❌ Already connected to an exit peer")
		fmt.Println("   Use 'disconnect' first")
		return
	}
	
	targetRegion := "us-west-1" // Default
	if len(parts) > 1 {
		targetRegion = parts[1]
	}
	
	fmt.Printf("🔍 Requesting exit peer in region: %s...\n", targetRegion)
	
	exitConfig, err := ui.peer.ConnectToExit(targetRegion)
	if err != nil {
		fmt.Printf("❌ Failed to connect: %v\n", err)
		return
	}
	
	fmt.Printf("✅ Connected to exit peer: %s\n", exitConfig.ExitPeerID)
	fmt.Printf("   Endpoint: %s\n", exitConfig.Endpoint)
	fmt.Printf("   Session: %s\n", exitConfig.SessionID)
}

func (ui *UIInterface) handleDisconnect() {
	if ui.peer.GetCurrentExit() == nil {
		fmt.Println("❌ Not connected to any exit peer")
		return
	}
	
	if err := ui.peer.DisconnectFromExit(); err != nil {
		fmt.Printf("❌ Failed to disconnect: %v\n", err)
		return
	}
	
	fmt.Println("✅ Disconnected from exit peer")
}

func (ui *UIInterface) printActiveClients() {
	currentMode := ui.peer.GetCurrentMode()
	if currentMode != client.ModeExit && currentMode != client.ModeHybrid {
		fmt.Println("❌ Not in exit mode - no clients to show")
		return
	}
	
	clients := ui.peer.GetActiveClients()
	if len(clients) == 0 {
		fmt.Println("👥 No clients currently connected")
		return
	}
	
	fmt.Printf("👥 Active Clients (%d):\n", len(clients))
	for i, client := range clients {
		fmt.Printf("  %d. %s\n", i+1, client.ClientID)
		fmt.Printf("     IP: %s\n", client.AllocatedIP)
		fmt.Printf("     Connected: %s\n", client.ConnectedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("     Session: %s\n", client.SessionID)
		fmt.Println()
	}
}

func (ui *UIInterface) printDetailedStats() {
	stats := ui.peer.GetStats()
	
	fmt.Println("📊 Detailed Statistics:")
	for key, value := range stats {
		switch v := value.(type) {
		case map[string]interface{}:
			fmt.Printf("  %s:\n", key)
			for k2, v2 := range v {
				fmt.Printf("    %s: %v\n", k2, v2)
			}
		default:
			fmt.Printf("  %s: %v\n", key, value)
		}
	}
	fmt.Println()
}

func printPrompt() {
	fmt.Print("myDvpn> ")
}