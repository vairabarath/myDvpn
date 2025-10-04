package client

import (
	"fmt"
	"sync"

	"myDvpn/utils"
	"github.com/sirupsen/logrus"
)

// Peer represents a client peer
type Peer struct {
	id              string
	region          string
	supernodeAddr   string
	logger          *logrus.Logger
	
	streamManager   *PersistentStreamManager
	wgManager       *utils.WireGuardManager
	
	// WireGuard configuration
	interfaceName   string
	privateKey      string
	currentExit     *ExitConfig
	
	mutex           sync.RWMutex
}

// ExitConfig represents the current exit configuration
type ExitConfig struct {
	ExitPeerID    string
	PublicKey     string
	Endpoint      string
	AllowedIPs    []string
	SessionID     string
}

// NewPeer creates a new client peer
func NewPeer(id, region, supernodeAddr string, logger *logrus.Logger) (*Peer, error) {
	// Create persistent stream manager
	streamManager, err := NewPersistentStreamManager(id, "client", region, supernodeAddr, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream manager: %w", err)
	}

	// Create WireGuard manager
	wgManager, err := utils.NewWireGuardManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create WireGuard manager: %w", err)
	}

	return &Peer{
		id:            id,
		region:        region,
		supernodeAddr: supernodeAddr,
		logger:        logger,
		streamManager: streamManager,
		wgManager:     wgManager,
		interfaceName: fmt.Sprintf("wg-client-%s", id),
	}, nil
}

// Start starts the client peer
func (p *Peer) Start() error {
	// Start persistent stream
	if err := p.streamManager.Start(); err != nil {
		return fmt.Errorf("failed to start stream manager: %w", err)
	}

	// Initialize WireGuard interface
	if err := p.initializeWireGuard(); err != nil {
		return fmt.Errorf("failed to initialize WireGuard: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"peer_id": p.id,
		"region":  p.region,
	}).Info("Client peer started")

	return nil
}

// Stop stops the client peer
func (p *Peer) Stop() error {
	// Stop stream manager
	p.streamManager.Stop()

	// Cleanup WireGuard
	if err := p.cleanupWireGuard(); err != nil {
		p.logger.WithError(err).Warn("Failed to cleanup WireGuard interface")
	}

	// Close WireGuard manager
	if err := p.wgManager.Close(); err != nil {
		p.logger.WithError(err).Warn("Failed to close WireGuard manager")
	}

	p.logger.WithField("peer_id", p.id).Info("Client peer stopped")
	return nil
}

// initializeWireGuard initializes the WireGuard interface
func (p *Peer) initializeWireGuard() error {
	// Generate private key
	privateKey, err := utils.GenerateKey()
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}
	p.privateKey = privateKey.String()

	// Create interface
	if err := p.wgManager.CreateInterface(p.interfaceName); err != nil {
		return fmt.Errorf("failed to create interface: %w", err)
	}

	// Set private key
	if err := p.wgManager.SetInterfacePrivateKey(p.interfaceName, privateKey); err != nil {
		return fmt.Errorf("failed to set private key: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"interface":   p.interfaceName,
		"public_key":  privateKey.PublicKey().String(),
	}).Info("WireGuard interface initialized")

	return nil
}

// cleanupWireGuard cleans up the WireGuard interface
func (p *Peer) cleanupWireGuard() error {
	return p.wgManager.DeleteInterface(p.interfaceName)
}

// RequestExit requests an exit peer from the SuperNode
func (p *Peer) RequestExit(targetRegion string) (*ExitConfig, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// TODO: Implement exit request logic
	// This would involve sending a request to the SuperNode for an exit peer
	// For now, this is a placeholder

	p.logger.WithFields(logrus.Fields{
		"peer_id":       p.id,
		"target_region": targetRegion,
	}).Info("Requesting exit peer")

	// Placeholder exit config
	exitConfig := &ExitConfig{
		ExitPeerID: "exit-placeholder",
		PublicKey:  "placeholder-key",
		Endpoint:   "127.0.0.1:51820",
		AllowedIPs: []string{"0.0.0.0/0"},
		SessionID:  "session-placeholder",
	}

	p.currentExit = exitConfig
	return exitConfig, nil
}

// ConnectToExit connects to an exit peer using WireGuard
func (p *Peer) ConnectToExit(config *ExitConfig) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if config == nil {
		return fmt.Errorf("exit config is nil")
	}

	// Remove existing peer if any
	if p.currentExit != nil {
		if err := p.wgManager.RemovePeer(p.interfaceName, p.currentExit.PublicKey); err != nil {
			p.logger.WithError(err).Warn("Failed to remove existing peer")
		}
	}

	// Add new peer
	peerConfig := utils.PeerConfig{
		PublicKey:  config.PublicKey,
		Endpoint:   config.Endpoint,
		AllowedIPs: config.AllowedIPs,
	}

	if err := p.wgManager.AddPeer(p.interfaceName, peerConfig); err != nil {
		return fmt.Errorf("failed to add peer: %w", err)
	}

	// Set interface IP (typically allocated by the exit peer)
	// For now, use a default IP
	if err := p.wgManager.SetInterfaceIP(p.interfaceName, "10.8.0.2/24"); err != nil {
		return fmt.Errorf("failed to set interface IP: %w", err)
	}

	p.currentExit = config

	p.logger.WithFields(logrus.Fields{
		"peer_id":     p.id,
		"exit_peer":   config.ExitPeerID,
		"endpoint":    config.Endpoint,
		"session_id":  config.SessionID,
	}).Info("Connected to exit peer")

	return nil
}

// DisconnectFromExit disconnects from the current exit peer
func (p *Peer) DisconnectFromExit() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.currentExit == nil {
		return fmt.Errorf("not connected to any exit peer")
	}

	// Remove peer
	if err := p.wgManager.RemovePeer(p.interfaceName, p.currentExit.PublicKey); err != nil {
		return fmt.Errorf("failed to remove peer: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"peer_id":    p.id,
		"exit_peer":  p.currentExit.ExitPeerID,
		"session_id": p.currentExit.SessionID,
	}).Info("Disconnected from exit peer")

	p.currentExit = nil
	return nil
}

// GetCurrentExit returns the current exit configuration
func (p *Peer) GetCurrentExit() *ExitConfig {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.currentExit
}

// GetPublicKey returns the public key of this peer
func (p *Peer) GetPublicKey() (string, error) {
	if p.privateKey == "" {
		return "", fmt.Errorf("private key not initialized")
	}

	// Parse private key to get public key
	privateKey, err := utils.GenerateKey() // This is a placeholder
	if err != nil {
		return "", fmt.Errorf("failed to get public key: %w", err)
	}

	return privateKey.PublicKey().String(), nil
}

// IsConnected returns the connection status
func (p *Peer) IsConnected() bool {
	return p.streamManager.IsConnected()
}

// GetSessionID returns the current session ID
func (p *Peer) GetSessionID() string {
	return p.streamManager.GetSessionID()
}

// GetStats returns peer statistics
func (p *Peer) GetStats() map[string]interface{} {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	stats := map[string]interface{}{
		"peer_id":     p.id,
		"region":      p.region,
		"connected":   p.streamManager.IsConnected(),
		"session_id":  p.streamManager.GetSessionID(),
		"interface":   p.interfaceName,
	}

	if p.currentExit != nil {
		stats["current_exit"] = map[string]interface{}{
			"exit_peer_id": p.currentExit.ExitPeerID,
			"endpoint":     p.currentExit.Endpoint,
			"session_id":   p.currentExit.SessionID,
		}
	}

	return stats
}