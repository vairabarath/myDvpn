package client

import (
	"fmt"
	"sync"
	"time"

	"myDvpn/clientPeer/proto"
	"myDvpn/utils"
	"github.com/sirupsen/logrus"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// PeerMode represents the current operating mode of the peer
type PeerMode string

const (
	ModeClient PeerMode = "client" // Consuming VPN services
	ModeExit   PeerMode = "exit"   // Providing VPN services
	ModeHybrid PeerMode = "hybrid" // Both client and exit simultaneously
)

// UnifiedPeer represents a peer that can act as both client and exit
type UnifiedPeer struct {
	id              string
	region          string
	supernodeAddr   string
	logger          *logrus.Logger
	
	// Connection management
	streamManager   *PersistentStreamManager
	wgManager       *utils.WireGuardManager
	
	// Mode management
	currentMode     PeerMode
	modeMutex       sync.RWMutex
	
	// Client mode components
	clientInterface string
	clientPrivateKey wgtypes.Key
	currentExit     *UnifiedExitConfig
	
	// Exit mode components  
	exitInterface   string
	exitPrivateKey  wgtypes.Key
	exitListenPort  int
	activeClients   map[string]*ClientInfo
	clientsMux      sync.RWMutex
	ipAllocator     *IPAllocator
	
	// UI callbacks
	onModeChanged   func(PeerMode)
	onClientConnected func(*UnifiedExitConfig)
	onExitClientAdded func(*ClientInfo)
	
	mutex           sync.RWMutex
}

// UnifiedExitConfig represents connection to an exit peer
type UnifiedExitConfig struct {
	ExitPeerID    string
	PublicKey     string
	Endpoint      string
	AllowedIPs    []string
	SessionID     string
	ConnectedAt   time.Time
}

// ClientInfo represents a client connected to this exit peer
type ClientInfo struct {
	ClientID      string
	PublicKey     string
	AllocatedIP   string
	AllowedIPs    []string
	SessionID     string
	ConnectedAt   time.Time
}

// IPAllocator manages IP allocation for exit mode
type IPAllocator struct {
	cidr      string
	usedIPs   map[string]bool
	mutex     sync.Mutex
}

// NewIPAllocator creates a new IP allocator
func NewIPAllocator(cidr string) *IPAllocator {
	return &IPAllocator{
		cidr:    cidr,
		usedIPs: make(map[string]bool),
	}
}

// AllocateIP allocates an IP address for a client
func (ia *IPAllocator) AllocateIP() (string, error) {
	ia.mutex.Lock()
	defer ia.mutex.Unlock()

	ip, err := utils.AllocateClientIP(ia.cidr, ia.usedIPs)
	if err != nil {
		return "", err
	}

	ia.usedIPs[ip] = true
	return ip, nil
}

// ReleaseIP releases an IP address
func (ia *IPAllocator) ReleaseIP(ip string) {
	ia.mutex.Lock()
	defer ia.mutex.Unlock()
	delete(ia.usedIPs, ip)
}

// NewUnifiedPeer creates a new unified peer
func NewUnifiedPeer(id, region, supernodeAddr string, exitPort int, logger *logrus.Logger) (*UnifiedPeer, error) {
	// Create WireGuard manager
	wgManager, err := utils.NewWireGuardManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create WireGuard manager: %w", err)
	}

	// Generate keys for both modes
	clientPrivateKey, err := utils.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate client private key: %w", err)
	}

	exitPrivateKey, err := utils.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate exit private key: %w", err)
	}

	peer := &UnifiedPeer{
		id:              id,
		region:          region,
		supernodeAddr:   supernodeAddr,
		logger:          logger,
		wgManager:       wgManager,
		currentMode:     ModeClient, // Start in client mode
		
		// Client mode setup
		clientInterface:  fmt.Sprintf("wg-client-%s", id),
		clientPrivateKey: clientPrivateKey,
		
		// Exit mode setup
		exitInterface:   fmt.Sprintf("wg-exit-%s", id),
		exitPrivateKey:  exitPrivateKey,
		exitListenPort:  exitPort,
		activeClients:   make(map[string]*ClientInfo),
		ipAllocator:     NewIPAllocator("10.9.0.0/24"),
	}

	// Create stream manager with dynamic role reporting
	streamManager, err := NewPersistentStreamManager(id, peer.getCurrentRole(), region, supernodeAddr, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream manager: %w", err)
	}
	peer.streamManager = streamManager

	// Register custom command handlers for both modes
	peer.registerCommandHandlers()

	return peer, nil
}

