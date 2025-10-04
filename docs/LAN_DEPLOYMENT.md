# myDvpn LAN Deployment Guide - Custom Network

## 6-Computer Office Setup (Your Network)

This guide sets up a complete myDvpn network across 6 computers in your LAN to test the full system including WireGuard traffic forwarding.

## Network Topology (Your IPs)

```
Computer 1: BaseNode (India)      - 192.168.1.104:50051
Computer 2: BaseNode (US)         - 192.168.1.103:50051  
Computer 3: SuperNode (India)     - 192.168.1.101:50052 → BaseNode India
Computer 4: SuperNode (US)        - 192.168.1.46:50052 → BaseNode US
Computer 5: Client (India)        - 192.168.1.40 → SuperNode India
Computer 6: Exit Peer (US)        - 192.168.1.41 → SuperNode US
```

## Traffic Flow Test Scenario

1. **Client (Computer 5)** requests exit peer in US region
2. **SuperNode India** queries **BaseNode India** 
3. **BaseNode India** coordinates with **BaseNode US**
4. **BaseNode US** finds **SuperNode US**
5. **SuperNode US** allocates **Exit Peer (Computer 6)**
6. **WireGuard tunnel** established: Client → SuperNode India → SuperNode US → Exit Peer
7. **Internet traffic** flows through this tunnel

## Pre-requisites

On ALL computers:
```bash
# Install WireGuard
sudo apt update
sudo apt install wireguard-tools

# Install Go (if not installed)
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Clone and build myDvpn
git clone <your-repo> myDvpn
cd myDvpn
./scripts/build.sh
```

## Computer Assignments

### Computer 1: BaseNode (India) - 192.168.1.104
- **Role**: Global coordination for India region
- **Ports**: 50051 (gRPC), 8080 (metrics)
- **Purpose**: Track SuperNodes, coordinate cross-region requests

### Computer 2: BaseNode (US) - 192.168.1.103  
- **Role**: Global coordination for US region
- **Ports**: 50051 (gRPC), 8080 (metrics)
- **Purpose**: Track SuperNodes, coordinate cross-region requests

### Computer 3: SuperNode (India) - 192.168.1.101
- **Role**: Regional coordinator and relay for India
- **Ports**: 50052 (gRPC), 51820 (WireGuard relay), 8080 (metrics)
- **Purpose**: Manage India peers, provide relay services

### Computer 4: SuperNode (US) - 192.168.1.46
- **Role**: Regional coordinator and relay for US  
- **Ports**: 50052 (gRPC), 51821 (WireGuard relay), 8080 (metrics)
- **Purpose**: Manage US peers, provide relay services

### Computer 5: Unified Client (India) - 192.168.1.40
- **Role**: End-user client that can switch to exit mode
- **Ports**: 51822 (WireGuard exit when enabled)
- **Purpose**: Consume VPN services, optionally provide exit services

### Computer 6: Unified Client (US) - 192.168.1.41
- **Role**: End-user client that can switch to exit mode
- **Ports**: 51823 (WireGuard exit when enabled)  
- **Purpose**: Provide exit services, optionally consume VPN services

## Deployment Steps

Follow the computer-specific instructions below for each machine.

## Testing Scenarios

After deployment, test these scenarios:

1. **Basic Connectivity**: All peers connect to their SuperNodes
2. **Cross-Region Discovery**: India client requests US exit peer
3. **WireGuard Tunnel**: Verify encrypted tunnel establishment
4. **Traffic Forwarding**: Test internet access through tunnel
5. **Role Switching**: Switch Computer 5 to exit mode
6. **Hybrid Operation**: Computer 5 provides exits while using Computer 6's exit
7. **Failover**: Stop Computer 6, verify failback to Computer 5

## Monitoring

Monitor the system using:
- Real-time logs on each computer
- Prometheus metrics on port 8080
- WireGuard status: `sudo wg show`
- Network traffic: `iftop`, `nethogs`
- Ping tests through tunnels

Let's begin with the individual computer configurations!