# myDvpn Operations Runbook

## Quick Start

### 1. Build Components
```bash
./scripts/build.sh
```

### 2. Run Integration Test
```bash
./scripts/test.sh
```

This starts all components and demonstrates a full client-to-exit connection flow.

## Production Deployment

### Prerequisites

- Linux servers with root access
- WireGuard kernel module loaded (`modprobe wireguard`)
- iptables for NAT/forwarding rules
- Go 1.21+ for building components

### Deployment Architecture

```
BaseNode (Global)
├── SuperNode-East (us-east-1)
│   ├── Client Peers
│   └── Relay Interfaces
└── SuperNode-West (us-west-1)
    ├── Exit Peers
    └── Relay Interfaces
```

### Step 1: Deploy BaseNode

**Single Instance:**
```bash
# On central server
./bin/basenode \
  --listen=0.0.0.0:50051 \
  --log-level=info
```

**With Load Balancer:**
```bash
# Behind HAProxy/nginx
./bin/basenode \
  --listen=127.0.0.1:50051 \
  --log-level=info
```

### Step 2: Deploy SuperNodes

**Regional SuperNode:**
```bash
# us-east-1 SuperNode
./bin/supernode \
  --id=sn-east-1 \
  --region=us-east-1 \
  --listen=0.0.0.0:50052 \
  --basenode=basenode.example.com:50051 \
  --log-level=info

# us-west-1 SuperNode  
./bin/supernode \
  --id=sn-west-1 \
  --region=us-west-1 \
  --listen=0.0.0.0:50052 \
  --basenode=basenode.example.com:50051 \
  --log-level=info
```

### Step 3: Deploy Exit Peers

```bash
# Exit peer in us-west-1
./bin/exitpeer \
  --id=exit-usw1-001 \
  --region=us-west-1 \
  --supernode=sn-west-1.example.com:50052 \
  --port=51820 \
  --log-level=info
```

### Step 4: Configure Clients

```bash
# Client in us-east-1
./bin/client \
  --id=client-001 \
  --region=us-east-1 \
  --supernode=sn-east-1.example.com:50052 \
  --log-level=info
```

## Configuration Management

### Environment Variables

All components support environment variable configuration:

```bash
export MYDVPN_LOG_LEVEL=info
export MYDVPN_BASENODE_ADDR=basenode.example.com:50051
export MYDVPN_REGION=us-east-1
```

### Configuration Files

Create config files in `/etc/mydvpn/`:

```yaml
# /etc/mydvpn/supernode.yml
id: sn-east-1
region: us-east-1
listen_addr: 0.0.0.0:50052
basenode_addr: basenode.example.com:50051
log_level: info
```

### TLS Configuration

Enable TLS for production:

```bash
# Generate certificates
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365

# Start with TLS
./bin/supernode \
  --tls-cert=cert.pem \
  --tls-key=key.pem \
  --listen=0.0.0.0:50052
```

## Service Management

### Systemd Service Files

**BaseNode Service:**
```ini
# /etc/systemd/system/mydvpn-basenode.service
[Unit]
Description=myDvpn BaseNode
After=network.target

[Service]
Type=simple
User=mydvpn
ExecStart=/usr/local/bin/basenode --listen=0.0.0.0:50051
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

**SuperNode Service:**
```ini
# /etc/systemd/system/mydvpn-supernode.service
[Unit]
Description=myDvpn SuperNode
After=network.target

[Service]
Type=simple
User=mydvpn
ExecStart=/usr/local/bin/supernode \
  --id=sn-east-1 \
  --region=us-east-1 \
  --listen=0.0.0.0:50052 \
  --basenode=basenode.example.com:50051
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

**Enable Services:**
```bash
sudo systemctl enable mydvpn-basenode
sudo systemctl enable mydvpn-supernode
sudo systemctl start mydvpn-basenode
sudo systemctl start mydvpn-supernode
```

## Monitoring

### Health Checks

```bash
# Check component status
curl http://localhost:8080/health

# Check active streams
curl http://localhost:8080/debug/streams

# Export metrics
curl http://localhost:8080/metrics
```

### Prometheus Configuration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'mydvpn-basenode'
    static_configs:
      - targets: ['basenode:8080']
  
  - job_name: 'mydvpn-supernodes'
    static_configs:
      - targets: ['sn-east-1:8080', 'sn-west-1:8080']
```

### Grafana Dashboard

Key metrics to monitor:
- `mydvpn_active_streams_total`
- `mydvpn_commands_processed_total`
- `mydvpn_wg_peers_count`
- `mydvpn_stream_auth_failures_total`

### Log Aggregation

Use structured logging with ELK stack:

```bash
# Configure rsyslog to forward to Elasticsearch
./bin/supernode --log-format=json | \
  filebeat -c filebeat.yml
```

## Troubleshooting

### Common Issues

**Stream Connection Failures:**
```bash
# Check network connectivity
telnet supernode.example.com 50052