// getCurrentRole returns the current role for SuperNode registration
func (up *UnifiedPeer) getCurrentRole() string {
	up.modeMutex.RLock()
	defer up.modeMutex.RUnlock()
	
	switch up.currentMode {
	case ModeClient:
		return "client"
	case ModeExit:
		return "exit"
	case ModeHybrid:
		return "hybrid"
	default:
		return "client"
	}
}

// Start starts the unified peer
func (up *UnifiedPeer) Start() error {
	// Start persistent stream
	if err := up.streamManager.Start(); err != nil {
		return fmt.Errorf("failed to start stream manager: %w", err)
	}

	// Initialize in client mode by default
	if err := up.initializeClientMode(); err != nil {
		return fmt.Errorf("failed to initialize client mode: %w", err)
	}

	up.logger.WithFields(logrus.Fields{
		"peer_id": up.id,
		"region":  up.region,
		"mode":    up.currentMode,
	}).Info("Unified peer started")

	return nil
}

// Stop stops the unified peer
func (up *UnifiedPeer) Stop() error {
	// Stop stream manager
	up.streamManager.Stop()

	// Cleanup both modes
	up.cleanupClientMode()
	up.cleanupExitMode()

	// Close WireGuard manager
	if err := up.wgManager.Close(); err != nil {
		up.logger.WithError(err).Warn("Failed to close WireGuard manager")
	}

	up.logger.WithField("peer_id", up.id).Info("Unified peer stopped")
	return nil
}

// ToggleExitMode toggles the peer between client and exit modes
func (up *UnifiedPeer) ToggleExitMode(enabled bool) error {
	up.modeMutex.Lock()
	defer up.modeMutex.Unlock()

	if enabled {
		return up.switchToExitMode()
	} else {
		return up.switchToClientMode()
	}
}

// switchToExitMode switches the peer to exit mode
func (up *UnifiedPeer) switchToExitMode() error {
	if up.currentMode == ModeExit || up.currentMode == ModeHybrid {
		return nil // Already in exit mode
	}

	up.logger.Info("Switching to exit mode...")

	// Initialize exit mode interface
	if err := up.initializeExitMode(); err != nil {
		return fmt.Errorf("failed to initialize exit mode: %w", err)
	}

	// Update mode
	oldMode := up.currentMode
	up.currentMode = ModeExit

	// Notify SuperNode of role change
	go up.updateSupernodeRole()

	// Notify UI
	if up.onModeChanged != nil {
		up.onModeChanged(up.currentMode)
	}

	up.logger.WithFields(logrus.Fields{
		"old_mode": oldMode,
		"new_mode": up.currentMode,
	}).Info("Switched to exit mode")

	return nil
}

// switchToClientMode switches the peer to client mode
func (up *UnifiedPeer) switchToClientMode() error {
	if up.currentMode == ModeClient {
		return nil // Already in client mode
	}

	up.logger.Info("Switching to client mode...")

	// Cleanup exit mode
	up.cleanupExitMode()

	// Update mode
	oldMode := up.currentMode
	up.currentMode = ModeClient

	// Notify SuperNode of role change
	go up.updateSupernodeRole()

	// Notify UI
	if up.onModeChanged != nil {
		up.onModeChanged(up.currentMode)
	}

	up.logger.WithFields(logrus.Fields{
		"old_mode": oldMode,
		"new_mode": up.currentMode,
	}).Info("Switched to client mode")

	return nil
}

