
# Full architecture, entity roles, and end-to-end flow — drop this to your AI agent

Alright — here’s the single-source, no-BS blueprint you can hand to your AI agent. It covers **every entity**, **their responsibilities**, exact **message flows**, failure modes, and where this maps into your repo. This is the operational spec: implement, test, ship.

---

## System overview (high level)

We’re building a decentralized VPN control plane with persistent control streams and dynamic data-plane orchestration. Key idea: **every peer (client or exit) and every SuperNode keeps an outbound persistent gRPC stream** to its supervising SuperNode so control messages and relay orchestration are instantaneous and NAT-safe. The BaseNode indexes SuperNodes per region and helps route cross-region requests.

High-level topology:

```
ClientPeer --(persistent gRPC stream & optional WG)--> LocalSuperNode --(gRPC to BaseNode)--> BaseNode
ExitPeer  --(persistent gRPC stream & WG if available)--> ExitSuperNode
LocalSuperNode <--(inter-supernode gRPC/request)--> RemoteSuperNode <--(control->ExitPeer stream)--> ExitPeer
```

Traffic paths:

* Direct path if reachable: `ClientPeer <-> ExitPeer` (WireGuard)
* Relay path if not reachable: `ClientPeer -> LocalSuperNode -> RemoteSuperNode -> ExitPeer` (WireGuard + SN NAT/relay in between)

---

## Entities & roles (who does what)

### 1) **ClientPeer (end-user device)**

* Role: Consumer of VPN service.
* Responsibilities:

  * Register with local SuperNode (one-time / on boot).
  * Open a **persistent control stream** to local SuperNode (auth, heartbeat, commands).
  * Send heartbeats / telemetry (latency, throughput).
  * Accept commands from SuperNode: `SETUP_EXIT`, `ROTATE_PEER`, `DISCONNECT`, `RELAY_UPDATE`.
  * Bring up/tear down WireGuard (data plane) when commanded.
* Files in repo: `clientPeer/*` (`client/peer.go`, `persistent_stream.go`, `utils`, `wg` helpers).

### 2) **ExitPeer (user-hosted exit node)**

* Role: Egress gateway to the internet; provides WireGuard endpoint(s).
* Responsibilities:

  * Maintain outbound persistent stream to its supervising SuperNode.
  * When instructed (via `SETUP_EXIT`), add client pubkey to WG interface, allocate allowed IP, enable forwarding/NAT.
  * Report live stats via heartbeat (via control stream).
  * Can be behind NAT; always make outbound connection to SN.
* Files: `exitpeer/*` (`exitpeer/server.go`, `utils.ConfigureWG`, `allocateIPForPeer`).

### 3) **SuperNode (regional coordinator & relay)**

* Role: regional control plane, relay point, and first line of orchestration.
* Responsibilities:

  * Accept registrations from clients & exit peers.
  * Host `PersistentControlStream` server and maintain `StreamManager` (active streams, roles, last heartbeat).
  * Respond to client requests for exit peers (`RequestExit`): query BaseNode, pick remote SN, request exit peers, orchestrate relay if needed.
  * If exit is not directly reachable, orchestrate relay by:

    * instructing exit peer (via its persistent stream) to add WG peer,
    * configuring local relay WG + NAT for the client,
    * returning WG config to client to connect to the SuperNode relay endpoint.
  * Provide admin APIs: list streams, push commands, metrics.
* Files: `super/server/super_node.go`, `super/server/stream_manager.go`, `super/utils`, `dataplane/*`.

### 4) **BaseNode**

* Role: global directory & minimal coordinator for regions.
* Responsibilities:

  * Track SuperNodes (region, IP, load).
  * Respond to `RequestExitRegion` from SuperNodes to find candidate remote SuperNodes.
* Files: `base/*` (`base/server`, `pb`, `proto`).

---

## Protocol: messages & RPC surface (contract)

Primary change: single bi-directional `PersistentControlStream(stream ControlMessage) returns (stream ControlMessage)`.

Core `ControlMessage` `oneof`:

