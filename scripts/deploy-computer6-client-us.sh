#!/bin/bash

# Computer 6: Unified Client (US) - 192.168.1.41
# This script sets up the US client with exit mode capability

set -e

echo "👤 Setting up Unified Client (US) on Computer 6"
echo "=============================================="
echo "IP: 192.168.1.41"
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
cat > /etc/mydvpn/client-us.conf << EOF
# Unified Client US Configuration
id=client-us
region=us
supernode_addr=192.168.1.46:50052
exit_port=51823
log_level=info
EOF

# Create systemd service
sudo tee /etc/systemd/system/mydvpn-client-us.service > /dev/null << EOF
[Unit]
Description=myDvpn Unified Client (US)
After=network.target
Documentation=https://github.com/mydvpn/docs
Requires=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$PWD
ExecStart=$PWD/bin/unified-client \\
    --id=client-us \\
    --region=us \\
    --supernode=192.168.1.46:50052 \\
    --exit-port=51823 \\
    --log-level=info \\
    --no-ui
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=mydvpn-client-us

# Capabilities for WireGuard
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW

[Install]
WantedBy=multi-user.target
EOF

# Setup firewall rules
echo "🔥 Configuring firewall..."
sudo ufw allow 51823/udp comment "myDvpn Client US WireGuard (exit mode)"

# Enable IP forwarding for exit mode
echo "🌐 Enabling IP forwarding..."
echo 'net.ipv4.ip_forward=1' | sudo tee -a /etc/sysctl.conf
sudo sysctl -p

# Create interactive script for testing
cat > test-client-us.sh << 'EOF'
#!/bin/bash

echo "🇺🇸 myDvpn Unified Client (US) - Interactive Test"
echo "==============================================="

# Function to run unified client interactively
run_interactive() {
    echo "🚀 Starting interactive unified client..."
    echo ""
    echo "Available commands when client starts:"
    echo "  status          - Show current status"
    echo "  toggle-exit on  - Enable exit mode (provide VPN to others)"
    echo "  connect india   - Connect to India exit peer"
    echo "  clients         - Show connected clients (when in exit mode)"
    echo "  disconnect      - Disconnect from current exit"
    echo "  quit            - Exit"
    echo ""
    echo "Press Enter to start..."
    read
    
    sudo ./bin/unified-client \
        --id=client-us-interactive \
        --region=us \
        --supernode=192.168.1.46:50052 \
        --exit-port=51823 \
        --log-level=info
}

# Function to run as service
run_service() {
    echo "🛠️  Service management commands:"
    echo ""
    echo "Start service:    sudo systemctl start mydvpn-client-us"
    echo "Stop service:     sudo systemctl stop mydvpn-client-us"
    echo "Restart service:  sudo systemctl restart mydvpn-client-us"
    echo "View logs:        sudo journalctl -u mydvpn-client-us -f"
    echo "Service status:   sudo systemctl status mydvpn-client-us"
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
        echo "SuperNode US connectivity:"
        nc -z 192.168.1.104 50052 && echo "   ✅ Connected" || echo "   ❌ Not connected"
        echo ""
        echo "Service status:"
        sudo systemctl is-active mydvpn-client-us && echo "   ✅ Service running" || echo "   ❌ Service not running"
        echo ""
        echo "Cross-region connectivity:"
        nc -z 192.168.1.103 50052 && echo "   ✅ Can reach India SuperNode" || echo "   ❌ Cannot reach India SuperNode"
        echo ""
        echo "Logs (last 10 lines):"
        sudo journalctl -u mydvpn-client-us -n 10 --no-pager
        ;;
    *)
        echo "Invalid choice"
        ;;
esac
EOF

chmod +x test-client-us.sh

# Enable and start service (but don't start immediately)
echo "🚀 Setting up service (not starting yet)..."
sudo systemctl daemon-reload
sudo systemctl enable mydvpn-client-us

echo ""
echo "✅ Unified Client US setup complete!"
echo ""
echo "🎯 Testing options:"
echo ""
echo "1. Interactive testing:"
echo "   ./test-client-us.sh"
echo ""
echo "2. Start as service:"
echo "   sudo systemctl start mydvpn-client-us"
echo ""
echo "3. View logs:"
echo "   sudo journalctl -u mydvpn-client-us -f"
echo ""
echo "🔗 This client will connect to SuperNode US at:"
echo "   192.168.1.46:50052"
echo ""
echo "🌟 Key features to test:"
echo "   • Basic client mode (consume VPN)"
echo "   • Toggle to exit mode (provide VPN)"  
echo "   • Connect to India exit peer via relay"
echo "   • Hybrid mode (provide + consume simultaneously)"
echo ""
echo "📝 All computers are now configured!"
echo "   Ready for end-to-end testing scenarios."