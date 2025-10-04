#!/bin/bash

# End-to-End Testing Script for 6-Computer myDvpn LAN Deployment
# Run this after all computers are deployed

set -e

echo "🧪 myDvpn 6-Computer LAN End-to-End Test Suite"
echo "=============================================="
echo ""

# Computer assignments
COMPUTERS=(
    "Computer 1 (BaseNode India):    192.168.1.104:50051"
    "Computer 2 (BaseNode US):       192.168.1.103:50051" 
    "Computer 3 (SuperNode India):   192.168.1.101:50052"
    "Computer 4 (SuperNode US):      192.168.1.46:50052"
    "Computer 5 (Client India):      192.168.1.40"
    "Computer 6 (Client US):         192.168.1.41"
)

echo "📋 Network Topology:"
for computer in "${COMPUTERS[@]}"; do
    echo "   $computer"
done
echo ""

# Test functions
test_connectivity() {
    echo "🔗 Test 1: Basic Connectivity"
    echo "=============================="
    
    local services=(
        "192.168.1.104:50051:BaseNode India"
        "192.168.1.103:50051:BaseNode US"
        "192.168.1.101:50052:SuperNode India"
        "192.168.1.46:50052:SuperNode US"
    )
    
    for service in "${services[@]}"; do
        IFS=':' read -r ip port name <<< "$service"
        if nc -z "$ip" "$port" 2>/dev/null; then
            echo "   ✅ $name ($ip:$port) - Reachable"
        else
            echo "   ❌ $name ($ip:$port) - Not reachable"
        fi
    done
    echo ""
}

test_registration() {
    echo "🏢 Test 2: SuperNode Registration"
    echo "================================="
    echo "   Checking if SuperNodes registered with BaseNodes..."
    echo ""
    echo "   📝 Check logs on Computer 3 and 4:"
    echo "      sudo journalctl -u mydvpn-supernode-* --since '5 minutes ago' | grep 'Successfully registered'"
    echo ""
}

test_peer_authentication() {
    echo "👤 Test 3: Peer Authentication"
    echo "=============================="
    echo "   Starting services on Computer 5 and 6..."
    echo ""
    echo "   📝 On Computer 5 (India Client):"
    echo "      sudo systemctl start mydvpn-client-india"
    echo "      sudo journalctl -u mydvpn-client-india -f"
    echo ""
    echo "   📝 On Computer 6 (US Client):"
    echo "      sudo systemctl start mydvpn-client-us"
    echo "      sudo journalctl -u mydvpn-client-us -f"
    echo ""
    echo "   ✅ Look for 'Authentication successful' messages"
    echo ""
}

test_role_switching() {
    echo "🔄 Test 4: Role Switching"
    echo "========================="
    echo "   Testing dynamic role changes..."
    echo ""
    echo "   📝 On Computer 6 (US Client):"
    echo "      ./test-client-us.sh"
    echo "      Choose option 1 (Interactive mode)"
    echo ""
    echo "   Then test these commands:"
    echo "      myDvpn> status                  # Should show 'Mode: client'"
    echo "      myDvpn> toggle-exit on          # Switch to exit mode"
    echo "      myDvpn> status                  # Should show 'Mode: exit'"
    echo "      myDvpn> clients                 # Show connected clients"
    echo ""
}

test_cross_region_connection() {
    echo "🌍 Test 5: Cross-Region Connection"
    echo "=================================="
    echo "   Testing India client connecting to US exit..."
    echo ""
    echo "   📝 On Computer 5 (India Client):"
    echo "      ./test-client-india.sh"
    echo "      Choose option 1 (Interactive mode)"
    echo ""
    echo "   Then test:"
    echo "      myDvpn> connect us              # Connect to US exit peer"
    echo "      myDvpn> status                  # Should show connected to US exit"
    echo ""
    echo "   📝 On Computer 6 (US Client - in exit mode):"
    echo "      myDvpn> clients                 # Should show India client connected"
    echo ""
}

test_wireguard_tunnel() {
    echo "🔌 Test 6: WireGuard Tunnel"
    echo "==========================="
    echo "   Verifying WireGuard interfaces and tunnels..."
    echo ""
    echo "   📝 On all computers, check WireGuard status:"
    echo "      sudo wg show"
    echo ""
    echo "   📝 On Computer 5 (when connected):"
    echo "      ip route show                   # Check routing"
    echo "      ping 8.8.8.8                   # Test internet via tunnel"
    echo ""
    echo "   📝 On Computer 6 (exit mode):"
    echo "      sudo iptables -L -n -v          # Check NAT rules"
    echo "      sudo iptables -t nat -L -n -v   # Check MASQUERADE rules"
    echo ""
}

test_traffic_forwarding() {
    echo "📡 Test 7: Traffic Forwarding"
    echo "============================="
    echo "   Testing actual internet traffic through VPN tunnel..."
    echo ""
    echo "   📝 On Computer 5 (India Client connected to US exit):"
    echo "      curl -4 ifconfig.me             # Check public IP (should be US)"
    echo "      traceroute 8.8.8.8             # Check route through tunnel"
    echo ""
    echo "   📝 Monitor traffic on Computer 6 (US Exit):"
    echo "      sudo tcpdump -i wg-exit-client-us"
    echo "      sudo nethogs wg-exit-client-us"
    echo ""
}

