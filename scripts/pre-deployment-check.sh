#!/bin/bash

# Pre-deployment verification script
# Run this before deploying to verify prerequisites

echo "🔍 myDvpn LAN Deployment - Pre-flight Check"
echo "==========================================="
echo ""

check_prerequisites() {
    echo "📋 Checking prerequisites..."
    
    # Check Go installation
    if command -v go &> /dev/null; then
        echo "   ✅ Go is installed: $(go version)"
    else
        echo "   ❌ Go is not installed"
        echo "      Install: wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz"
        echo "               sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz"
        echo "               echo 'export PATH=\$PATH:/usr/local/go/bin' >> ~/.bashrc"
    fi
    
    # Check WireGuard
    if command -v wg &> /dev/null; then
        echo "   ✅ WireGuard is installed: $(wg --version)"
    else
        echo "   ❌ WireGuard is not installed"
        echo "      Install: sudo apt update && sudo apt install wireguard-tools"
    fi
    
    # Check if project is built
    if [ -f "./bin/basenode" ] && [ -f "./bin/supernode" ] && [ -f "./bin/unified-client" ]; then
        echo "   ✅ myDvpn binaries are built"
    else
        echo "   ⚠️  myDvpn binaries not found"
        echo "      Run: ./scripts/build.sh"
    fi
    
    # Check network connectivity between computers
    echo ""
    echo "🌐 Testing network connectivity..."
    
    local computers=("192.168.1.104" "192.168.1.103" "192.168.1.101" "192.168.1.46" "192.168.1.40" "192.168.1.41")
    local current_ip=$(hostname -I | awk '{print $1}')
    
    echo "   Current computer IP: $current_ip"
    echo ""
    
    for ip in "${computers[@]}"; do
        if [ "$ip" != "$current_ip" ]; then
            if ping -c 1 -W 2 "$ip" &> /dev/null; then
                echo "   ✅ Can reach $ip"
            else
                echo "   ❌ Cannot reach $ip"
            fi
        else
            echo "   🏠 This computer: $ip"
        fi
    done
    
    echo ""
}

show_deployment_plan() {
    echo "📋 Deployment Plan"
    echo "=================="
    echo ""
    echo "Computer assignments (verify IPs are correct):"
    echo "   Computer 1: 192.168.1.104 - BaseNode India"
    echo "   Computer 2: 192.168.1.103 - BaseNode US"
    echo "   Computer 3: 192.168.1.101 - SuperNode India"
    echo "   Computer 4: 192.168.1.46 - SuperNode US"
    echo "   Computer 5: 192.168.1.40 - Client India"
    echo "   Computer 6: 192.168.1.41 - Client US"
    echo ""
    echo "Deployment order:"
    echo "   1. Deploy BaseNodes first (Computer 1 & 2)"
    echo "   2. Deploy SuperNodes (Computer 3 & 4)"
    echo "   3. Deploy Clients (Computer 5 & 6)"
    echo "   4. Run test suite"
    echo ""
}

show_quick_start() {
    echo "🚀 Quick Start Guide"
    echo "==================="
    echo ""
    echo "On each computer, run:"
    echo "   1. Copy myDvpn project"
    echo "   2. cd myDvpn"
    echo "   3. ./scripts/setup-computer.sh"
    echo "   4. Select the appropriate computer number"
    echo ""
    echo "After all computers are configured:"
    echo "   ./scripts/test-lan-deployment.sh"
    echo ""
}

show_troubleshooting() {
    echo "🔧 Troubleshooting"
    echo "=================="
    echo ""
    echo "Common issues:"
    echo "   • Permission denied: Run scripts as regular user (not root)"
    echo "   • WireGuard errors: Ensure kernel module loaded (sudo modprobe wireguard)"
    echo "   • Connection failures: Check firewall (sudo ufw status)"
    echo "   • Service failures: Check logs (sudo journalctl -u mydvpn-*)"
    echo ""
    echo "Firewall rules needed:"
    echo "   sudo ufw allow 50051/tcp  # BaseNode"
    echo "   sudo ufw allow 50052/tcp  # SuperNode gRPC"
    echo "   sudo ufw allow 51820:51825/udp  # WireGuard"
    echo "   sudo ufw allow 8080/tcp   # Metrics"
    echo ""
}

# Main menu
while true; do
    echo "Choose an option:"
    echo "1) Check prerequisites"
    echo "2) Show deployment plan"
    echo "3) Quick start guide"
    echo "4) Troubleshooting tips"
    echo "5) Exit"
    echo ""
    
    read -p "Enter choice (1-5): " choice
    
    case $choice in
        1) check_prerequisites ;;
        2) show_deployment_plan ;;
        3) show_quick_start ;;
        4) show_troubleshooting ;;
        5) echo "👋 Good luck with your deployment!"; exit 0 ;;
        *) echo "❌ Invalid choice" ;;
    esac
    
    echo ""
    read -p "Press Enter to continue..."
    echo ""
done