// ConnectToExit connects to an exit peer (client mode)
func (up *UnifiedPeer) ConnectToExit(targetRegion string) (*UnifiedExitConfig, error) {
	up.modeMutex.RLock()
	defer up.modeMutex.RUnlock()

	if up.currentMode != ModeClient && up.currentMode != ModeHybrid {
		return nil, fmt.Errorf("peer is not in client mode")
	}

	up.mutex.Lock()
	defer up.mutex.Unlock()

	// TODO: Implement exit request to SuperNode
	// This would involve sending a request message to the SuperNode
	// For now, this is a placeholder that demonstrates the interface

	up.logger.WithFields(logrus.Fields{
		"peer_id":       up.id,
		"target_region": targetRegion,
	}).Info("Requesting exit peer connection")

	// Placeholder exit config
	exitConfig := &UnifiedExitConfig{
		ExitPeerID:  "exit-placeholder",
		PublicKey:   "placeholder-key",
		Endpoint:    "127.0.0.1:51820",
		AllowedIPs:  []string{"0.0.0.0/0"},
		SessionID:   "session-placeholder",
		ConnectedAt: time.Now(),
	}

	up.currentExit = exitConfig

	// Notify UI
	if up.onClientConnected != nil {
		up.onClientConnected(exitConfig)
	}

	return exitConfig, nil
}

// DisconnectFromExit disconnects from the current exit peer
func (up *UnifiedPeer) DisconnectFromExit() error {
	up.mutex.Lock()
	defer up.mutex.Unlock()

	if up.currentExit == nil {
		return fmt.Errorf("not connected to any exit peer")
	}

	// Remove peer from WireGuard
	if err := up.wgManager.RemovePeer(up.clientInterface, up.currentExit.PublicKey); err != nil {
		up.logger.WithError(err).Warn("Failed to remove exit peer from WireGuard")
	}

	up.logger.WithFields(logrus.Fields{
		"peer_id":    up.id,
		"exit_peer":  up.currentExit.ExitPeerID,
		"session_id": up.currentExit.SessionID,
	}).Info("Disconnected from exit peer")

	up.currentExit = nil
	return nil
}

// initializeClientMode sets up client mode interface
func (up *UnifiedPeer) initializeClientMode() error {
	// Create client interface
	if err := up.wgManager.CreateInterface(up.clientInterface); err != nil {
		return fmt.Errorf("failed to create client interface: %w", err)
	}

	// Set private key
	if err := up.wgManager.SetInterfacePrivateKey(up.clientInterface, up.clientPrivateKey); err != nil {
		return fmt.Errorf("failed to set client private key: %w", err)
	}

	up.logger.WithFields(logrus.Fields{
		"interface":   up.clientInterface,
		"public_key":  up.clientPrivateKey.PublicKey().String(),
	}).Info("Client mode interface initialized")

	return nil
}

// initializeExitMode sets up exit mode interface
func (up *UnifiedPeer) initializeExitMode() error {
	// Create exit interface
	if err := up.wgManager.CreateInterface(up.exitInterface); err != nil {
		return fmt.Errorf("failed to create exit interface: %w", err)
	}

	// Set private key
	if err := up.wgManager.SetInterfacePrivateKey(up.exitInterface, up.exitPrivateKey); err != nil {
		return fmt.Errorf("failed to set exit private key: %w", err)
	}

	// Set listen port
	if err := up.wgManager.SetInterfaceListenPort(up.exitInterface, up.exitListenPort); err != nil {
		return fmt.Errorf("failed to set exit listen port: %w", err)
	}

	// Set interface IP
	if err := up.wgManager.SetInterfaceIP(up.exitInterface, "10.9.0.1/24"); err != nil {
		return fmt.Errorf("failed to set exit interface IP: %w", err)
	}

	// Enable IP forwarding and NAT
	if err := utils.EnableIPForwarding(); err != nil {
		return fmt.Errorf("failed to enable IP forwarding: %w", err)
	}

	if err := utils.AddNATRule(up.exitInterface, "eth0"); err != nil {
		return fmt.Errorf("failed to add NAT rule: %w", err)
	}

	up.logger.WithFields(logrus.Fields{
		"interface":   up.exitInterface,
		"listen_port": up.exitListenPort,
		"public_key":  up.exitPrivateKey.PublicKey().String(),
	}).Info("Exit mode interface initialized")

	return nil
}

// cleanupClientMode cleans up client mode interface
func (up *UnifiedPeer) cleanupClientMode() {
	if err := up.wgManager.DeleteInterface(up.clientInterface); err != nil {
		up.logger.WithError(err).Warn("Failed to delete client interface")
	}
}

