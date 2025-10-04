# Testing Guide

## Overview

This guide covers testing strategies for myDvpn components including unit tests, integration tests, and end-to-end scenarios.

## Test Categories

### Unit Tests

Each package includes unit tests for core functionality:

```bash
# Run all unit tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./utils/
go test ./base/server/
go test ./super/server/
```

Key areas covered:
- Cryptographic operations (signature verification)
- IP allocation algorithms
- WireGuard configuration parsing
- Message serialization/deserialization

### Integration Tests

Integration tests verify component interactions:

```bash
# Run full integration test
./scripts/test.sh

# Manual integration testing
./scripts/integration-test.sh
```

Test scenarios:
- BaseNode + SuperNode registration
- Client + SuperNode persistent streams
- Exit peer allocation and setup
- Cross-region exit requests

### End-to-End Tests

Full system tests simulating real-world scenarios:

```bash
# Run E2E test suite
./scripts/e2e-test.sh
```

Scenarios covered:
- Complete client-to-exit connection flow
- Relay setup and traffic forwarding
- Failover and reconnection handling
- Multi-region deployments

## Test Environments

### Local Development

Use the included test script for local development:

```bash
./scripts/test.sh
```

This starts all components locally and demonstrates basic functionality.

### Docker Environment

For isolated testing:

```bash
# Build containers
docker-compose -f docker/test-compose.yml build

# Run test environment  
docker-compose -f docker/test-compose.yml up
```

### Virtual Network Testing

For advanced network simulation:

```bash
# Create network namespaces
./scripts/setup-test-network.sh

# Run tests in isolated environment
./scripts/test-in-netns.sh
```

## Test Data and Fixtures

### Mock Data

Test fixtures are provided for:
- Sample peer configurations
- Pre-generated key pairs
- Network topology definitions

Located in `testdata/` directories within each package.

### Test Keys

Test Ed25519 key pairs for development (DO NOT use in production):

```
# Client test key
Private: MC4CAQAwBQYDK2VwBCIEIJ...
Public: MCowBQYDK2VwAyEAJ...

# Exit peer test key  
Private: MC4CAQAwBQYDK2VwBCIEIF...
Public: MCowBQYDK2VwAyEAX...
```

## Performance Testing

### Load Testing

Test SuperNode capacity:

```bash
# Start SuperNode
./bin/supernode --id=test-sn --region=test --listen=0.0.0.0:50052

# Run load test
go run ./test/load/stream-test.go --target=localhost:50052 --clients=1000
```

### Stress Testing

Test system behavior under stress:

```bash
# Memory stress test
go run ./test/stress/memory-test.go

# Network stress test  
go run ./test/stress/network-test.go
```

### Benchmark Tests

Measure component performance:

```bash
# Run benchmarks
go test -bench=. ./...

# Specific benchmark
go test -bench=BenchmarkStreamManager ./super/server/
```

## Security Testing

### Authentication Tests

Verify signature verification:

```bash
go test -run TestAuthVerification ./super/server/
```

### Replay Attack Tests

Test nonce and timestamp validation:

```bash
go test -run TestReplayProtection ./...
```

### Rate Limiting Tests

Verify rate limiting works:

```bash
go test -run TestRateLimit ./...
```

## Chaos Testing

### Network Partitions

Simulate network failures:

```bash
# Start components
./scripts/test.sh

# In another terminal, simulate partition
sudo iptables -A INPUT -p tcp --dport 50052 -j DROP
sleep 30
sudo iptables -D INPUT -p tcp --dport 50052 -j DROP
```

### Component Failures

Test failure scenarios:

```bash
# Kill BaseNode during operation
pkill -f basenode

# Observe SuperNode behavior
tail -f logs/supernode-*.log
```

### Resource Exhaustion

Test resource limits:

```bash
# Memory exhaustion
stress --vm 1 --vm-bytes 1G --timeout 60s

# File descriptor exhaustion  
ulimit -n 100
./bin/supernode &
```

## Test Utilities

### Network Utilities

Helper scripts for network testing:

```bash
# Check connectivity
./scripts/check-connectivity.sh

# Monitor traffic
./scripts/monitor-traffic.sh wg-test

# Verify routing
./scripts/check-routes.sh
```

### Debug Utilities

Tools for debugging test failures:

```bash
# Dump component state
curl localhost:8080/debug/state

# Export metrics
curl localhost:8080/metrics

# Generate debug report
./scripts/debug-report.sh
```

## Continuous Integration

### GitHub Actions

Automated testing on pull requests:

```yaml
# .github/workflows/test.yml
name: Test
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: 1.21
      - run: go test ./...
      - run: ./scripts/integration-test.sh
```

### Test Reports

Generate test reports:

```bash
# Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Test report with timing
go test -v ./... 2>&1 | tee test-report.txt
```

## Troubleshooting Tests

### Common Issues

**Permission denied errors:**
```bash
# Run with sudo for network operations
sudo ./scripts/test.sh
```

**Port conflicts:**
```bash
# Check for processes using ports
sudo netstat -tulpn | grep :50051
```

**WireGuard module not loaded:**
```bash
# Load WireGuard module
sudo modprobe wireguard
```

### Debug Mode

Enable debug logging for troubleshooting:

```bash
./bin/supernode --log-level=debug
```

### Test Cleanup

Clean up test artifacts:

```bash
# Remove test interfaces
sudo ip link delete wg-test 2>/dev/null || true

# Clean iptables rules
sudo iptables -F
sudo iptables -t nat -F

# Kill test processes
pkill -f myDvpn
```

## Writing New Tests

### Test Structure

Follow Go testing conventions:

```go
func TestFeatureName(t *testing.T) {
    // Setup
    
    // Execute
    
    // Assert
    
    // Cleanup
}
```

### Test Helpers

Use provided test utilities:

```go
// Create test SuperNode
sn := testutil.NewTestSuperNode(t)
defer sn.Stop()

// Generate test keys
keys := testutil.GenerateTestKeys(t)

// Setup test network
net := testutil.SetupTestNetwork(t)
defer net.Cleanup()
```

### Mock Objects

Use mocks for external dependencies:

```go
// Mock BaseNode client
mockClient := &MockBaseNodeClient{}
mockClient.On("RegisterSuperNode", mock.Anything).Return(success, nil)
```