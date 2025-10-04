#!/bin/bash

# Computer 3: SuperNode (India) - 192.168.1.101
# This script sets up the India SuperNode with relay capabilities

set -e

echo "🏢 Setting up SuperNode (India) on Computer 3"
echo "=============================================="
echo "IP: 192.168.1.101"
echo "Role: Regional coordinator and relay for India"
echo ""

# Check if running as root
if [[ $EUID -eq 0 ]]; then
   echo "❌ This script should not be run as root"
   echo "   Run as regular user, sudo will be used when needed"
   exit 1
fi

# Verify myDvpn is built
if [ ! -f "./bin/supernode" ]; then
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

# Create SuperNode configuration
cat > /etc/mydvpn/supernode-india.conf << EOF
# SuperNode India Configuration
id=supernode-india
region=india
listen_addr=0.0.0.0:50052
basenode_addr=192.168.1.104:50051
log_level=info
relay_interface=wg-relay-india
relay_port=51820
metrics_port=8080
EOF

# Create systemd service
sudo tee /etc/systemd/system/mydvpn-supernode-india.service > /dev/null << EOF
[Unit]
Description=myDvpn SuperNode (India)
After=network.target
Documentation=https://github.com/mydvpn/docs
Requires=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$PWD
ExecStart=$PWD/bin/supernode \\
    --id=supernode-india \\
    --region=india \\
    --listen=0.0.0.0:50052 \\
    --basenode=192.168.1.104:50051 \\
    --log-level=info
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=mydvpn-supernode-india

# Capabilities for WireGuard
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW

[Install]
WantedBy=multi-user.target
EOF

# Setup firewall rules
echo "🔥 Configuring firewall..."
sudo ufw allow 50052/tcp comment "myDvpn SuperNode India gRPC"
sudo ufw allow 51820/udp comment "myDvpn SuperNode India WireGuard relay"
sudo ufw allow 8080/tcp comment "myDvpn SuperNode India metrics"

# Enable IP forwarding
echo "🌐 Enabling IP forwarding..."
echo 'net.ipv4.ip_forward=1' | sudo tee -a /etc/sysctl.conf
sudo sysctl -p

# Enable and start service
echo "🚀 Starting SuperNode India service..."
sudo systemctl daemon-reload
sudo systemctl enable mydvpn-supernode-india
sudo systemctl start mydvpn-supernode-india

# Wait for startup
sleep 5

# Verify service is running
if sudo systemctl is-active --quiet mydvpn-supernode-india; then
    echo "✅ SuperNode India is running successfully!"
    echo ""
    echo "📊 Service Status:"
    sudo systemctl status mydvpn-supernode-india --no-pager -l
    echo ""
    echo "🌐 Testing connectivity:"
    if netstat -tlnp 2>/dev/null | grep -q ":50052"; then
        echo "   ✅ gRPC port 50052 is listening"
    else
        echo "   ⚠️  gRPC port 50052 not detected"
    fi
    
    if netstat -unp 2>/dev/null | grep -q ":51820"; then
        echo "   ✅ WireGuard relay port 51820 is listening"
    else
        echo "   ⚠️  WireGuard relay port 51820 not detected"
    fi
    echo ""
    echo "🔗 Testing connection to India BaseNode..."
    if nc -z 192.168.1.104 50051 2>/dev/null; then
        echo "   ✅ Can reach India BaseNode (192.168.1.104:50051)"
    else
        echo "   ❌ Cannot reach India BaseNode (check Computer 1)"
    fi
    echo ""
    echo "📝 Useful commands:"
    echo "   sudo journalctl -u mydvpn-supernode-india -f   # View logs"
    echo "   sudo systemctl stop mydvpn-supernode-india     # Stop service"
    echo "   sudo systemctl restart mydvpn-supernode-india  # Restart service"
    echo "   sudo wg show                                    # Show WireGuard status"
    echo ""
    echo "🔗 Next step: Configure Computer 5 (Client India) to connect to:"
    echo "   192.168.1.101:50052"
else
    echo "❌ SuperNode India failed to start!"
    echo "📋 Check logs:"
    sudo journalctl -u mydvpn-supernode-india --no-pager -l
    exit 1
fi

echo ""
echo "🎯 SuperNode India setup complete!"
echo "   This node will manage India peers and provide relay services"
echo "   for cross-region connections to US peers."