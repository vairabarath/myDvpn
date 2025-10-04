#!/bin/bash

# Quick setup script to copy myDvpn project structure to your current directory
# Run this to create the complete project structure where you are

echo "🛠️  myDvpn Project Setup"
echo "======================="
echo ""
echo "This will create the complete myDvpn project structure in your current directory:"
echo "   $(pwd)"
echo ""

read -p "Continue? (y/n): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Cancelled."
    exit 1
fi

echo "📁 Creating directory structure..."

# Create main directories
mkdir -p {base/{proto,server},clientPeer/{client,proto},super/{server,dataplane},utils,cmd/{basenode,supernode,client,exitpeer,unified-client},docs,scripts,exitpeer}

echo "📝 Creating go.mod..."
cat > go.mod << 'EOF'
module myDvpn

go 1.21

require (
	github.com/sirupsen/logrus v1.9.3
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20241231184526-a9ab2273dd10
	google.golang.org/grpc v1.75.1
	google.golang.org/protobuf v1.36.10
)

require (
	golang.org/x/crypto v0.39.0 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250707201910-8d1bb00bc6a7 // indirect
	gonum.org/v1/gonum v0.16.0 // indirect
)
EOF

echo "🔧 Creating essential files..."

# Create a minimal main file to test
mkdir -p cmd/test
cat > cmd/test/main.go << 'EOF'
package main

import "fmt"

func main() {
	fmt.Println("myDvpn project structure created successfully!")
	fmt.Println("You now have a complete Go project ready for development.")
}
EOF

echo "📦 Initializing Go module..."
go mod tidy

echo "✅ Testing basic Go build..."
if go build -o bin/test ./cmd/test; then
    echo "✅ Go build working correctly!"
    ./bin/test
    rm -f bin/test
else
    echo "❌ Go build failed - check Go installation"
fi

echo ""
echo "🎯 Project structure created successfully!"
echo ""
echo "📋 What's created:"
echo "   • Go module (go.mod) with all dependencies"
echo "   • Complete directory structure for myDvpn"
echo "   • Ready for source code files"
echo ""
echo "📝 Next steps:"
echo "   1. Copy your myDvpn source files into the appropriate directories"
echo "   2. Run: go mod tidy"
echo "   3. Run: ./scripts/build.sh"
echo "   4. Deploy using: ./scripts/setup-computer.sh"
echo ""
echo "🔗 Directory structure:"
tree -d . 2>/dev/null || find . -type d | grep -v "\.git" | sort