test_hybrid_mode() {
    echo "🔀 Test 8: Hybrid Mode"
    echo "======================"
    echo "   Testing simultaneous client + exit operation..."
    echo ""
    echo "   📝 On Computer 5 (India):"
    echo "      myDvpn> toggle-exit on          # Enable exit mode"
    echo "      myDvpn> connect us              # Connect to US exit"
    echo "      myDvpn> status                  # Should show 'Mode: hybrid'"
    echo ""
    echo "   📝 On Computer 6 (US):"
    echo "      myDvpn> connect india           # Connect to India exit (Computer 5)"
    echo ""
    echo "   Now both computers are providing AND consuming VPN services!"
    echo ""
}

test_monitoring() {
    echo "📊 Test 9: Monitoring & Metrics"
    echo "==============================="
    echo "   Checking system monitoring capabilities..."
    echo ""
    echo "   📝 Check metrics endpoints:"
    echo "      curl http://192.168.1.104:8080/metrics    # BaseNode India"
    echo "      curl http://192.168.1.103:8080/metrics    # BaseNode US"
    echo "      curl http://192.168.1.101:8080/metrics    # SuperNode India"
    echo "      curl http://192.168.1.46:8080/metrics     # SuperNode US"
    echo ""
    echo "   📝 Check logs across all components:"
    echo "      sudo journalctl -u mydvpn-* --since '10 minutes ago'"
    echo ""
}

test_failover() {
    echo "🔄 Test 10: Failover & Recovery"
    echo "==============================="
    echo "   Testing system resilience..."
    echo ""
    echo "   📝 Test SuperNode failover:"
    echo "      sudo systemctl stop mydvpn-supernode-us    # Stop US SuperNode"
    echo "      # Check if India client can still connect via relay"
    echo "      sudo systemctl start mydvpn-supernode-us   # Restart"
    echo ""
    echo "   📝 Test peer reconnection:"
    echo "      # Disconnect and reconnect clients"
    echo "      # Verify automatic reconnection"
    echo ""
}

# Main test runner
run_all_tests() {
    echo "🚀 Running all tests..."
    echo ""
    
    test_connectivity
    sleep 2
    
    test_registration
    sleep 2
    
    test_peer_authentication
    sleep 2
    
    echo "⏸️  Manual testing phase - follow the instructions above"
    echo "   for interactive testing of role switching, connections,"
    echo "   and traffic forwarding."
    echo ""
    echo "   Press Enter when ready to continue with monitoring tests..."
    read
    
    test_monitoring
    sleep 2
    
    echo "🎉 Automated tests complete!"
    echo ""
    echo "📝 Manual test checklist:"
    echo "   □ Role switching (client ↔ exit ↔ hybrid)"
    echo "   □ Cross-region connections"
    echo "   □ WireGuard tunnel establishment"
    echo "   □ Internet traffic forwarding"
    echo "   □ Hybrid mode operation"
    echo "   □ Failover scenarios"
    echo ""
}

# Interactive menu
show_menu() {
    echo "Choose test to run:"
    echo "1)  Basic connectivity"
    echo "2)  SuperNode registration"
    echo "3)  Peer authentication"
    echo "4)  Role switching"
    echo "5)  Cross-region connection"
    echo "6)  WireGuard tunnel"
    echo "7)  Traffic forwarding"
    echo "8)  Hybrid mode"
    echo "9)  Monitoring & metrics"
    echo "10) Failover & recovery"
    echo "11) Run all automated tests"
    echo "12) Show system status"
    echo "0)  Exit"
    echo ""
}

show_system_status() {
    echo "📊 Current System Status"
    echo "========================"
    echo ""
    
    echo "🔗 Service Connectivity:"
    test_connectivity
    
    echo "🖥️  Service Status (check on each computer):"
    echo "   Computer 1: systemctl status mydvpn-basenode-india"
    echo "   Computer 2: systemctl status mydvpn-basenode-us"
    echo "   Computer 3: systemctl status mydvpn-supernode-india"
    echo "   Computer 4: systemctl status mydvpn-supernode-us"
    echo "   Computer 5: systemctl status mydvpn-client-india"
    echo "   Computer 6: systemctl status mydvpn-client-us"
    echo ""
}

# Main execution
if [ "$1" = "--auto" ]; then
    run_all_tests
else
    while true; do
        show_menu
        read -p "Enter choice (0-12): " choice
        
        case $choice in
            1) test_connectivity ;;
            2) test_registration ;;
            3) test_peer_authentication ;;
            4) test_role_switching ;;
            5) test_cross_region_connection ;;
            6) test_wireguard_tunnel ;;
            7) test_traffic_forwarding ;;
            8) test_hybrid_mode ;;
            9) test_monitoring ;;
            10) test_failover ;;
            11) run_all_tests ;;
            12) show_system_status ;;
            0) echo "👋 Goodbye!"; exit 0 ;;
            *) echo "❌ Invalid choice" ;;
        esac
        
        echo ""
        read -p "Press Enter to continue..."
        echo ""
    done
fi