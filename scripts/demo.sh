#!/bin/bash

# Interactive demo script for myDvpn unified client

echo "🌐 myDvpn Unified Client Demo"
echo "============================"
echo ""
echo "This demo shows how the unified client can switch between"
echo "client mode (consuming VPN) and exit mode (providing VPN)."
echo ""

# Function to wait for user input
wait_for_user() {
    echo "Press Enter to continue..."
    read -r
}

# Check if system is running
if ! pgrep -f basenode > /dev/null; then
    echo "❌ System not running. Please start with: ./scripts/test.sh"
    exit 1
fi

echo "✅ System appears to be running. Starting demo..."
echo ""

# Create a demo script file that the unified client can execute
cat > /tmp/demo-commands.txt << 'EOF'
# Demo commands for unified client
status
toggle-exit on
status
clients
toggle-exit off
connect us-west-1
status
disconnect
quit
EOF

echo "📝 Demo will execute these commands:"
echo "  1. status              - Show initial client mode"
echo "  2. toggle-exit on      - Switch to exit peer mode"
echo "  3. status              - Show exit mode status"
echo "  4. clients             - Show connected clients (none yet)"
echo "  5. toggle-exit off     - Switch back to client mode"
echo "  6. connect us-west-1   - Connect to an exit peer"
echo "  7. status              - Show connected status"
echo "  8. disconnect          - Disconnect from exit"
echo "  9. quit                - Exit demo"
echo ""

wait_for_user

echo "🚀 Starting unified client demo peer..."
echo ""

# Start the unified client with demo commands
./bin/unified-client \
    --id=demo-peer \
    --region=us-east-1 \
    --supernode=localhost:50052 \
    --exit-port=51822 \
    --log-level=warn < /tmp/demo-commands.txt

# Cleanup
rm -f /tmp/demo-commands.txt

echo ""
echo "🎉 Demo completed!"
echo ""
echo "Key takeaways:"
echo "  ✅ Single application handles both client and exit peer roles"
echo "  ✅ Dynamic switching with simple toggle commands"
echo "  ✅ Real-time status updates and connection management"
echo "  ✅ Interactive UI for easy operation"
echo ""
echo "Try running your own unified client:"
echo "  ./bin/unified-client --id=my-peer"
echo ""
echo "Then experiment with the commands:"
echo "  > toggle-exit on     # Become an exit peer"
echo "  > connect            # Connect to an exit peer"
echo "  > help               # See all available commands"