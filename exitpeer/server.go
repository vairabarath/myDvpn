package exitpeer

import (
	"fmt"
	"sync"
	"time"

	"myDvpn/clientPeer/client"
	"myDvpn/clientPeer/proto"
	"myDvpn/utils"
	"github.com/sirupsen/logrus"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// ExitPeer represents an exit peer server
type ExitPeer struct {
	id              string
	region          string
	supernodeAddr   string
	logger          *logrus.Logger
	
	streamManager   *client.PersistentStreamManager
	wgManager       *utils.WireGuardManager
	
	// WireGuard configuration
	interfaceName   string
	privateKey      wgtypes.Key
	listenPort      int
	
	// Client management
	activeClients   map[string]*ClientInfo
	clientsMux      sync.RWMutex
	ipAllocator     *IPAllocator
}

// ClientInfo represents information about a connected client
type ClientInfo struct {
	ClientID      string
	PublicKey     string
	AllocatedIP   string
	AllowedIPs    []string
	SessionID     string
	SetupTime     int64
}

// IPAllocator manages IP allocation for clients
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

// NewExitPeer creates a new exit peer
func NewExitPeer(id, region, supernodeAddr string, listenPort int, logger *logrus.Logger) (*ExitPeer, error) {
	// Create persistent stream manager
	streamManager, err := client.NewPersistentStreamManager(id, "exit", region, supernodeAddr, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream manager: %w", err)
	}

	// Create WireGuard manager
	wgManager, err := utils.NewWireGuardManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create WireGuard manager: %w", err)
	}

	// Generate private key
	privateKey, err := utils.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	ep := &ExitPeer{
		id:            id,
		region:        region,
		supernodeAddr: supernodeAddr,
		logger:        logger,
		streamManager: streamManager,
		wgManager:     wgManager,
		interfaceName: fmt.Sprintf("wg-exit-%s", id),
		privateKey:    privateKey,
		listenPort:    listenPort,
		activeClients: make(map[string]*ClientInfo),
		ipAllocator:   NewIPAllocator("10.9.0.0/24"), // Exit peer network
	}

	// Register custom command handlers
	ep.registerCommandHandlers()

	return ep, nil
}

// Start starts the exit peer
func (ep *ExitPeer) Start() error {
	// Initialize WireGuard interface
	if err := ep.initializeWireGuard(); err != nil {
		return fmt.Errorf("failed to initialize WireGuard: %w", err)
	}

	// Enable IP forwarding and NAT
	if err := ep.enableForwarding(); err != nil {
		return fmt.Errorf("failed to enable forwarding: %w", err)
	}

	// Start persistent stream
	if err := ep.streamManager.Start(); err != nil {
		return fmt.Errorf("failed to start stream manager: %w", err)
	}

	ep.logger.WithFields(logrus.Fields{
		"peer_id":    ep.id,
		"region":     ep.region,
		"interface":  ep.interfaceName,
		"listen_port": ep.listenPort,
		"public_key": ep.privateKey.PublicKey().String(),
	}).Info("Exit peer started")

	return nil
}

// Stop stops the exit peer
func (ep *ExitPeer) Stop() error {
	// Remove all clients
	ep.clientsMux.Lock()
	for clientID := range ep.activeClients {
		if err := ep.removeClientUnsafe(clientID); err != nil {
			ep.logger.WithError(err).WithField("client_id", clientID).Warn("Failed to remove client during shutdown")
		}
	}
	ep.clientsMux.Unlock()

	// Stop stream manager
	ep.streamManager.Stop()

	// Cleanup WireGuard
	if err := ep.cleanupWireGuard(); err != nil {
		ep.logger.WithError(err).Warn("Failed to cleanup WireGuard interface")
	}

	// Close WireGuard manager
	if err := ep.wgManager.Close(); err != nil {
		ep.logger.WithError(err).Warn("Failed to close WireGuard manager")
	}

	ep.logger.WithField("peer_id", ep.id).Info("Exit peer stopped")
	return nil
}

