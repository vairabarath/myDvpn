package dataplane

import (
	"fmt"
	"os/exec"
	"sync"

	"github.com/sirupsen/logrus"
)

// RelayManager manages NAT and forwarding rules for relay traffic
type RelayManager struct {
	logger            *logrus.Logger
	activeRules       map[string]*RelayRule
	rulesMux          sync.RWMutex
	externalInterface string
}

// RelayRule represents a forwarding rule
type RelayRule struct {
	ClientID  string
	ClientIP  string
	ExitIP    string
	ExitPort  int
	LocalPort int
	SessionID string
}

// NewRelayManager creates a new relay manager
func NewRelayManager(logger *logrus.Logger, externalInterface string) *RelayManager {
	return &RelayManager{
		logger:            logger,
		activeRules:       make(map[string]*RelayRule),
		externalInterface: externalInterface,
	}
}

// SetupRelay sets up NAT and forwarding rules for relaying traffic
func (rm *RelayManager) SetupRelay(rule *RelayRule) error {
	rm.rulesMux.Lock()
	defer rm.rulesMux.Unlock()

	// Check if rule already exists
	if _, exists := rm.activeRules[rule.ClientID]; exists {
		rm.logger.WithField("client_id", rule.ClientID).Info("Relay rule already exists")
		return nil
	}

	// Enable IP forwarding
	if err := rm.enableIPForwarding(); err != nil {
		return fmt.Errorf("failed to enable IP forwarding: %w", err)
	}

	// Set up MASQUERADE rule for outbound traffic
	if err := rm.addMasqueradeRule(rule.ClientIP); err != nil {
		return fmt.Errorf("failed to add masquerade rule: %w", err)
	}

	// Set up forwarding rules if needed for specific exit
	if rule.ExitIP != "" && rule.ExitPort > 0 {
		if err := rm.addPortForwardRule(rule); err != nil {
			rm.removeMasqueradeRule(rule.ClientIP) // Cleanup on failure
			return fmt.Errorf("failed to add port forward rule: %w", err)
		}
	}

	rm.activeRules[rule.ClientID] = rule

	rm.logger.WithFields(logrus.Fields{
		"client_id":  rule.ClientID,
		"client_ip":  rule.ClientIP,
		"exit_ip":    rule.ExitIP,
		"exit_port":  rule.ExitPort,
		"local_port": rule.LocalPort,
		"session_id": rule.SessionID,
	}).Info("Set up relay rule")

	return nil
}

// RemoveRelay removes NAT and forwarding rules for a client
func (rm *RelayManager) RemoveRelay(clientID string) error {
	rm.rulesMux.Lock()
	defer rm.rulesMux.Unlock()

	rule, exists := rm.activeRules[clientID]
	if !exists {
		return fmt.Errorf("no relay rule found for client %s", clientID)
	}

	// Remove masquerade rule
	if err := rm.removeMasqueradeRule(rule.ClientIP); err != nil {
		rm.logger.WithError(err).Warn("Failed to remove masquerade rule")
	}

	// Remove port forward rule if it exists
	if rule.ExitIP != "" && rule.ExitPort > 0 {
		if err := rm.removePortForwardRule(rule); err != nil {
			rm.logger.WithError(err).Warn("Failed to remove port forward rule")
		}
	}

	delete(rm.activeRules, clientID)

	rm.logger.WithField("client_id", clientID).Info("Removed relay rule")
	return nil
}

// enableIPForwarding enables IP forwarding on the system
func (rm *RelayManager) enableIPForwarding() error {
	cmd := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable IP forwarding: %w", err)
	}
	return nil
}

// addMasqueradeRule adds a MASQUERADE rule for outbound traffic
func (rm *RelayManager) addMasqueradeRule(clientIP string) error {
	// Add MASQUERADE rule for traffic from client IP
	cmd := exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING",
		"-s", fmt.Sprintf("%s/32", clientIP),
		"-o", rm.externalInterface,
		"-j", "MASQUERADE")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add MASQUERADE rule: %w", err)
	}

	// Allow forwarding for this client
	cmd = exec.Command("iptables", "-A", "FORWARD",
		"-s", fmt.Sprintf("%s/32", clientIP),
		"-j", "ACCEPT")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add FORWARD rule: %w", err)
	}

	return nil
}

// removeMasqueradeRule removes a MASQUERADE rule
func (rm *RelayManager) removeMasqueradeRule(clientIP string) error {
	// Remove MASQUERADE rule
	cmd := exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING",
		"-s", fmt.Sprintf("%s/32", clientIP),
		"-o", rm.externalInterface,
		"-j", "MASQUERADE")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove MASQUERADE rule: %w", err)
	}

	// Remove FORWARD rule
	cmd = exec.Command("iptables", "-D", "FORWARD",
		"-s", fmt.Sprintf("%s/32", clientIP),
		"-j", "ACCEPT")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove FORWARD rule: %w", err)
	}

	return nil
}

// addPortForwardRule adds a port forwarding rule
func (rm *RelayManager) addPortForwardRule(rule *RelayRule) error {
	// Add DNAT rule for incoming traffic
	cmd := exec.Command("iptables", "-t", "nat", "-A", "PREROUTING",
		"-p", "udp",
		"--dport", fmt.Sprintf("%d", rule.LocalPort),
		"-j", "DNAT",
		"--to-destination", fmt.Sprintf("%s:%d", rule.ExitIP, rule.ExitPort))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add DNAT rule: %w", err)
	}

	// Add FORWARD rule for the forwarded traffic
	cmd = exec.Command("iptables", "-A", "FORWARD",
		"-p", "udp",
		"-d", rule.ExitIP,
		"--dport", fmt.Sprintf("%d", rule.ExitPort),
		"-j", "ACCEPT")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add FORWARD rule for port forwarding: %w", err)
	}

	return nil
}

// removePortForwardRule removes a port forwarding rule
func (rm *RelayManager) removePortForwardRule(rule *RelayRule) error {
	// Remove DNAT rule
	cmd := exec.Command("iptables", "-t", "nat", "-D", "PREROUTING",
		"-p", "udp",
		"--dport", fmt.Sprintf("%d", rule.LocalPort),
		"-j", "DNAT",
		"--to-destination", fmt.Sprintf("%s:%d", rule.ExitIP, rule.ExitPort))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove DNAT rule: %w", err)
	}

	// Remove FORWARD rule
	cmd = exec.Command("iptables", "-D", "FORWARD",
		"-p", "udp",
		"-d", rule.ExitIP,
		"--dport", fmt.Sprintf("%d", rule.ExitPort),
		"-j", "ACCEPT")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove FORWARD rule for port forwarding: %w", err)
	}

	return nil
}

// GetActiveRules returns all active relay rules
func (rm *RelayManager) GetActiveRules() []*RelayRule {
	rm.rulesMux.RLock()
	defer rm.rulesMux.RUnlock()

	var rules []*RelayRule
	for _, rule := range rm.activeRules {
		rules = append(rules, rule)
	}
	return rules
}

// Cleanup removes all relay rules
func (rm *RelayManager) Cleanup() error {
	rm.rulesMux.Lock()
	defer rm.rulesMux.Unlock()

	for clientID := range rm.activeRules {
		if err := rm.RemoveRelay(clientID); err != nil {
			rm.logger.WithError(err).WithField("client_id", clientID).Warn("Failed to remove relay rule during cleanup")
		}
	}

	rm.activeRules = make(map[string]*RelayRule)
	rm.logger.Info("Relay manager cleaned up")
	return nil
}
