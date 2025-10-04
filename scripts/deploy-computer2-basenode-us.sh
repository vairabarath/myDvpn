#!/bin/bash

# Computer 2: BaseNode (US) - 192.168.1.103
# This script sets up the US BaseNode

set -e

echo "🇺🇸 Setting up BaseNode (US) on Computer 2"
echo "==========================================="
echo "IP: 192.168.1.103"
echo "Role: Global coordination for US region"
echo ""

# Check if running as root
if [[ $EUID -eq 0 ]]; then
   echo "❌ This script should not be run as root"
   echo "   Run as regular user, sudo will be used when needed"
   exit 1
fi

# Verify myDvpn is built
if [ ! -f "./bin/basenode" ]; then
    echo "📦 Building myDvpn..."
    ./scripts/build.sh
fi

# Create configuration directory
sudo mkdir -p /etc/mydvpn
sudo chown $USER:$USER /etc/mydvpn

# Create BaseNode configuration
cat > /etc/mydvpn/basenode-us.conf << EOF
# BaseNode US Configuration
listen_addr=0.0.0.0:50051
log_level=info
region=us
node_id=basenode-us
metrics_port=8080
peer_basenode=192.168.1.104:50051
EOF

# Create systemd service
sudo tee /etc/systemd/system/mydvpn-basenode-us.service > /dev/null << EOF
[Unit]
Description=myDvpn BaseNode (US)
After=network.target
Documentation=https://github.com/mydvpn/docs

[Service]
Type=simple
User=$USER
WorkingDirectory=$PWD
ExecStart=$PWD/bin/basenode \\
    --listen=0.0.0.0:50051 \\
    --log-level=info
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=mydvpn-basenode-us

[Install]
WantedBy=multi-user.target
EOF

# Setup firewall rules
echo "🔥 Configuring firewall..."
sudo ufw allow 50051/tcp comment "myDvpn BaseNode US gRPC"
sudo ufw allow 8080/tcp comment "myDvpn BaseNode US metrics"

# Enable and start service
echo "🚀 Starting BaseNode US service..."
sudo systemctl daemon-reload
sudo systemctl enable mydvpn-basenode-us
sudo systemctl start mydvpn-basenode-us

# Wait for startup
sleep 3

# Verify service is running
if sudo systemctl is-active --quiet mydvpn-basenode-us; then
    echo "✅ BaseNode US is running successfully!"
    echo ""
    echo "📊 Service Status:"
    sudo systemctl status mydvpn-basenode-us --no-pager -l
    echo ""
    echo "🌐 Testing connectivity:"
    if netstat -tlnp 2>/dev/null | grep -q ":50051"; then
        echo "   ✅ Port 50051 is listening"
    else
        echo "   ⚠️  Port 50051 not detected (may still be starting)"
    fi
    echo ""
    echo "🔗 Testing connection to India BaseNode..."
    if nc -z 192.168.1.104 50051 2>/dev/null; then
        echo "   ✅ Can reach India BaseNode (192.168.1.104:50051)"
    else
        echo "   ⚠️  Cannot reach India BaseNode (check if Computer 1 is running)"
    fi
    echo ""
    echo "📝 Useful commands:"
    echo "   sudo journalctl -u mydvpn-basenode-us -f       # View logs"
    echo "   sudo systemctl stop mydvpn-basenode-us         # Stop service"
    echo "   sudo systemctl restart mydvpn-basenode-us      # Restart service"
    echo ""
    echo "🔗 Next step: Configure Computer 4 (SuperNode US) to connect to:"
    echo "   192.168.1.103:50051"
else
    echo "❌ BaseNode US failed to start!"
    echo "📋 Check logs:"
    sudo journalctl -u mydvpn-basenode-us --no-pager -l
    exit 1
fi

echo ""
echo "🎯 BaseNode US setup complete!"
echo "   This node will coordinate SuperNodes in the US region"
echo "   and handle cross-region communication with India BaseNode."