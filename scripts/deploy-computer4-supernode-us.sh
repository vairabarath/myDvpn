#!/bin/bash

# Computer 4: SuperNode (US) - 192.168.1.46
# This script sets up the US SuperNode with relay capabilities

set -e

echo "🏢 Setting up SuperNode (US) on Computer 4"
echo "==========================================="
echo "IP: 192.168.1.46"
echo "Role: Regional coordinator and relay for US"
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
cat > /etc/mydvpn/supernode-us.conf << EOF
# SuperNode US Configuration
id=supernode-us
region=us
listen_addr=0.0.0.0:50052
basenode_addr=192.168.1.103:50051
log_level=info
relay_interface=wg-relay-us
relay_port=51821
metrics_port=8080
EOF

# Create systemd service
sudo tee /etc/systemd/system/mydvpn-supernode-us.service > /dev/null << EOF
[Unit]
Description=myDvpn SuperNode (US)
After=network.target
Documentation=https://github.com/mydvpn/docs
Requires=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$PWD
ExecStart=$PWD/bin/supernode \\
    --id=supernode-us \\
    --region=us \\
    --listen=0.0.0.0:50052 \\
    --basenode=192.168.1.103:50051 \\
    --log-level=info
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=mydvpn-supernode-us

# Capabilities for WireGuard
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW

[Install]
WantedBy=multi-user.target
EOF

# Setup firewall rules
echo "🔥 Configuring firewall..."
sudo ufw allow 50052/tcp comment "myDvpn SuperNode US gRPC"
sudo ufw allow 51821/udp comment "myDvpn SuperNode US WireGuard relay"
sudo ufw allow 8080/tcp comment "myDvpn SuperNode US metrics"

# Enable IP forwarding
echo "🌐 Enabling IP forwarding..."
echo 'net.ipv4.ip_forward=1' | sudo tee -a /etc/sysctl.conf
sudo sysctl -p

# Enable and start service
echo "🚀 Starting SuperNode US service..."
sudo systemctl daemon-reload
sudo systemctl enable mydvpn-supernode-us
sudo systemctl start mydvpn-supernode-us

# Wait for startup
sleep 5

# Verify service is running
if sudo systemctl is-active --quiet mydvpn-supernode-us; then
    echo "✅ SuperNode US is running successfully!"
    echo ""
    echo "📊 Service Status:"
    sudo systemctl status mydvpn-supernode-us --no-pager -l
    echo ""
    echo "🌐 Testing connectivity:"
    if netstat -tlnp 2>/dev/null | grep -q ":50052"; then
        echo "   ✅ gRPC port 50052 is listening"
    else
        echo "   ⚠️  gRPC port 50052 not detected"
    fi
    
    if netstat -unp 2>/dev/null | grep -q ":51821"; then
        echo "   ✅ WireGuard relay port 51821 is listening"
    else
        echo "   ⚠️  WireGuard relay port 51821 not detected"
    fi
    echo ""
    echo "🔗 Testing connection to US BaseNode..."
    if nc -z 192.168.1.103 50051 2>/dev/null; then
        echo "   ✅ Can reach US BaseNode (192.168.1.103:50051)"
    else
        echo "   ❌ Cannot reach US BaseNode (check Computer 2)"
    fi
    echo ""
    echo "🔗 Testing cross-region connectivity..."
    if nc -z 192.168.1.101 50052 2>/dev/null; then
        echo "   ✅ Can reach India SuperNode (192.168.1.101:50052)"
    else
        echo "   ⚠️  Cannot reach India SuperNode (check Computer 3)"
    fi
    echo ""
    echo "📝 Useful commands:"
    echo "   sudo journalctl -u mydvpn-supernode-us -f      # View logs"
    echo "   sudo systemctl stop mydvpn-supernode-us        # Stop service"
    echo "   sudo systemctl restart mydvpn-supernode-us     # Restart service"
    echo "   sudo wg show                                    # Show WireGuard status"
    echo ""
    echo "🔗 Next step: Configure Computer 6 (Client US) to connect to:"
    echo "   192.168.1.46:50052"
else
    echo "❌ SuperNode US failed to start!"
    echo "📋 Check logs:"
    sudo journalctl -u mydvpn-supernode-us --no-pager -l
    exit 1
fi

echo ""
echo "🎯 SuperNode US setup complete!"
echo "   This node will manage US peers and provide relay services"
echo "   for cross-region connections from India peers."