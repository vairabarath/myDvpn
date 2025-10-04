#!/bin/bash

# Computer 1: BaseNode (India) - 192.168.1.104
# This script sets up the India BaseNode

set -e

echo "🇮🇳 Setting up BaseNode (India) on Computer 1"
echo "=============================================="
echo "IP: 192.168.1.104"
echo "Role: Global coordination for India region"
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
cat > /etc/mydvpn/basenode-india.conf << EOF
# BaseNode India Configuration
listen_addr=0.0.0.0:50051
log_level=info
region=india
node_id=basenode-india
metrics_port=8080
EOF

# Create systemd service
sudo tee /etc/systemd/system/mydvpn-basenode-india.service > /dev/null << EOF
[Unit]
Description=myDvpn BaseNode (India)
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
SyslogIdentifier=mydvpn-basenode-india

[Install]
WantedBy=multi-user.target
EOF

# Setup firewall rules
echo "🔥 Configuring firewall..."
sudo ufw allow 50051/tcp comment "myDvpn BaseNode India gRPC"
sudo ufw allow 8080/tcp comment "myDvpn BaseNode India metrics"

# Enable and start service
echo "🚀 Starting BaseNode India service..."
sudo systemctl daemon-reload
sudo systemctl enable mydvpn-basenode-india
sudo systemctl start mydvpn-basenode-india

# Wait for startup
sleep 3

# Verify service is running
if sudo systemctl is-active --quiet mydvpn-basenode-india; then
    echo "✅ BaseNode India is running successfully!"
    echo ""
    echo "📊 Service Status:"
    sudo systemctl status mydvpn-basenode-india --no-pager -l
    echo ""
    echo "🌐 Testing connectivity:"
    if netstat -tlnp 2>/dev/null | grep -q ":50051"; then
        echo "   ✅ Port 50051 is listening"
    else
        echo "   ⚠️  Port 50051 not detected (may still be starting)"
    fi
    echo ""
    echo "📝 Useful commands:"
    echo "   sudo journalctl -u mydvpn-basenode-india -f    # View logs"
    echo "   sudo systemctl stop mydvpn-basenode-india      # Stop service"
    echo "   sudo systemctl restart mydvpn-basenode-india   # Restart service"
    echo ""
    echo "🔗 Next step: Configure Computer 3 (SuperNode India) to connect to:"
    echo "   192.168.1.104:50051"
else
    echo "❌ BaseNode India failed to start!"
    echo "📋 Check logs:"
    sudo journalctl -u mydvpn-basenode-india --no-pager -l
    exit 1
fi

echo ""
echo "🎯 BaseNode India setup complete!"
echo "   This node will coordinate SuperNodes in the India region"
echo "   and handle cross-region communication with US BaseNode."