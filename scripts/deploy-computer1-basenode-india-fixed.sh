#!/bin/bash

# Fix deployment script for any directory

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

# Get current directory
PROJECT_DIR="$(pwd)"

# Check if we're in the right directory (has myDvpn structure)
if [ ! -f "go.mod" ]; then
    echo "📦 Initializing Go project..."
    echo "   This directory doesn't appear to have the myDvpn project."
    echo "   Please ensure you're in the myDvpn project directory with source files."
    echo ""
    echo "   If you need to create the project structure:"
    echo "   ./scripts/create-project.sh"
    exit 1
fi

# Build myDvpn if binaries don't exist
if [ ! -f "./bin/basenode" ]; then
    echo "📦 Building myDvpn..."
    if [ -f "./scripts/build.sh" ]; then
        ./scripts/build.sh
    else
        echo "Building manually..."
        mkdir -p bin
        go build -o bin/basenode ./cmd/basenode 2>/dev/null || {
            echo "❌ Failed to build basenode. Make sure source files exist in cmd/basenode/"
            exit 1
        }
    fi
fi

# Verify binary exists
if [ ! -f "./bin/basenode" ]; then
    echo "❌ BaseNode binary not found after build!"
    echo "   Make sure you have the complete myDvpn source code"
    exit 1
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

# Create systemd service with correct path
sudo tee /etc/systemd/system/mydvpn-basenode-india.service > /dev/null << EOF
[Unit]
Description=myDvpn BaseNode (India)
After=network.target
Documentation=https://github.com/mydvpn/docs

[Service]
Type=simple
User=$USER
WorkingDirectory=$PROJECT_DIR
ExecStart=$PROJECT_DIR/bin/basenode \\
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
    echo ""
    echo "🔧 Try checking:"
    echo "   1. Binary exists: ls -la $PROJECT_DIR/bin/basenode"
    echo "   2. Binary is executable: file $PROJECT_DIR/bin/basenode"
    echo "   3. Dependencies: ldd $PROJECT_DIR/bin/basenode"
    exit 1
fi

echo ""
echo "🎯 BaseNode India setup complete!"
echo "   This node will coordinate SuperNodes in the India region"
echo "   and handle cross-region communication with US BaseNode."