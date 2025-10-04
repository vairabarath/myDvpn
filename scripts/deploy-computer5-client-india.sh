#!/bin/bash

# Computer 5: Unified Client (India) - 192.168.1.40
# This script sets up the India client with exit mode capability

set -e

echo "👤 Setting up Unified Client (India) on Computer 5"
echo "================================================="
echo "IP: 192.168.1.40"
echo "Role: End-user client with exit mode capability"
echo ""

# Check if running as root
if [[ $EUID -eq 0 ]]; then
   echo "❌ This script should not be run as root"
   echo "   Run as regular user, sudo will be used when needed"
   exit 1
fi

# Verify myDvpn is built
if [ ! -f "./bin/unified-client" ]; then
    echo "📦 Building myDvpn..."
    ./scripts/build.sh
fi

# Check WireGuard is installed
if ! command -v wg &> /dev/null; then
    echo "📦 Installing WireGuard..."
    sudo apt update
    sudo apt install -y wireguard-tools
fi

# Create configuration directory
sudo mkdir -p /etc/mydvpn
sudo chown $USER:$USER /etc/mydvpn

# Create unified client configuration
cat > /etc/mydvpn/client-india.conf << EOF
# Unified Client India Configuration
id=client-india
region=india
supernode_addr=192.168.1.101:50052
exit_port=51822
log_level=info
EOF

# Create systemd service
sudo tee /etc/systemd/system/mydvpn-client-india.service > /dev/null << EOF
[Unit]
Description=myDvpn Unified Client (India)
After=network.target
Documentation=https://github.com/mydvpn/docs
Requires=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$PWD
ExecStart=$PWD/bin/unified-client \\
    --id=client-india \\
    --region=india \\
    --supernode=192.168.1.101:50052 \\
    --exit-port=51822 \\
    --log-level=info \\
    --no-ui
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=mydvpn-client-india

# Capabilities for WireGuard
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW

[Install]
WantedBy=multi-user.target
EOF

# Setup firewall rules
echo "🔥 Configuring firewall..."
sudo ufw allow 51822/udp comment "myDvpn Client India WireGuard (exit mode)"

# Enable IP forwarding for exit mode
echo "🌐 Enabling IP forwarding..."
echo 'net.ipv4.ip_forward=1' | sudo tee -a /etc/sysctl.conf
sudo sysctl -p

# Create interactive script for testing
cat > test-client-india.sh << 'EOF'
#!/bin/bash

echo "🇮🇳 myDvpn Unified Client (India) - Interactive Test"
echo "=================================================="

# Function to run unified client interactively
run_interactive() {
    echo "🚀 Starting interactive unified client..."
    echo ""
    echo "Available commands when client starts:"
    echo "  status          - Show current status"
    echo "  toggle-exit on  - Enable exit mode (provide VPN to others)"
    echo "  connect us      - Connect to US exit peer"
    echo "  clients         - Show connected clients (when in exit mode)"
    echo "  disconnect      - Disconnect from current exit"
    echo "  quit            - Exit"
    echo ""
    echo "Press Enter to start..."
    read
    
    sudo ./bin/unified-client \
        --id=client-india-interactive \
        --region=india \
        --supernode=192.168.1.101:50052 \
        --exit-port=51822 \
        --log-level=info
}

# Function to run as service
run_service() {
    echo "🛠️  Service management commands:"
    echo ""
    echo "Start service:    sudo systemctl start mydvpn-client-india"
    echo "Stop service:     sudo systemctl stop mydvpn-client-india"
    echo "Restart service:  sudo systemctl restart mydvpn-client-india"
    echo "View logs:        sudo journalctl -u mydvpn-client-india -f"
    echo "Service status:   sudo systemctl status mydvpn-client-india"
    echo ""
}

echo "Choose testing mode:"
echo "1) Interactive mode (manual commands)"
echo "2) Service mode (background daemon)"
echo "3) Show WireGuard status"
echo "4) Show system status"
echo ""
read -p "Enter choice (1-4): " choice

case $choice in
    1)
        run_interactive
        ;;
    2)
        run_service
        ;;
    3)
        echo "🔌 WireGuard interfaces:"
        sudo wg show
        echo ""
        echo "🌐 Network interfaces:"
        ip addr show | grep -E "(wg-|inet )"
        ;;
    4)
        echo "📊 System Status:"
        echo ""
        echo "SuperNode India connectivity:"
        nc -z 192.168.1.101 50052 && echo "   ✅ Connected" || echo "   ❌ Not connected"
        echo ""
        echo "Service status:"
        sudo systemctl is-active mydvpn-client-india && echo "   ✅ Service running" || echo "   ❌ Service not running"
        echo ""
        echo "Logs (last 10 lines):"
        sudo journalctl -u mydvpn-client-india -n 10 --no-pager
        ;;
    *)
        echo "Invalid choice"
        ;;
esac
EOF

chmod +x test-client-india.sh

# Enable and start service (but don't start immediately)
echo "🚀 Setting up service (not starting yet)..."
sudo systemctl daemon-reload
sudo systemctl enable mydvpn-client-india

echo ""
echo "✅ Unified Client India setup complete!"
echo ""
echo "🎯 Testing options:"
echo ""
echo "1. Interactive testing:"
echo "   ./test-client-india.sh"
echo ""
echo "2. Start as service:"
echo "   sudo systemctl start mydvpn-client-india"
echo ""
echo "3. View logs:"
echo "   sudo journalctl -u mydvpn-client-india -f"
echo ""
echo "🔗 This client will connect to SuperNode India at:"
echo "   192.168.1.101:50052"
echo ""
echo "🌟 Key features to test:"
echo "   • Basic client mode (consume VPN)"
echo "   • Toggle to exit mode (provide VPN)"
echo "   • Connect to US exit peer via relay"
echo "   • Hybrid mode (provide + consume simultaneously)"
echo ""
echo "📝 Next: Configure Computer 6 (US client) then run test scenarios!"