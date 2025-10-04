#!/bin/bash

# Simple build verification script

echo "=== Building myDvpn Unified Client System ==="

# Get the current directory (where the script is run from)
PROJECT_DIR="$(pwd)"

# Clean previous builds
rm -rf bin/
mkdir -p bin

# Check if we have the right directory structure
if [ ! -f "go.mod" ]; then
    echo "📦 Initializing Go module..."
    go mod init myDvpn
    go mod tidy
fi

echo "Building BaseNode..."
if go build -o bin/basenode ./cmd/basenode 2>/dev/null; then
    echo "✓ BaseNode built successfully"
else
    echo "❌ BaseNode build failed - check if source files exist"
fi

echo "Building SuperNode..."
if go build -o bin/supernode ./cmd/supernode 2>/dev/null; then
    echo "✓ SuperNode built successfully"
else
    echo "❌ SuperNode build failed - check if source files exist"
fi

echo "Building Unified Client..."
if go build -o bin/unified-client ./cmd/unified-client 2>/dev/null; then
    echo "✓ Unified Client built successfully"
else
    echo "❌ Unified Client build failed - check if source files exist"
fi

echo "Building Legacy Client (for compatibility)..."
if go build -o bin/client ./cmd/client 2>/dev/null; then
    echo "✓ Legacy Client built successfully"
else
    echo "❌ Legacy Client build failed - check if source files exist"
fi

echo "Building Legacy Exit Peer (for compatibility)..."
if go build -o bin/exitpeer ./cmd/exitpeer 2>/dev/null; then
    echo "✓ Legacy Exit Peer built successfully"
else
    echo "❌ Legacy Exit Peer build failed - check if source files exist"
fi

echo ""
echo "=== Build Summary ==="
ls -la bin/
echo ""

echo "=== Component Versions ==="
if [ -f "./bin/basenode" ]; then
    ./bin/basenode --help 2>&1 | head -3 || echo "BaseNode help output"
else
    echo "❌ BaseNode binary not found"
fi

if [ -f "./bin/supernode" ]; then
    ./bin/supernode --help 2>&1 | head -3 || echo "SuperNode help output"
else
    echo "❌ SuperNode binary not found"
fi

if [ -f "./bin/unified-client" ]; then
    ./bin/unified-client --help 2>&1 | head -3 || echo "Unified Client help output"
else
    echo "❌ Unified Client binary not found"
fi

echo ""
if [ -f "./bin/basenode" ] && [ -f "./bin/supernode" ] && [ -f "./bin/unified-client" ]; then
    echo "=== All components built successfully! ==="
    echo ""
    echo "🌟 NEW: Unified Client Application 🌟"
    echo "  - Single application that can act as both client and exit peer"
    echo "  - Toggle between modes with 'toggle-exit on/off'"
    echo "  - Interactive UI for easy management"
    echo ""
    echo "To test the unified system:"
    echo "  ./scripts/test.sh              # Start full system"
    echo "  ./bin/unified-client           # Start interactive client"
    echo ""
    echo "To start a peer manually:"
    echo "  ./bin/unified-client --id=my-peer --region=us-east-1 --supernode=localhost:50052"
    echo ""
    echo "Then in the UI:"
    echo "  > toggle-exit on               # Become an exit peer"
    echo "  > connect us-west-1            # Connect to exit in us-west-1"
    echo "  > status                       # Check current status"
    echo "  > help                         # See all commands"
else
    echo "❌ Some components failed to build!"
    echo "   Make sure you have the complete myDvpn source code"
    echo "   and run this from the project root directory."
fi