// initializeWireGuard initializes the WireGuard interface
func (ep *ExitPeer) initializeWireGuard() error {
	// Create interface
	if err := ep.wgManager.CreateInterface(ep.interfaceName); err != nil {
		return fmt.Errorf("failed to create interface: %w", err)
	}

	// Set private key
	if err := ep.wgManager.SetInterfacePrivateKey(ep.interfaceName, ep.privateKey); err != nil {
		return fmt.Errorf("failed to set private key: %w", err)
	}

	// Set listen port
	if err := ep.wgManager.SetInterfaceListenPort(ep.interfaceName, ep.listenPort); err != nil {
		return fmt.Errorf("failed to set listen port: %w", err)
	}

	// Set interface IP
	if err := ep.wgManager.SetInterfaceIP(ep.interfaceName, "10.9.0.1/24"); err != nil {
		return fmt.Errorf("failed to set interface IP: %w", err)
	}

	ep.logger.WithField("interface", ep.interfaceName).Info("WireGuard interface initialized")
	return nil
}

// cleanupWireGuard cleans up the WireGuard interface
func (ep *ExitPeer) cleanupWireGuard() error {
	return ep.wgManager.DeleteInterface(ep.interfaceName)
}

// enableForwarding enables IP forwarding and NAT
func (ep *ExitPeer) enableForwarding() error {
	// Enable IP forwarding
	if err := utils.EnableIPForwarding(); err != nil {
		return fmt.Errorf("failed to enable IP forwarding: %w", err)
	}

	// Add NAT rule (assuming eth0 as external interface)
	if err := utils.AddNATRule(ep.interfaceName, "eth0"); err != nil {
		return fmt.Errorf("failed to add NAT rule: %w", err)
	}

	ep.logger.Info("IP forwarding and NAT enabled")
	return nil
}

// registerCommandHandlers registers custom command handlers for exit peer
func (ep *ExitPeer) registerCommandHandlers() {
	// Override the SETUP_EXIT handler
	ep.streamManager.RegisterCommandHandler(proto.CommandType_SETUP_EXIT, ep.handleSetupExit)
}

// handleSetupExit handles SETUP_EXIT commands from SuperNode
func (ep *ExitPeer) handleSetupExit(cmd *proto.Command) *proto.CommandResponse {
	clientID := cmd.Payload["client_id"]
	clientPubKey := cmd.Payload["client_pubkey"]
	sessionID := cmd.Payload["session_id"]
	allowedIPs := cmd.Payload["allowed_ips"]

	if clientID == "" || clientPubKey == "" || sessionID == "" {
		return &proto.CommandResponse{
			CommandId: cmd.CommandId,
			Success:   false,
			Message:   "Missing required parameters",
		}
	}

	if err := ep.addClient(clientID, clientPubKey, sessionID, allowedIPs); err != nil {
		ep.logger.WithError(err).Error("Failed to add client")
		return &proto.CommandResponse{
			CommandId: cmd.CommandId,
			Success:   false,
			Message:   fmt.Sprintf("Failed to add client: %v", err),
		}
	}

	// Get client info for response
	ep.clientsMux.RLock()
	clientInfo := ep.activeClients[clientID]
	ep.clientsMux.RUnlock()

	result := make(map[string]string)
	if clientInfo != nil {
		result["allocated_ip"] = clientInfo.AllocatedIP
		result["endpoint"] = fmt.Sprintf("0.0.0.0:%d", ep.listenPort) // Will be replaced with actual IP
		result["public_key"] = ep.privateKey.PublicKey().String()
	}

	return &proto.CommandResponse{
		CommandId: cmd.CommandId,
		Success:   true,
		Message:   "Client added successfully",
		Result:    result,
	}
}

