#!/bin/bash

# Demo script that simulates the unified client functionality
# without requiring root access for WireGuard operations

echo "🎮 myDvpn Unified Client Demo (Simulation Mode)"
echo "==============================================="
echo ""
echo "This demo simulates the unified client functionality without"
echo "requiring sudo access for WireGuard interface creation."
echo ""

# Check if system is running
if ! pgrep -f basenode > /dev/null; then
    echo "❌ System not running. Starting background services..."
    
    # Start minimal services for demo
    cd /mnt/EDU/myDvpn
    
    # Start BaseNode in background
    ./bin/basenode --listen=127.0.0.1:50061 --log-level=error > /dev/null 2>&1 &
    BASENODE_PID=$!
    
    # Start SuperNode in background  
    ./bin/supernode --id=demo-sn --region=demo --listen=127.0.0.1:50062 --basenode=127.0.0.1:50061 --log-level=error > /dev/null 2>&1 &
    SUPERNODE_PID=$!
    
    sleep 2
    echo "✅ Demo services started"
else
    echo "✅ System is running"
fi

echo ""
echo "🎯 Simulating Unified Client Commands:"
echo "======================================="

# Simulate the unified client experience
simulate_command() {
    local cmd="$1"
    local response="$2"
    echo ""
    echo "myDvpn> $cmd"
    sleep 1
    echo "$response"
    sleep 2
}

echo ""
echo "📱 Starting unified client: peer-demo"
echo "   Region: us-east-1"
echo "   Mode: client (default)"
echo ""

simulate_command "status" "📊 Current Status:
  Mode: client
  Connected: true
  🚪 Exit Peer: Not connected"

simulate_command "help" "Available commands:
  help (h)           - Show this help
  status (s)         - Show current status
  toggle-exit (te)   - Toggle exit node mode on/off
  connect (c)        - Connect to exit peer
  disconnect (d)     - Disconnect from current exit
  clients (cl)       - Show connected clients (exit mode)
  quit (q)           - Exit the application"

simulate_command "toggle-exit on" "✅ Exit mode enabled - You are now providing VPN services!
   Other peers can connect through you.
🔄 Mode changed to: exit"

simulate_command "status" "📊 Current Status:
  Mode: exit
  Connected: true
  👥 Active Clients: 0
  🔑 Exit Public Key: mB8pTq1q2rK3sL4tU5vW6xY7zA8bC9dE0fF1gG2hH3iI="

simulate_command "clients" "👥 No clients currently connected"

echo ""
echo "🌟 Simulating another peer connecting to our exit..."
sleep 2

simulate_command "clients" "👥 Active Clients (1):
  1. remote-user-123
     IP: 10.9.0.2
     Connected: $(date '+%Y-%m-%d %H:%M:%S')
     Session: session-abc123"

simulate_command "connect us-west-1" "🔍 Requesting exit peer in region: us-west-1...
✅ Connected to exit peer: peer-provider-west
   Endpoint: 203.0.113.45:51820
   Session: session-xyz789
🔄 Mode changed to: hybrid"

simulate_command "status" "📊 Current Status:
  Mode: hybrid
  Connected: true
  🚪 Exit Peer: peer-provider-west (203.0.113.45:51820)
  👥 Active Clients: 1
  
  💡 You are now:
     • Using VPN services from us-west-1
     • Providing VPN services to other users"

simulate_command "stats" "📊 Detailed Statistics:
  peer_id: peer-demo
  region: us-east-1
  mode: hybrid
  connected: true
  session_id: demo-session-123
  client_interface: wg-client-peer-demo
  current_exit:
    exit_peer_id: peer-provider-west
    endpoint: 203.0.113.45:51820
    session_id: session-xyz789
  exit_interface: wg-exit-peer-demo
  exit_listen_port: 51820
  active_clients: 1
  exit_public_key: mB8pTq1q2rK3sL4tU5vW6xY7zA8bC9dE0fF1gG2hH3iI="

simulate_command "toggle-exit off" "✅ Exit mode disabled - You are now in client-only mode.
🔄 Mode changed to: client"

simulate_command "disconnect" "✅ Disconnected from exit peer"

simulate_command "quit" "👋 Goodbye!"

echo ""
echo "🎉 Demo completed!"
echo ""
echo "Key Features Demonstrated:"
echo "  ✅ Dynamic mode switching (client ↔ exit ↔ hybrid)"
echo "  ✅ Real-time status updates"
echo "  ✅ Client connection monitoring (exit mode)"
echo "  ✅ Cross-region connectivity"
echo "  ✅ Simultaneous client/exit operation (hybrid mode)"
echo ""
echo "🚀 To run with real WireGuard interfaces:"
echo "  sudo ./bin/unified-client --id=my-peer"
echo ""
echo "💡 The unified client enables true decentralization where"
echo "   every user can contribute to network capacity!"

# Cleanup demo services if we started them
if [ ! -z "$BASENODE_PID" ]; then
    kill $BASENODE_PID $SUPERNODE_PID 2>/dev/null || true
fi