# Check certificate validity
openssl s_client -connect supernode.example.com:50052

# Check logs
tail -f /var/log/mydvpn/supernode.log
```

**WireGuard Interface Issues:**
```bash
# Check WireGuard status
sudo wg show

# Check interface configuration
ip addr show wg-relay-*

# Check routing table
ip route show table all
```

**NAT/Firewall Issues:**
```bash
# Check iptables rules
sudo iptables -L -n -v
sudo iptables -t nat -L -n -v

# Test connectivity
nc -u target-ip 51820

# Check kernel modules
lsmod | grep wireguard
```

### Debug Mode

Enable debug logging:
```bash
./bin/supernode --log-level=debug
```

### Performance Issues

**High CPU Usage:**
```bash
# Check process stats
top -p $(pgrep supernode)

# Profile Go application
go tool pprof http://localhost:8080/debug/pprof/profile

# Check system resources
iostat -x 1
```

**Memory Leaks:**
```bash
# Monitor memory usage
go tool pprof http://localhost:8080/debug/pprof/heap

# Check for goroutine leaks
go tool pprof http://localhost:8080/debug/pprof/goroutine
```

### Network Debugging

**Packet Capture:**
```bash
# Capture WireGuard traffic
sudo tcpdump -i any udp port 51820

# Capture control traffic
sudo tcpdump -i any tcp port 50052

# Monitor interface traffic
sudo tcpdump -i wg-relay-sn-east-1
```

**Connectivity Testing:**
```bash
# Test end-to-end connectivity
ping -I wg-client-001 8.8.8.8

# Check route to exit
traceroute -i wg-client-001 8.8.8.8

# Verify NAT configuration
sudo iptables -t nat -L POSTROUTING -n -v
```

## Backup and Recovery

### Configuration Backup

```bash
# Backup configurations
tar -czf mydvpn-config-$(date +%Y%m%d).tar.gz \
  /etc/mydvpn/ \
  /etc/systemd/system/mydvpn-*
```

### State Recovery

```bash
# BaseNode state recovery
# (BaseNode is stateless in current implementation)

# SuperNode recovery
# Restart and peers will reconnect automatically

# Clean restart after failure
sudo systemctl stop mydvpn-supernode
sudo ip link delete wg-relay-* 2>/dev/null || true
sudo systemctl start mydvpn-supernode
```

## Security

### Access Control

```bash
# Firewall rules for SuperNode
sudo ufw allow 50052/tcp  # gRPC control plane
sudo ufw allow 51820/udp  # WireGuard data plane

# Restrict admin endpoints
sudo ufw deny 8080/tcp
# Or bind to localhost only
```

### Certificate Management

```bash
# Rotate TLS certificates
./scripts/rotate-certs.sh

# Update peers with new certificates
systemctl reload mydvpn-supernode
```

### Peer Management

```bash
# Revoke peer access
curl -X POST http://localhost:8080/admin/revoke-peer \
  -d '{"peer_id":"client-001"}'

# List active peers
curl http://localhost:8080/admin/peers
```

## Scaling

### Horizontal Scaling

**Multiple SuperNodes per Region:**
```bash
# Load balancer configuration
upstream supernode-east {
    server sn-east-1:50052;
    server sn-east-2:50052;
}
```

**BaseNode Clustering:**
```bash
# Use external consensus system
etcd --initial-cluster=node1=http://10.0.1.1:2379,node2=http://10.0.1.2:2379
```

### Performance Tuning

**OS Tuning:**
```bash
# Increase file descriptor limits
echo 'mydvpn soft nofile 65536' >> /etc/security/limits.conf
echo 'mydvpn hard nofile 65536' >> /etc/security/limits.conf

# Optimize network buffers
echo 'net.core.rmem_max = 134217728' >> /etc/sysctl.conf
echo 'net.core.wmem_max = 134217728' >> /etc/sysctl.conf
```

**Application Tuning:**
```bash
# Increase Go garbage collection target
export GOGC=400

# Tune gRPC settings
export GRPC_GO_MAX_CONNECTION_IDLE=30s
```

## Maintenance

### Regular Tasks

**Daily:**
- Check service status
- Monitor disk usage
- Review error logs

**Weekly:**
- Update dependencies
- Restart services (rolling)
- Clean old log files

**Monthly:**
- Security patching
- Certificate rotation
- Capacity planning review

### Upgrade Procedure

```bash
# 1. Build new version
./scripts/build.sh

# 2. Rolling upgrade SuperNodes
systemctl stop mydvpn-supernode
cp bin/supernode /usr/local/bin/
systemctl start mydvpn-supernode

# 3. Verify functionality
./scripts/health-check.sh

# 4. Update BaseNode (brief downtime)
systemctl stop mydvpn-basenode
cp bin/basenode /usr/local/bin/
systemctl start mydvpn-basenode
```