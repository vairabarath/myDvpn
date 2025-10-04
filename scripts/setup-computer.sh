#!/bin/bash

# Quick setup script for all 6 computers
# Copy this to each computer and run the appropriate section

echo "🖥️  myDvpn 6-Computer Quick Setup"
echo "================================"
echo ""

echo "Choose your computer:"
echo "1) Computer 1 - BaseNode India (192.168.1.104)"
echo "2) Computer 2 - BaseNode US (192.168.1.103)"
echo "3) Computer 3 - SuperNode India (192.168.1.101)"
echo "4) Computer 4 - SuperNode US (192.168.1.46)"
echo "5) Computer 5 - Client India (192.168.1.40)"
echo "6) Computer 6 - Client US (192.168.1.41)"
echo "7) Deploy all (if running from shared location)"
echo ""

read -p "Enter choice (1-7): " choice

case $choice in
    1)
        echo "🇮🇳 Setting up Computer 1 - BaseNode India"
        ./scripts/deploy-computer1-basenode-india.sh
        ;;
    2)
        echo "🇺🇸 Setting up Computer 2 - BaseNode US"
        ./scripts/deploy-computer2-basenode-us.sh
        ;;
    3)
        echo "🏢 Setting up Computer 3 - SuperNode India"
        ./scripts/deploy-computer3-supernode-india.sh
        ;;
    4)
        echo "🏢 Setting up Computer 4 - SuperNode US"
        ./scripts/deploy-computer4-supernode-us.sh
        ;;
    5)
        echo "👤 Setting up Computer 5 - Client India"
        ./scripts/deploy-computer5-client-india.sh
        ;;
    6)
        echo "👤 Setting up Computer 6 - Client US"
        ./scripts/deploy-computer6-client-us.sh
        ;;
    7)
        echo "🚀 Deploy All Instructions"
        echo "=========================="
        echo ""
        echo "Copy the myDvpn project to each computer, then run:"
        echo ""
        echo "Computer 1 (192.168.1.104): ./scripts/deploy-computer1-basenode-india.sh"
        echo "Computer 2 (192.168.1.103): ./scripts/deploy-computer2-basenode-us.sh"
        echo "Computer 3 (192.168.1.101): ./scripts/deploy-computer3-supernode-india.sh"
        echo "Computer 4 (192.168.1.46): ./scripts/deploy-computer4-supernode-us.sh"
        echo "Computer 5 (192.168.1.40): ./scripts/deploy-computer5-client-india.sh"
        echo "Computer 6 (192.168.1.41): ./scripts/deploy-computer6-client-us.sh"
        echo ""
        echo "Then run the test suite: ./scripts/test-lan-deployment.sh"
        ;;
    *)
        echo "❌ Invalid choice"
        exit 1
        ;;
esac

echo ""
echo "✅ Setup completed for this computer!"
echo ""
echo "📝 Next steps:"
echo "1. Ensure all 6 computers are configured"
echo "2. Run the test suite: ./scripts/test-lan-deployment.sh"
echo "3. Try interactive clients for manual testing"