// cleanupExitMode cleans up exit mode interface
func (up *UnifiedPeer) cleanupExitMode() {
	// Remove all clients
	up.clientsMux.Lock()
	for clientID := range up.activeClients {
		up.removeClientUnsafe(clientID)
	}
	up.clientsMux.Unlock()

	// Delete interface
	if err := up.wgManager.DeleteInterface(up.exitInterface); err != nil {
		up.logger.WithError(err).Warn("Failed to delete exit interface")
	}
}

// registerCommandHandlers registers command handlers for both modes
func (up *UnifiedPeer) registerCommandHandlers() {
	up.streamManager.RegisterCommandHandler(proto.CommandType_SETUP_EXIT, up.handleSetupExitCommand)
	up.streamManager.RegisterCommandHandler(proto.CommandType_ROTATE_PEER, up.handleRotatePeerCommand)
	up.streamManager.RegisterCommandHandler(proto.CommandType_RELAY_SETUP, up.handleRelaySetupCommand)
	up.streamManager.RegisterCommandHandler(proto.CommandType_DISCONNECT, up.handleDisconnectCommand)
}

// handleSetupExitCommand handles SETUP_EXIT commands (exit mode)
func (up *UnifiedPeer) handleSetupExitCommand(cmd *proto.Command) *proto.CommandResponse {
	up.modeMutex.RLock()
	defer up.modeMutex.RUnlock()

	if up.currentMode != ModeExit && up.currentMode != ModeHybrid {
		return &proto.CommandResponse{
			CommandId: cmd.CommandId,
			Success:   false,
			Message:   "Peer is not in exit mode",
		}
	}

	clientID := cmd.Payload["client_id"]
	clientPubKey := cmd.Payload["client_pubkey"]
	sessionID := cmd.Payload["session_id"]

	if clientID == "" || clientPubKey == "" || sessionID == "" {
		return &proto.CommandResponse{
			CommandId: cmd.CommandId,
			Success:   false,
			Message:   "Missing required parameters",
		}
	}

	if err := up.addClient(clientID, clientPubKey, sessionID); err != nil {
		up.logger.WithError(err).Error("Failed to add client")
		return &proto.CommandResponse{
			CommandId: cmd.CommandId,
			Success:   false,
			Message:   fmt.Sprintf("Failed to add client: %v", err),
		}
	}

	return &proto.CommandResponse{
		CommandId: cmd.CommandId,
		Success:   true,
		Message:   "Client added successfully",
	}
}

// addClient adds a client in exit mode
func (up *UnifiedPeer) addClient(clientID, clientPubKey, sessionID string) error {
	up.clientsMux.Lock()
	defer up.clientsMux.Unlock()

	// Check if client already exists
	if _, exists := up.activeClients[clientID]; exists {
		return fmt.Errorf("client %s already exists", clientID)
	}

	// Allocate IP for client
	allocatedIP, err := up.ipAllocator.AllocateIP()
	if err != nil {
		return fmt.Errorf("failed to allocate IP: %w", err)
	}

	// Add peer to WireGuard
	peerConfig := utils.PeerConfig{
		PublicKey:  clientPubKey,
		AllowedIPs: []string{fmt.Sprintf("%s/32", allocatedIP)},
	}

	if err := up.wgManager.AddPeer(up.exitInterface, peerConfig); err != nil {
		up.ipAllocator.ReleaseIP(allocatedIP)
		return fmt.Errorf("failed to add peer to WireGuard: %w", err)
	}

	// Store client info
	clientInfo := &ClientInfo{
		ClientID:    clientID,
		PublicKey:   clientPubKey,
		AllocatedIP: allocatedIP,
		AllowedIPs:  []string{"0.0.0.0/0"},
		SessionID:   sessionID,
		ConnectedAt: time.Now(),
	}

	up.activeClients[clientID] = clientInfo

	// Notify UI
	if up.onExitClientAdded != nil {
		up.onExitClientAdded(clientInfo)
	}

	up.logger.WithFields(logrus.Fields{
		"client_id":    clientID,
		"allocated_ip": allocatedIP,
		"session_id":   sessionID,
	}).Info("Added client in exit mode")

	return nil
}