* `AuthRequest` / `AuthResponse` — initial handshake (role, peer_id, pubkey_b64, signature, nonce, region).
* `PingRequest` / `PongResponse` — heartbeat/latency.
* `Command` / `CommandResponse` — server → peer instructions, peer → server acknowledgements.
* `InfoRequest` / `InfoResponse` — optional state sync.

Command types:

* `SETUP_EXIT` — make peer an exit for a client (payload: client_pubkey, allowed_ips, session_id)
* `ROTATE_PEER` — switch to new exit
* `RELAY_SETUP` — instruct SN dataplane to allocate relay IP & create forwarding
* `DISCONNECT` — graceful disconnection

**Security**: sign the auth payload with peer private key (signature of `peer_id||role||region||nonce`). Use TLS for transport. Optionally mTLS among SuperNodes.

---

## Full flow: function-by-function, message-by-message

This is the concrete step sequence the agent must implement: every arrow corresponds to a function or RPC.

### A. Bootstrap & registration (client & exit)

1. **ClientPeer boot** → `client.Register()` (existing RPC) to local SN: `RegisterClientPeer`.

   * Server: `RegisterClientPeer()` stores in `registeredPeers`.
2. **ExitPeer boot** → `exit peer` registers with its supervising SuperNode the same way (or uses a SuperNode registration routine).

**Goal**: SN knows the peer id, region, IP (for gRPC), WG public keys.

---

### B. Open persistent stream (control channel)

1. ClientPeer calls `PersistentControlStream()` (client-side stream creation).

   * Client: `PersistentStreamManager.connect()` → `stream := client.PersistentControlStream(ctx)` → `authenticate()`.
   * Send `AuthRequest`.
2. SN receives new stream in `PersistentControlStream()` handler → constructs `PersistentClientStream` object and calls `handle()`:

   * `PersistentClientStream.handle()` reads first message (must be `AuthRequest`), validates signature, sets `peerID`, `role`, registers stream in `StreamManager.registerStream(peerID, stream)`.
3. On authentication success, SN replies `AuthResponse{success:true}` back on the stream.

**Where in code**: client: `PersistentStreamManager.connect()` / `authenticate()`; server: `PersistentClientStream.handleAuthRequest()` / `sm.registerStream()`.

---

### C. Ongoing heartbeats & stream keepalive

* Client: every 10s `sendPing()` (ControlMessage PingRequest).
* Server: `handlePing()` → update `lastHeartbeat` and reply `PongResponse`.
* StreamManager periodically `cleanupStaleConnections()` and removes streams with no heartbeat for >30s.

**Where**: `PersistentStreamManager.startHeartbeat()` and server `PersistentClientStream.handlePing()`; cleanup in `StreamManager.cleanupStaleConnections()`.

---

### D. Client requests an exit (normal user action)

Function: `ClientPeer.RequestExitEndpoint(region, minBW, maxLatency)`.

1. Client makes RPC to local SN: `RequestExit(ctx, req)` OR sends a Control Command depending on migration state.
2. SN server `RequestExit()`:

   * Validate client is registered.
   * Create `ExitRegionRequest` and call `s.baseClient.RequestExitRegion(...)` to get remote SuperNodes for region.
3. SN selects one or more remote SuperNodes (or picks local candidates).
4. SN dials chosen RemoteSuperNode(s) (gRPC) and calls RemoteSN.RequestExitPeer(req):

   * RemoteSN.RequestExitPeer will search its `registeredPeers` for matching Exit peers and **return a list** of `ExitPeerResponse` objects (this is the multi-candidate refactor you implemented).
5. Local SN receives candidate list, applies ranking (latency/bandwidth/health) and picks best candidate(s).

**Where**: `super/server/super_node.go:RequestExit()` and `RequestExitPeer()`.

---

### E. How SuperNode connects to the ExitPeer (two cases)

#### Case 1 — Direct connect possible (client can reach exit peer)

1. SN returns `WireguardConfig` to client with ExitPeer endpoint: client configures WG and traffic flows directly.

#### Case 2 — ExitPeer only outbound (behind NAT) OR client cannot reach exit

**We rely on persistent streams and command channel**:

