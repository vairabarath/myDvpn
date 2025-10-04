package dataplane

import (
	"fmt"
	"sync"

	"myDvpn/utils"
	"github.com/sirupsen/logrus"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// WireGuardDataplane manages WireGuard interfaces and routing
type WireGuardDataplane struct {
	interfaceName string
	listenPort    int
	privateKey    wgtypes.Key
	wgManager     *utils.WireGuardManager
	activePeers   map[string]*PeerInfo
	peersMux      sync.RWMutex
	logger        *logrus.Logger
	ipAllocator   *IPAllocator
}

// PeerInfo contains information about an active peer
type PeerInfo struct {
	PeerID     string
	PublicKey  string
	ClientID   string
	AllocatedIP string
	AllowedIPs []string
	SessionID  string
}

// IPAllocator manages IP address allocation
type IPAllocator struct {
	cidr    string
	usedIPs map[string]bool
	mutex   sync.Mutex
}

// NewIPAllocator creates a new IP allocator
func NewIPAllocator(cidr string) *IPAllocator {
	return &IPAllocator{
		cidr:    cidr,
		usedIPs: make(map[string]bool),
	}
}

// AllocateIP allocates a new IP address
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

// NewWireGuardDataplane creates a new WireGuard dataplane
func NewWireGuardDataplane(interfaceName string, listenPort int, logger *logrus.Logger) (*WireGuardDataplane, error) {
	wgManager, err := utils.NewWireGuardManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create WireGuard manager: %w", err)
	}

	// Generate private key
	privateKey, err := utils.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	return &WireGuardDataplane{
		interfaceName: interfaceName,
		listenPort:    listenPort,
		privateKey:    privateKey,
		wgManager:     wgManager,
		activePeers:   make(map[string]*PeerInfo),
		logger:        logger,
		ipAllocator:   NewIPAllocator("10.8.0.0/24"), // Default relay network
	}, nil
}

// Initialize initializes the WireGuard interface
func (wd *WireGuardDataplane) Initialize() error {
	// Create WireGuard interface
	if err := wd.wgManager.CreateInterface(wd.interfaceName); err != nil {
		return fmt.Errorf("failed to create interface: %w", err)
	}

	// Set private key
	if err := wd.wgManager.SetInterfacePrivateKey(wd.interfaceName, wd.privateKey); err != nil {
		return fmt.Errorf("failed to set private key: %w", err)
	}

	// Set listen port
	if err := wd.wgManager.SetInterfaceListenPort(wd.interfaceName, wd.listenPort); err != nil {
		return fmt.Errorf("failed to set listen port: %w", err)
	}

	// Set interface IP
	if err := wd.wgManager.SetInterfaceIP(wd.interfaceName, "10.8.0.1/24"); err != nil {
		return fmt.Errorf("failed to set interface IP: %w", err)
	}

	wd.logger.WithFields(logrus.Fields{
		"interface": wd.interfaceName,
		"port":      wd.listenPort,
		"public_key": wd.privateKey.PublicKey().String(),
	}).Info("WireGuard dataplane initialized")

	return nil
}

// SetupRelayForClient sets up relay forwarding for a client
func (wd *WireGuardDataplane) SetupRelayForClient(clientID, clientPublicKey, sessionID string) (*PeerInfo, error) {
	wd.peersMux.Lock()
	defer wd.peersMux.Unlock()

	// Check if client already has a relay
	for _, peer := range wd.activePeers {
		if peer.ClientID == clientID {
			return peer, nil // Return existing relay
		}
	}

	// Allocate IP for client
	allocatedIP, err := wd.ipAllocator.AllocateIP()
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP: %w", err)
	}

	// Add peer to WireGuard interface
	peerConfig := utils.PeerConfig{
		PublicKey:  clientPublicKey,
		AllowedIPs: []string{fmt.Sprintf("%s/32", allocatedIP)},
	}

	if err := wd.wgManager.AddPeer(wd.interfaceName, peerConfig); err != nil {
		wd.ipAllocator.ReleaseIP(allocatedIP)
		return nil, fmt.Errorf("failed to add peer to WireGuard: %w", err)
	}

	peerInfo := &PeerInfo{
		PeerID:      fmt.Sprintf("relay-%s", clientID),
		PublicKey:   clientPublicKey,
		ClientID:    clientID,
		AllocatedIP: allocatedIP,
		AllowedIPs:  []string{fmt.Sprintf("%s/32", allocatedIP)},
		SessionID:   sessionID,
	}

	wd.activePeers[peerInfo.PeerID] = peerInfo

	wd.logger.WithFields(logrus.Fields{
		"client_id":     clientID,
		"allocated_ip":  allocatedIP,
		"session_id":    sessionID,
		"client_pubkey": clientPublicKey,
	}).Info("Set up relay for client")

	return peerInfo, nil
}

// RemoveRelay removes relay forwarding for a client
func (wd *WireGuardDataplane) RemoveRelay(clientID string) error {
	wd.peersMux.Lock()
	defer wd.peersMux.Unlock()

	var peerToRemove *PeerInfo
	for _, peer := range wd.activePeers {
		if peer.ClientID == clientID {
			peerToRemove = peer
			break
		}
	}

	if peerToRemove == nil {
		return fmt.Errorf("no relay found for client %s", clientID)
	}

	// Remove peer from WireGuard
	if err := wd.wgManager.RemovePeer(wd.interfaceName, peerToRemove.PublicKey); err != nil {
		return fmt.Errorf("failed to remove peer from WireGuard: %w", err)
	}

	// Release IP
	wd.ipAllocator.ReleaseIP(peerToRemove.AllocatedIP)

	// Remove from active peers
	delete(wd.activePeers, peerToRemove.PeerID)

	wd.logger.WithFields(logrus.Fields{
		"client_id":    clientID,
		"allocated_ip": peerToRemove.AllocatedIP,
	}).Info("Removed relay for client")

	return nil
}

// GetPublicKey returns the public key of this interface
func (wd *WireGuardDataplane) GetPublicKey() string {
	return wd.privateKey.PublicKey().String()
}

// GetActivePeers returns a list of active peers
func (wd *WireGuardDataplane) GetActivePeers() []*PeerInfo {
	wd.peersMux.RLock()
	defer wd.peersMux.RUnlock()

	var peers []*PeerInfo
	for _, peer := range wd.activePeers {
		peers = append(peers, peer)
	}
	return peers
}

// Cleanup cleans up the WireGuard interface
func (wd *WireGuardDataplane) Cleanup() error {
	// Remove all peers first
	wd.peersMux.Lock()
	for clientID := range wd.activePeers {
		// Release IPs
		if peer := wd.activePeers[clientID]; peer != nil {
			wd.ipAllocator.ReleaseIP(peer.AllocatedIP)
		}
	}
	wd.activePeers = make(map[string]*PeerInfo)
	wd.peersMux.Unlock()

	// Delete interface
	if err := wd.wgManager.DeleteInterface(wd.interfaceName); err != nil {
		wd.logger.WithError(err).Warn("Failed to delete WireGuard interface")
	}

	// Close WireGuard manager
	if err := wd.wgManager.Close(); err != nil {
		wd.logger.WithError(err).Warn("Failed to close WireGuard manager")
	}

	wd.logger.WithField("interface", wd.interfaceName).Info("WireGuard dataplane cleaned up")
	return nil
}