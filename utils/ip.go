package utils

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

// AllocateClientIP allocates an IP address for a client in the given CIDR range
func AllocateClientIP(cidr string, usedIPs map[string]bool) (string, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR: %w", err)
	}

	// Start from the second IP in the range (first is usually gateway)
	ip := ipNet.IP
	for ip := ip.Mask(ipNet.Mask); ipNet.Contains(ip); incIP(ip) {
		// Skip network and broadcast addresses
		if ip.Equal(ipNet.IP) || ip.Equal(getBroadcast(ipNet)) {
			continue
		}

		ipStr := ip.String()
		if !usedIPs[ipStr] {
			return ipStr, nil
		}
	}

	return "", fmt.Errorf("no available IP addresses in CIDR %s", cidr)
}

// incIP increments an IP address
func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// getBroadcast gets the broadcast address for a network
func getBroadcast(ipNet *net.IPNet) net.IP {
	ip := make(net.IP, len(ipNet.IP))
	copy(ip, ipNet.IP)
	
	for i := 0; i < len(ip); i++ {
		ip[i] |= ^ipNet.Mask[i]
	}
	
	return ip
}

// ValidateIP validates an IP address string
func ValidateIP(ip string) error {
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}
	return nil
}

// ValidateCIDR validates a CIDR string
func ValidateCIDR(cidr string) error {
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %w", err)
	}
	return nil
}

// ParseEndpoint parses an endpoint string (IP:port) and validates it
func ParseEndpoint(endpoint string) (string, int, error) {
	parts := strings.Split(endpoint, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid endpoint format, expected IP:port")
	}

	ip := parts[0]
	if err := ValidateIP(ip); err != nil {
		return "", 0, err
	}

	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid port: %w", err)
	}

	if port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("port out of range: %d", port)
	}

	return ip, port, nil
}

// IsPrivateIP checks if an IP address is private
func IsPrivateIP(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// Check for private IP ranges
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12", 
		"192.168.0.0/16",
	}

	for _, cidr := range privateRanges {
		_, ipNet, _ := net.ParseCIDR(cidr)
		if ipNet.Contains(parsedIP) {
			return true
		}
	}

	return false
}

// EnableIPForwarding enables IP forwarding on the system
func EnableIPForwarding() error {
	cmd := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1")
	return cmd.Run()
}

// AddNATRule adds a NAT rule for the specified interfaces
func AddNATRule(internalInterface, externalInterface string) error {
	cmd := exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", 
		"-o", externalInterface, "-j", "MASQUERADE")
	return cmd.Run()
}