// removeClientUnsafe removes a client without locking
func (up *UnifiedPeer) removeClientUnsafe(clientID string) error {
	clientInfo, exists := up.activeClients[clientID]
	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}

	// Remove peer from WireGuard
	if err := up.wgManager.RemovePeer(up.exitInterface, clientInfo.PublicKey); err != nil {
		return fmt.Errorf("failed to remove peer from WireGuard: %w", err)
	}

	// Release IP
	up.ipAllocator.ReleaseIP(clientInfo.AllocatedIP)

	// Remove from active clients
	delete(up.activeClients, clientID)

	up.logger.WithFields(logrus.Fields{
		"client_id":    clientID,
		"allocated_ip": clientInfo.AllocatedIP,
	}).Info("Removed client from exit mode")

	return nil
}

// updateSupernodeRole notifies SuperNode of role change
func (up *UnifiedPeer) updateSupernodeRole() {
	// TODO: Implement role update to SuperNode
	// This would involve re-authenticating with the new role
	up.logger.WithField("new_role", up.getCurrentRole()).Info("Updated SuperNode role")
}

// Placeholder handlers for other commands
func (up *UnifiedPeer) handleRotatePeerCommand(cmd *proto.Command) *proto.CommandResponse {
	return &proto.CommandResponse{
		CommandId: cmd.CommandId,
		Success:   true,
		Message:   "Rotate peer command handled",
	}
}

func (up *UnifiedPeer) handleRelaySetupCommand(cmd *proto.Command) *proto.CommandResponse {
	return &proto.CommandResponse{
		CommandId: cmd.CommandId,
		Success:   true,
		Message:   "Relay setup command handled",
	}
}

func (up *UnifiedPeer) handleDisconnectCommand(cmd *proto.Command) *proto.CommandResponse {
	// Gracefully disconnect
	go func() {
		time.Sleep(1 * time.Second)
		up.Stop()
	}()
	
	return &proto.CommandResponse{
		CommandId: cmd.CommandId,
		Success:   true,
		Message:   "Disconnect command handled",
	}
}

// UI Callback setters
func (up *UnifiedPeer) SetModeChangedCallback(callback func(PeerMode)) {
	up.onModeChanged = callback
}

func (up *UnifiedPeer) SetClientConnectedCallback(callback func(*UnifiedExitConfig)) {
	up.onClientConnected = callback
}

func (up *UnifiedPeer) SetExitClientAddedCallback(callback func(*ClientInfo)) {
	up.onExitClientAdded = callback
}

// Getters
func (up *UnifiedPeer) GetCurrentMode() PeerMode {
	up.modeMutex.RLock()
	defer up.modeMutex.RUnlock()
	return up.currentMode
}

func (up *UnifiedPeer) GetCurrentExit() *UnifiedExitConfig {
	up.mutex.RLock()
	defer up.mutex.RUnlock()
	return up.currentExit
}

func (up *UnifiedPeer) GetActiveClients() []*ClientInfo {
	up.clientsMux.RLock()
	defer up.clientsMux.RUnlock()

	var clients []*ClientInfo
	for _, client := range up.activeClients {
		clients = append(clients, client)
	}
	return clients
}

func (up *UnifiedPeer) GetStats() map[string]interface{} {
	up.modeMutex.RLock()
	up.mutex.RLock()
	up.clientsMux.RLock()
	defer up.clientsMux.RUnlock()
	defer up.mutex.RUnlock()
	defer up.modeMutex.RUnlock()

	stats := map[string]interface{}{
		"peer_id":     up.id,
		"region":      up.region,
		"mode":        up.currentMode,
		"connected":   up.streamManager.IsConnected(),
		"session_id":  up.streamManager.GetSessionID(),
	}

	if up.currentMode == ModeClient || up.currentMode == ModeHybrid {
		stats["client_interface"] = up.clientInterface
		if up.currentExit != nil {
			stats["current_exit"] = map[string]interface{}{
				"exit_peer_id": up.currentExit.ExitPeerID,
				"endpoint":     up.currentExit.Endpoint,
				"session_id":   up.currentExit.SessionID,
				"connected_at": up.currentExit.ConnectedAt,
			}
		}
	}

	if up.currentMode == ModeExit || up.currentMode == ModeHybrid {
		stats["exit_interface"] = up.exitInterface
		stats["exit_listen_port"] = up.exitListenPort
		stats["active_clients"] = len(up.activeClients)
		stats["exit_public_key"] = up.exitPrivateKey.PublicKey().String()
	}

	return stats
}