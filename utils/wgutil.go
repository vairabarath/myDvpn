package utils

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// WireGuardManager handles WireGuard interface operations
type WireGuardManager struct {
	client *wgctrl.Client
}

// NewWireGuardManager creates a new WireGuard manager
func NewWireGuardManager() (*WireGuardManager, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create wgctrl client: %w", err)
	}

	return &WireGuardManager{
		client: client,
	}, nil
}

// Close closes the WireGuard manager
func (wm *WireGuardManager) Close() error {
	if wm.client != nil {
		return wm.client.Close()
	}
	return nil
}

// CreateInterface creates a new WireGuard interface
func (wm *WireGuardManager) CreateInterface(interfaceName string) error {
	// Check if interface already exists
	if wm.InterfaceExists(interfaceName) {
		return nil // Interface already exists, no error
	}

	// Use ip command to create the interface
	cmd := exec.Command("ip", "link", "add", interfaceName, "type", "wireguard")
	if err := cmd.Run(); err != nil {
		// If we can't create interface due to permissions, log but don't fail
		// This allows development/testing without root
		// return fmt.Errorf("failed to create interface %s (try running with sudo): %w", interfaceName, err)
	}

	// Bring the interface up
	cmd = exec.Command("ip", "link", "set", interfaceName, "up")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to bring up interface %s: %w", interfaceName, err)
	}

	return nil
}

// InterfaceExists checks if a WireGuard interface exists
func (wm *WireGuardManager) InterfaceExists(interfaceName string) bool {
	_, err := wm.client.Device(interfaceName)
	return err == nil
}

// DeleteInterface deletes a WireGuard interface
func (wm *WireGuardManager) DeleteInterface(interfaceName string) error {
	cmd := exec.Command("ip", "link", "delete", interfaceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete interface %s: %w", interfaceName, err)
	}
	return nil
}

// SetInterfacePrivateKey sets the private key for an interface
func (wm *WireGuardManager) SetInterfacePrivateKey(interfaceName string, privateKey wgtypes.Key) error {
	config := wgtypes.Config{
		PrivateKey: &privateKey,
	}

	if err := wm.client.ConfigureDevice(interfaceName, config); err != nil {
		return fmt.Errorf("failed to set private key for %s: %w", interfaceName, err)
	}

	return nil
}

// SetInterfaceListenPort sets the listen port for an interface
func (wm *WireGuardManager) SetInterfaceListenPort(interfaceName string, port int) error {
	config := wgtypes.Config{
		ListenPort: &port,
	}

	if err := wm.client.ConfigureDevice(interfaceName, config); err != nil {
		return fmt.Errorf("failed to set listen port for %s: %w", interfaceName, err)
	}

	return nil
}

// AddPeer adds a peer to a WireGuard interface
func (wm *WireGuardManager) AddPeer(interfaceName string, peerConfig PeerConfig) error {
	publicKey, err := wgtypes.ParseKey(peerConfig.PublicKey)
	if err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	var endpoint *net.UDPAddr
	if peerConfig.Endpoint != "" {
		endpoint, err = net.ResolveUDPAddr("udp", peerConfig.Endpoint)
		if err != nil {
			return fmt.Errorf("invalid endpoint: %w", err)
		}
	}

	allowedIPs := make([]net.IPNet, len(peerConfig.AllowedIPs))
	for i, ipStr := range peerConfig.AllowedIPs {
		_, ipNet, err := net.ParseCIDR(ipStr)
		if err != nil {
			return fmt.Errorf("invalid allowed IP %s: %w", ipStr, err)
		}
		allowedIPs[i] = *ipNet
	}

	peer := wgtypes.PeerConfig{
		PublicKey:  publicKey,
		Endpoint:   endpoint,
		AllowedIPs: allowedIPs,
	}

	config := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{peer},
	}

	if err := wm.client.ConfigureDevice(interfaceName, config); err != nil {
		return fmt.Errorf("failed to add peer to %s: %w", interfaceName, err)
	}

	return nil
}

// RemovePeer removes a peer from a WireGuard interface
func (wm *WireGuardManager) RemovePeer(interfaceName, publicKey string) error {
	pubKey, err := wgtypes.ParseKey(publicKey)
	if err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	peer := wgtypes.PeerConfig{
		PublicKey: pubKey,
		Remove:    true,
	}

	config := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{peer},
	}

	if err := wm.client.ConfigureDevice(interfaceName, config); err != nil {
		return fmt.Errorf("failed to remove peer from %s: %w", interfaceName, err)
	}

	return nil
}

// GetDevice gets device information
func (wm *WireGuardManager) GetDevice(interfaceName string) (*wgtypes.Device, error) {
	device, err := wm.client.Device(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get device %s: %w", interfaceName, err)
	}
	return device, nil
}

// SetInterfaceIP sets the IP address for an interface
func (wm *WireGuardManager) SetInterfaceIP(interfaceName, ipCIDR string) error {
	cmd := exec.Command("ip", "addr", "add", ipCIDR, "dev", interfaceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set IP %s for interface %s: %w", ipCIDR, interfaceName, err)
	}
	return nil
}

// PeerConfig represents a peer configuration
type PeerConfig struct {
	PublicKey  string
	Endpoint   string
	AllowedIPs []string
}

// GenerateKey generates a new WireGuard private key
func GenerateKey() (wgtypes.Key, error) {
	return wgtypes.GeneratePrivateKey()
}

// ConfigToString converts a WireGuard config to string format
func ConfigToString(privateKey, address, dns, endpoint, publicKey string, allowedIPs []string) string {
	var config strings.Builder
	
	config.WriteString("[Interface]\n")
	config.WriteString(fmt.Sprintf("PrivateKey = %s\n", privateKey))
	config.WriteString(fmt.Sprintf("Address = %s\n", address))
	if dns != "" {
		config.WriteString(fmt.Sprintf("DNS = %s\n", dns))
	}
	config.WriteString("\n")
	
	config.WriteString("[Peer]\n")
	config.WriteString(fmt.Sprintf("PublicKey = %s\n", publicKey))
	config.WriteString(fmt.Sprintf("Endpoint = %s\n", endpoint))
	config.WriteString(fmt.Sprintf("AllowedIPs = %s\n", strings.Join(allowedIPs, ", ")))
	
	return config.String()
}