1. SN (RemoteSnow) finds an ExitPeer that has an active persistent stream (ExitPeer is connected).
2. SN constructs `SETUP_EXIT` command with `client_pubkey`, `allowed_ips`, `session_id`.
3. SN calls `StreamManager.SendCommandToPeer(exitPeerID, command)` — which queues the command for ExitPeer on its outbound stream that is already open.
4. ExitPeer `PersistentStream` receives `Command`:

   * `handleSetupExit()` adds the peer to its local WireGuard interface using `wgctrl` (or netlink).
   * ExitPeer replies with `CommandResponse(success)` including any assigned endpoint info.
5. If traffic must be relayed through SuperNodes (because client can't reach exit directly), SN orchestrates relay:

   * SN-A (local to client) issues `RELAY_SETUP` to its dataplane manager: allocate relay IP, add client WG peer on SN-A, set NAT rules, configure forwarding.
   * SN-B (exit’s SN) tells ExitPeer to accept the client (ExitPeer adds the client as WG peer but expects traffic from SN-A relay IP).
6. SN-A returns to client a `WireguardConfig` pointing **to SN-A relay endpoint** (or instructs client to form WG with SN-A). Client connects to SN-A; SN-A forwards traffic to SN-B’s ExitPeer over SN-B’s persistent stream / or over a WG tunnel from SN-A to SN-B depending on topology.

**Where**: `StreamManager.SendCommandToPeer()`, `PersistentClientStream.commandSender()` (server), `PersistentStreamManager.handleSetupExit()` (client/exit).

---

### F. Relay dataplane setup (detailed)

* **Allocation**: SN-A uses a thread-safe pool to assign `10.200.x.y/32` to client.
* **WireGuard on SN-A**:

  * `EnsureInterface("wg-relay")`
  * Add client as WG peer: `PublicKey=client_pubkey, AllowedIPs=client_relay_ip`
  * SN-A may create a point-to-point WG to SN-B (if direct SN-to-SN path is required) or forward via exitPeer's persistent outbound path (control-plane tunneling + data forwarding).
* **NAT**:

  * `sysctl net.ipv4.ip_forward=1`
  * iptables: `-t nat -A POSTROUTING -s <client_relay_ip> -o <public_iface> -j MASQUERADE` or SN-A -> SN-B forwarding rules.
* **Traffic flow**: `Client -> WG(SN-A) -> SN-A forwards to SN-B (or ExitPeer) -> ExitPeer forwards to internet`.
* **Cleanup**: Remove WG peer and iptables when session ends or command id is revoked.

**Where**: Dataplane helpers `dataplane/wg.go`, `dataplane/relay.go`.

---

### G. Rotation & failover

* SN monitors exit peer health via heartbeats and WG stats.
* If exit peer fails:

  * SN picks next candidate (from list previously received or by querying again).
  * SN sends `ROTATE_PEER` to the client stream with the new config.
  * Client performs graceful switchover: bring up new peer, then remove old peer.

**Where**: `StreamManager` + `SuperNodeServer.RequestExit` + client `handleRotatePeer`.

---

### H. Shutdown & cleanup

* When client disconnects or session ends:

  * SN sends `DISCONNECT` to exit peer and to client (if applicable).
  * ExitPeer removes WG peer and frees allocated IP.
  * SN removes relay NAT rules and marks session terminated.
  * Persisted state (applied command ids) is kept for idempotence.

---

## Failure modes & recovery (explicit)

* **Auth failed**: server returns `AuthResponse{success:false}` and closes stream.
* **Network drop**: client reconnects with exponential backoff; on reconnect, client resends status/info to resync.
* **Partial relay failure**: SN attempts rollback — remove WG peer, delete NAT, inform client of failure.
* **Command timeout**: if `CommandResponse` not received within X seconds, mark `command_failures_total++`, retry up to N times, then escalate.
* **Duplicate commands**: commands carry `command_id` — handlers must be idempotent (check applied command ids store).
* **Resource exhaustion on SN**: reject new streams or return "busy" on registration. Expose `active_streams_total` metric and `max_capacity` heuristic.

---

## Observability & metrics to export (must)

* `active_streams_total` (gauge)
* `stream_auth_failures_total`
* `peer_heartbeat_latency_ms` (histogram)
* `command_latency_seconds` (histogram)
* `command_success_total`, `command_failure_total`
* `wg_peers_count` per SN
* `relay_sessions_active`
* `supernode_cpu_pct`, `supernode_net_tx_mbps`, `supernode_net_rx_mbps`

Add structured logging with: `peer_id`, `command_id`, `session_id`, `region`.

---

## Security checklist (must complete)

* Use TLS for gRPC servers (cert rotation).
* Verify signatures in `AuthRequest` and reject forged attempts.
* Nonce + timestamp for replay protection.
* Rate limit auth attempts by IP / peer id.
* CLI for admins to revoke a peer or blacklist a SuperNode.

---

## Repo mapping — where to modify (practical)

Use the tree you shared. Tasks map to files:

* Proto:

  * `base/proto/*.proto`
  * `clientPeer/proto/super_node.proto` (update `PersistentControlStream`)
  * Run `protoc` to generate pb files (client & server).
* Client:

  * `clientPeer/client/persistent_stream.go` (already exists — extend handlers to real WG ops).
  * `clientPeer/client/peer.go` (call `Start()` on boot).
* ExitPeer:

  * `exitpeer/server.go` → implement `handleSetupExit` to call `dataplane/wg.go`.
* Super:

  * `super/server/super_node.go` → hook `PersistentControlStream` into `StreamManager`.
  * `super/server/stream_manager.go` (you have skeleton) → extend `registerStream`, `SendCommandToPeer`, role storage.
  * `super/dataplane/wg.go` & `super/dataplane/relay.go` (new).
* Base:

  * `base/server` → ensure `RequestExitRegion` returns candidate SNs.
* Utilities:

  * `utils/ip.go`, `wgkeys.go`, `wgutil.go` (existing) → centralize WG remote add/remove.

---

## Acceptance tests (for your AI agent to run)

* **Unit**: auth verification, Ping/Pong, command marshalling.
* **Integration**: start SN + two client peers + exit peer behind NAT (simulate), run full exit request flow with relay, assert traffic path (ping/iperf) works.
* **Chaos**: drop mid-command, ensure rollback & logging.
* **Load**: simulate 1000 streams with no data-plane to measure FD/CPU/RAM.

---

## Deliverable checklist for AI agent

1. Updated proto + generated pb files.
2. Server `PersistentControlStream` implemented + `StreamManager`.
3. Client `PersistentStreamManager` integrated and started on boot.
4. `dataplane/wg.go` and `dataplane/relay.go` with real `wgctrl` calls or a tested fallback.
5. Command handlers (`SETUP_EXIT, ROTATE_PEER, RELAY_SETUP, DISCONNECT`) implemented and idempotent.
6. Integration tests and scripts to reproduce: registration → request exit → relay → cleanup.
7. Metrics (`/metrics`) and runbook for operators.
8. Docs: `docs/architecture.md`, `docs/runbook.md`, `docs/testing.md`.

---

## Example minimal sequence (as a narrative the AI agent can run)

1. Start BaseNode, register two SuperNodes (A, B) in different regions.
2. Start ExitPeer process; it registers to SuperNode B and opens persistent stream.
3. Start ClientPeer process; it registers to SuperNode A and opens persistent stream.
4. Client requests exit region of B.
5. SuperNode A queries BaseNode → gets SN B.
6. SN A calls `RequestExitPeer` on SN B.
7. SN B chooses exit peer, pushes `SETUP_EXIT` to that exit via its persistent stream.
8. ExitPeer applies WG peer and replies success.
9. SN A determines client cannot reach exit (simulate) → sets up relay: `RELAY_SETUP` on SN A dataplane & instructs exit to accept client (if needed).
10. SN A returns WG config to client (endpoint = SN A relay IP).
11. Client applies WG and traffic flows through SN A -> SN B -> ExitPeer.

---

If you want, I’ll now generate a **single README file** (ready to commit) that includes this exact spec + the `proto` snippet + a step-by-step test script (bash) to run a simulated relay demo locally. Want that?
