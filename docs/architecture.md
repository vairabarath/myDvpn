# myDvpn Architecture

## System Overview

myDvpn is a decentralized VPN control plane that provides dynamic peer discovery, cross-region routing, and intelligent relay capabilities. The system is designed around persistent control streams that enable real-time orchestration of WireGuard data plane connections.

## Component Architecture

### BaseNode
- **Role**: Global directory and coordination service
- **Responsibilities**:
  - Track SuperNodes by region and capacity
  - Route cross-region exit requests
  - Provide admin visibility into network topology
- **Deployment**: Single instance or clustered for HA

### SuperNode
- **Role**: Regional control plane and relay point
- **Responsibilities**:
  - Manage persistent streams from clients and exit peers
  - Orchestrate exit peer allocation 
  - Provide relay services when direct connections fail
  - Implement data plane forwarding via WireGuard and iptables
- **Deployment**: One per region, can scale horizontally

### ClientPeer
- **Role**: VPN client/consumer
- **Responsibilities**:
  - Maintain persistent control stream to local SuperNode
  - Accept commands for exit setup and peer rotation
  - Manage local WireGuard interface
- **Deployment**: End-user devices, mobile apps, etc.

### ExitPeer  
- **Role**: Internet gateway/egress point
- **Responsibilities**:
  - Maintain persistent stream to supervising SuperNode
  - Accept commands to add/remove client peers
  - Provide WireGuard endpoints and IP forwarding
  - Handle NAT and routing for client traffic
- **Deployment**: User-hosted nodes, VPS instances

## Protocol Design

### Persistent Control Streams
All peers maintain bi-directional gRPC streams to their SuperNode:

```protobuf
service ControlStream {
  rpc PersistentControlStream(stream ControlMessage) returns (stream ControlMessage);
}
```

Messages include:
- **AuthRequest/AuthResponse**: Cryptographic authentication
- **PingRequest/PongResponse**: Heartbeat and latency measurement  
- **Command/CommandResponse**: Server-to-peer instructions
- **InfoRequest/InfoResponse**: State synchronization

### Authentication
- Ed25519 signature-based authentication
- Signed payload: `peer_id||role||region||nonce`
- TLS transport encryption
- Replay protection via nonces

### Command Types
- **SETUP_EXIT**: Configure exit peer for specific client
- **ROTATE_PEER**: Switch client to different exit peer
- **RELAY_SETUP**: Configure SuperNode relay forwarding
- **DISCONNECT**: Graceful connection teardown

## Data Flow

### Direct Connection Flow
```
ClientPeer <--WireGuard--> ExitPeer
```
1. Client requests exit peer from local SuperNode
2. SuperNode queries BaseNode for target region SuperNodes
3. Remote SuperNode allocates exit peer and sends SETUP_EXIT command
4. Exit peer adds client's public key and returns endpoint info
5. Client configures WireGuard to connect directly to exit peer

### Relay Connection Flow  
```
ClientPeer <--WG--> LocalSuperNode <--inter-SN--> RemoteSuperNode <--WG--> ExitPeer
```
1. Direct connection attempt fails (NAT/firewall)
2. Local SuperNode sets up relay WireGuard interface
3. Remote SuperNode configures exit peer with relay endpoint
4. Client connects to local SuperNode relay endpoint
5. Traffic flows: Client -> Local SN -> Remote SN -> Exit Peer

## Failure Handling

### Network Partitions
- Clients automatically reconnect with exponential backoff
- SuperNodes re-register with BaseNode on reconnection
- Stale streams cleaned up after configurable timeout

### Component Failures
- BaseNode failure: SuperNodes cache peer allocations
- SuperNode failure: Clients failover to backup SuperNodes
- Exit peer failure: SuperNode reallocates clients to healthy peers

### Command Failures
- Commands have unique IDs and timeout handling
- Failed commands trigger rollback procedures
- Idempotent command processing prevents duplicate operations

## Security Model

### Trust Boundaries
- BaseNode trusts registered SuperNodes (operator-controlled)
- SuperNodes verify client/exit peer signatures
- Clients trust their configured SuperNode
- End-to-end crypto via WireGuard independent of control plane

### Attack Mitigation
- Rate limiting on authentication attempts
- Signature verification prevents peer impersonation  
- TLS prevents MITM on control channels
- WireGuard prevents data plane tampering

## Scalability

### Horizontal Scaling
- Multiple SuperNodes per region for load distribution
- BaseNode can be clustered with shared state
- Exit peers scale independently of control plane

### Performance Characteristics
- Persistent streams eliminate connection overhead
- Command processing: < 10ms typical latency
- Stream capacity: 10,000+ concurrent per SuperNode
- Relay throughput: Limited by SuperNode bandwidth

## Observability

### Metrics
- Active stream counts and health
- Command success/failure rates
- Peer allocation and utilization
- Network latency and throughput

### Logging
- Structured logging with peer/session context
- Command traces for debugging
- Security events and authentication failures

### Health Checks
- Heartbeat monitoring for all components
- Stream liveness detection
- WireGuard interface status checking

## Deployment Considerations

### Network Requirements
- SuperNodes require public IP addresses
- Exit peers can be behind NAT
- WireGuard UDP ports must be reachable
- Control plane uses standard gRPC ports

### Resource Requirements
- BaseNode: Low CPU/memory, persistent storage for state
- SuperNode: Moderate CPU/memory, bandwidth for relay
- Client/Exit: Minimal resources, WireGuard kernel module

### High Availability
- Deploy multiple SuperNodes per region
- Use load balancers for SuperNode discovery
- Implement BaseNode clustering for critical deployments
- Monitor and auto-restart failed components