// addClient adds a new client to the exit peer
func (ep *ExitPeer) addClient(clientID, clientPubKey, sessionID, allowedIPs string) error {
	ep.clientsMux.Lock()
	defer ep.clientsMux.Unlock()

	// Check if client already exists
	if _, exists := ep.activeClients[clientID]; exists {
		return fmt.Errorf("client %s already exists", clientID)
	}

	// Allocate IP for client
	allocatedIP, err := ep.ipAllocator.AllocateIP()
	if err != nil {
		return fmt.Errorf("failed to allocate IP: %w", err)
	}

	// Parse allowed IPs
	allowedIPList := []string{allowedIPs}
	if allowedIPs == "" {
		allowedIPList = []string{"0.0.0.0/0"} // Default to all traffic
	}

	// Add peer to WireGuard
	peerConfig := utils.PeerConfig{
		PublicKey:  clientPubKey,
		AllowedIPs: []string{fmt.Sprintf("%s/32", allocatedIP)},
	}

	if err := ep.wgManager.AddPeer(ep.interfaceName, peerConfig); err != nil {
		ep.ipAllocator.ReleaseIP(allocatedIP)
		return fmt.Errorf("failed to add peer to WireGuard: %w", err)
	}

	// Store client info
	clientInfo := &ClientInfo{
		ClientID:    clientID,
		PublicKey:   clientPubKey,
		AllocatedIP: allocatedIP,
		AllowedIPs:  allowedIPList,
		SessionID:   sessionID,
		SetupTime:   time.Now().Unix(),
	}

	ep.activeClients[clientID] = clientInfo

	ep.logger.WithFields(logrus.Fields{
		"client_id":    clientID,
		"allocated_ip": allocatedIP,
		"session_id":   sessionID,
	}).Info("Added client to exit peer")

	return nil
}

// removeClient removes a client from the exit peer
func (ep *ExitPeer) removeClient(clientID string) error {
	ep.clientsMux.Lock()
	defer ep.clientsMux.Unlock()
	return ep.removeClientUnsafe(clientID)
}

// removeClientUnsafe removes a client without locking (internal use)
func (ep *ExitPeer) removeClientUnsafe(clientID string) error {
	clientInfo, exists := ep.activeClients[clientID]
	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}

	// Remove peer from WireGuard
	if err := ep.wgManager.RemovePeer(ep.interfaceName, clientInfo.PublicKey); err != nil {
		return fmt.Errorf("failed to remove peer from WireGuard: %w", err)
	}

	// Release IP
	ep.ipAllocator.ReleaseIP(clientInfo.AllocatedIP)

	// Remove from active clients
	delete(ep.activeClients, clientID)

	ep.logger.WithFields(logrus.Fields{
		"client_id":    clientID,
		"allocated_ip": clientInfo.AllocatedIP,
	}).Info("Removed client from exit peer")

	return nil
}

// GetActiveClients returns a list of active clients
func (ep *ExitPeer) GetActiveClients() []*ClientInfo {
	ep.clientsMux.RLock()
	defer ep.clientsMux.RUnlock()

	var clients []*ClientInfo
	for _, client := range ep.activeClients {
		clients = append(clients, client)
	}
	return clients
}

// GetPublicKey returns the public key of this exit peer
func (ep *ExitPeer) GetPublicKey() string {
	return ep.privateKey.PublicKey().String()
}

// GetEndpoint returns the endpoint of this exit peer
func (ep *ExitPeer) GetEndpoint() string {
	return fmt.Sprintf("0.0.0.0:%d", ep.listenPort) // Should be replaced with actual external IP
}

// IsConnected returns the connection status to SuperNode
func (ep *ExitPeer) IsConnected() bool {
	return ep.streamManager.IsConnected()
}

// GetStats returns exit peer statistics
func (ep *ExitPeer) GetStats() map[string]interface{} {
	ep.clientsMux.RLock()
	defer ep.clientsMux.RUnlock()

	return map[string]interface{}{
		"peer_id":       ep.id,
		"region":        ep.region,
		"connected":     ep.streamManager.IsConnected(),
		"session_id":    ep.streamManager.GetSessionID(),
		"interface":     ep.interfaceName,
		"listen_port":   ep.listenPort,
		"public_key":    ep.privateKey.PublicKey().String(),
		"active_clients": len(ep.activeClients),
	}
}