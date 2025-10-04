# myDvpn - Decentralized VPN with Unified Clients

A decentralized VPN system where every participant can dynamically switch between consuming and providing VPN services through a unified client application.

## 🌟 Key Innovation: Unified Client Architecture

**Every peer is both a potential client AND exit node!** Users can:
- **🔗 Connect Mode**: Use VPN services (consume bandwidth)
- **🚪 Exit Mode**: Provide VPN services (share bandwidth) 
- **⚡ Dynamic Switching**: Toggle between modes with a single button

This creates a truly decentralized network where users contribute resources when they can and consume when they need to.

## Architecture Overview

myDvpn implements a hierarchical VPN architecture with the following components:

- **BaseNode**: Global directory service that tracks SuperNodes by region
- **SuperNode**: Regional coordinators that manage peer connections and relay traffic when needed
- **Unified Peers**: Applications that can dynamically switch between client and exit modes

## System Flow

1. **Bootstrap**: Peers register with their regional SuperNode with their current mode
2. **Dynamic Mode Switch**: Users can toggle exit mode on/off through the UI
3. **Exit Request**: When in client mode, peers can request exit services in any region
4. **Discovery**: Local SuperNode queries BaseNode to find SuperNodes in target region
5. **Allocation**: Remote SuperNode finds available exit peers (including hybrid peers)
6. **Connection**: Direct connection attempted first, relay setup if needed
7. **Data Flow**: Traffic flows either directly or through SuperNode relay

## Quick Start

### Build the System

```bash
./scripts/build.sh
```

### Run Full System Test

```bash
./scripts/test.sh
```

### Try the Interactive Demo

```bash
# Start the system first
./scripts/test.sh

# In another terminal, run the demo
./scripts/demo.sh
```

### Manual Usage

**Start a unified client:**
```bash
./bin/unified-client --id=my-peer --region=us-east-1 --supernode=localhost:50052
```

**Interactive commands:**
```
myDvpn> help                    # Show all commands
myDvpn> status                  # Check current status
myDvpn> toggle-exit on          # Enable exit mode (provide VPN)
myDvpn> toggle-exit off         # Disable exit mode (client only)
myDvpn> connect us-west-1       # Connect to exit peer in us-west-1
myDvpn> clients                 # Show connected clients (exit mode)
myDvpn> disconnect              # Disconnect from current exit
myDvpn> quit                    # Exit application
```

## User Experience

### For VPN Consumers (Client Mode)
1. Start the application
2. Use `connect [region]` to connect to an exit peer
3. Your traffic is now routed through the exit peer
4. Use `disconnect` when done

### For VPN Providers (Exit Mode)  
1. Start the application
2. Use `toggle-exit on` to start providing services
3. Other users can now connect through you
4. Use `clients` to see who's connected
5. Use `toggle-exit off` to stop providing services

### For Hybrid Users
- Keep exit mode on to help the network
- Connect to other exit peers when you need different geo-location
- Your client connections are independent of your exit services

## Component Architecture

### BaseNode
- **Role**: Global directory and coordination service
- **Deployment**: Single instance or clustered for HA

### SuperNode
- **Role**: Regional control plane and relay point  
- **Deployment**: One per region, can scale horizontally

### Unified Peer (`./bin/unified-client`)
- **Role**: Dynamic client/exit peer application
- **Modes**:
  - `client`: Consumes VPN services only
  - `exit`: Provides VPN services only  
  - `hybrid`: Both simultaneously (can serve clients while connected to other exits)
- **Deployment**: End-user devices, mobile apps, etc.

## Network Requirements

- SuperNodes need public IPs or port forwarding for peer connections
- Unified peers can be behind NAT (they make outbound connections to SuperNodes)
- WireGuard UDP traffic on configured ports
- Control plane uses standard gRPC ports

## Configuration

### Command Line Options

**Unified Client**:
- `--id`: Peer ID (unique identifier)
- `--region`: Region name (us-east-1, us-west-1, etc.)
- `--supernode`: SuperNode address to connect to
- `--exit-port`: WireGuard listen port for exit mode
- `--log-level`: Log level (debug, info, warn, error)
- `--no-ui`: Disable interactive UI (for automation)

**SuperNode**:
- `--id`: SuperNode ID
- `--region`: Region name
- `--listen`: Listen address
- `--basenode`: BaseNode address
- `--log-level`: Log level

**BaseNode**:
- `--listen`: Listen address (default: 0.0.0.0:50051)
- `--log-level`: Log level

## Security

- All control communication uses TLS-secured gRPC
- Peer authentication uses Ed25519 signatures
- WireGuard provides end-to-end encryption for data plane
- Replay protection via nonces and timestamps
- Role-based access control for commands

## Monitoring

All components export Prometheus metrics and provide structured logging.

Key metrics:
- `active_streams_total`: Number of active control streams
- `commands_processed_total`: Commands processed by SuperNodes
- `wg_peers_count`: Active WireGuard peers per component
- `peer_mode_distribution`: Distribution of peer modes (client/exit/hybrid)

## Development

### Legacy Components (Backward Compatibility)

The system still includes legacy single-purpose components:
- `./bin/client`: Client-only peer
- `./bin/exitpeer`: Exit-only peer

These are maintained for compatibility but the unified client is the recommended approach.

### Testing

```bash
# Unit tests
go test ./...

# Integration tests  
./scripts/test.sh

# Interactive demo
./scripts/demo.sh
```

## Future Roadmap

- **Mobile Apps**: iOS and Android unified clients
- **Web UI**: Browser-based management interface
- **Incentive System**: Token rewards for providing exit services
- **Advanced Routing**: Multi-hop routing and onion-style privacy
- **Mesh Networking**: Peer-to-peer discovery without SuperNodes

## License

MIT License

---

**🚀 Ready to try it?** Run `./scripts/build.sh` then `./bin/unified-client` to get started!