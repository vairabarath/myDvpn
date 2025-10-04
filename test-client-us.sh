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
    echo "Choose mode:"
    echo "1) Development mode (no WireGuard - works without sudo)"
    echo "2) Production mode (real WireGuard - requires sudo)"
    echo ""
    read -p "Enter choice (1-2): " mode_choice
    
    case $mode_choice in
        1)
            echo "🔧 Starting in development mode..."
            ./bin/unified-client-dev \
                --id=client-us-interactive \
                --region=us \
                --supernode=192.168.1.46:50052 \
                --exit-port=51823 \
                --log-level=info \
                --dev
            ;;
        2)
            echo "🛡️  Starting in production mode (requires sudo)..."
            sudo ./bin/unified-client \
                --id=client-us-interactive \
                --region=us \
                --supernode=192.168.1.46:50052 \
                --exit-port=51823 \
                --log-level=info
            ;;
        *)
            echo "Invalid choice, defaulting to development mode..."
            ./bin/unified-client-dev \
                --id=client-us-interactive \
                --region=us \
                --supernode=192.168.1.46:50052 \
                --exit-port=51823 \
                --log-level=info \
                --dev
            ;;
    esac
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
