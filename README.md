# Mini-KV

[![CI](https://github.com/gbhat618/mini-kv/actions/workflows/ci.yml/badge.svg)](https://github.com/gbhat618/mini-kv/actions/workflows/ci.yml)
[![Release](https://github.com/gbhat618/mini-kv/actions/workflows/release.yml/badge.svg)](https://github.com/gbhat618/mini-kv/actions/workflows/release.yml)
[![Coverage](./coverage_badge.svg)](https://github.com/gbhat618/mini-kv/actions/workflows/ci.yml)

A high-performance key-value store written in Go, inspired by Redis.

## Project Structure

```
mini-kv/
├── server/         # Server implementation (Go)
├── cli/            # CLI client (Go)
├── tests/          # End-to-end tests (Go)
├── docker/         # Dockerfiles
└── docker-compose.yml
```

## Supported Commands

- `GET <key>` - Get value by key
- `SET <key> <value>` - Set key-value pair
- `DEL <key>...` - Delete one or more keys
- `KEYS` - List all keys
- `PING` - Ping server
- `FLUSHDB` - Clear all keys

### Transaction Commands

- `BEGIN` - Start a transaction
- `COMMIT` - Commit the current transaction
- `ROLLBACK` - Rollback the current transaction

Transactions provide isolation: changes within a transaction are not visible to other clients until committed.

## Security Features

### Authentication
```bash
./bin/server -password "yourpassword"
./bin/cli --password "yourpassword" GET key
```

### TLS/SSL
```bash
./bin/server -tls-cert cert.pem -tls-key key.pem
./bin/cli --tls GET key
```

### Configuration Options
- `-max-key-size` - Maximum key size (default: 256 bytes)
- `-max-value-size` - Maximum value size (default: 10MB)
- `-max-connections` - Maximum concurrent connections (default: 100)
- `-read-timeout` - Connection read timeout (default: 30s)

## Clustering

### Starting a Cluster
```bash
# Start node 1
./bin/server -cluster -node-id node1 -peers localhost:16380,localhost:16381

# Start node 2  
./bin/server -cluster -node-id node2 -peers localhost:16379,localhost:16381

# Start node 3
./bin/server -cluster -node-id node3 -peers localhost:16379,localhost:16380
```

### Cluster Commands
- `CLUSTER INFO` - Show cluster information
- `CLUSTER MEMBERS` - List all cluster members
- `CLUSTER JOIN <peer>` - Join a peer node
- `CLUSTER ADDSLAVE <addr>` - Add a slave node

### Cluster Options
- `-cluster` - Enable clustering mode
- `-peers` - Comma-separated list of peer addresses
- `-node-id` - Unique node ID (auto-generated if not provided)
- `-gossip-port` - Port for inter-node gossip (default: 16379)

## Replication & Failover

### Setting Up Replication
```bash
# On master server
./bin/server -cluster

# On slave server
./bin/server -cluster
./bin/cli REPLICAOF master_host 6379
```

### Replication Commands
- `REPLICAOF <host> <port>` - Set master server (use `REPLICAOF NO ONE` to promote to master)
- `ROLE` - Show server role (master/slave)
- `INFO REPLICATION` - Show replication status
- `SYNC` - Get all data for initial sync (used by slaves)

### Failover
- Automatic health checking of master node
- Automatic failover when master becomes unreachable
- Slave promotion to master on failover

## Running

### Build and Test
```bash
docker compose build
docker compose up --abort-on-container-exit mini-kv-test
```

### Run Server Only
```bash
docker compose up mini-kv-server
```

### Use CLI
```bash
docker compose run mini-kv-cli ./cli --host mini-kv-server --port 6379 SET key value
docker compose run mini-kv-cli ./cli --host mini-kv-server --port 6379 GET key
```

### Build from Source
```bash
make build
```

## Performance Benchmarks

Run benchmarks with:
```bash
docker compose run --rm mini-kv-bench
```

### Results (10K iterations, 10 concurrent clients)
| Operation | Ops/sec  | Avg Latency | P95 Latency | P99 Latency |
|-----------|----------|-------------|-------------|--------------|
| PING      | ~40,500  | 236 us      | 610 us      | 1,498 us     |
| SET       | ~41,700  | 231 us      | 630 us      | 1,108 us     |
| GET       | ~41,700  | 232 us      | 699 us      | 1,532 us     |
| MIXED     | ~19,600  | 499 us      | 1,305 us    | 2,385 us     |

## Development

### Linting
```bash
golangci-lint run
```

### Testing
```bash
go test -v ./...
```

### Coverage
```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```
