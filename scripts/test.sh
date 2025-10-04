#!/bin/bash

# Build and test script for myDvpn unified client system

set -e

echo "Building myDvpn components..."

# Build all components
go build -o bin/basenode ./cmd/basenode
go build -o bin/supernode ./cmd/supernode 
go build -o bin/unified-client ./cmd/unified-client

echo "All components built successfully!"

echo "Starting integration test with unified clients..."

# Kill any existing processes
pkill -f myDvpn || true
sleep 2

# Create log directory
mkdir -p logs

# Start BaseNode
echo "Starting BaseNode..."
./bin/basenode --listen=0.0.0.0:50051 --log-level=info > logs/basenode.log 2>&1 &
BASENODE_PID=$!
sleep 2

# Start SuperNode A (us-east-1)
echo "Starting SuperNode A (us-east-1)..."
./bin/supernode --id=supernode-a --region=us-east-1 --listen=0.0.0.0:50052 --basenode=localhost:50051 --log-level=info > logs/supernode-a.log 2>&1 &
SUPERNODE_A_PID=$!
sleep 2

# Start SuperNode B (us-west-1) 
echo "Starting SuperNode B (us-west-1)..."
./bin/supernode --id=supernode-b --region=us-west-1 --listen=0.0.0.0:50053 --basenode=localhost:50051 --log-level=info > logs/supernode-b.log 2>&1 &
SUPERNODE_B_PID=$!
sleep 2

# Start Unified Client 1 in us-west-1 (will be exit peer)
echo "Starting Unified Client 1 (exit mode) in us-west-1..."
./bin/unified-client --id=peer-exit-1 --region=us-west-1 --supernode=localhost:50053 --exit-port=51820 --log-level=info --no-ui > logs/peer-exit-1.log 2>&1 &
PEER_EXIT_PID=$!
sleep 3

# Start Unified Client 2 in us-east-1 (will be client)
echo "Starting Unified Client 2 (client mode) in us-east-1..."
./bin/unified-client --id=peer-client-1 --region=us-east-1 --supernode=localhost:50052 --exit-port=51821 --log-level=info --no-ui > logs/peer-client-1.log 2>&1 &
PEER_CLIENT_PID=$!
sleep 3

echo ""
echo "🌐 myDvpn Unified Client System Started!"
echo "========================================"
echo ""
echo "Components running:"
echo "  📡 BaseNode (global directory)"
echo "  🏢 SuperNode A (us-east-1) - Client region"  
echo "  🏢 SuperNode B (us-west-1) - Exit region"
echo "  👤 Unified Peer 1 (us-west-1) - Can provide exit services"
echo "  👤 Unified Peer 2 (us-east-1) - Can consume VPN services"
echo ""
echo "To test the system manually:"
echo "  1. Start another unified client with UI:"
echo "     ./bin/unified-client --id=my-peer --region=us-east-1 --supernode=localhost:50052"
echo ""
echo "  2. In the interactive UI, try these commands:"
echo "     - 'status' to see current state"
echo "     - 'toggle-exit on' to become an exit peer"
echo "     - 'connect us-west-1' to connect to exit peer in us-west-1"
echo "     - 'clients' to see connected clients (when in exit mode)"
echo ""
echo "Logs are available in the logs/ directory"
echo "Press Ctrl+C to stop all components"

# Wait for interrupt
trap "echo 'Stopping all components...'; kill $BASENODE_PID $SUPERNODE_A_PID $SUPERNODE_B_PID $PEER_EXIT_PID $PEER_CLIENT_PID 2>/dev/null || true; exit 0" INT
wait