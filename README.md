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

## Quick Start

### Start Server
```bash
# Default (localhost:6379)
./bin/server

# Custom host and port (bind to all interfaces)
./bin/server -host 0.0.0.0 -port 6379

# With config file
./bin/server -config /path/to/config.ini

# With management console
./bin/server -console -console-port 8080
```

### Use CLI
```bash
# Connect to local server (default localhost:6379)
./bin/cli PING
./bin/cli GET mykey
./bin/cli SET mykey myvalue

# Connect to remote server
./bin/cli -h 192.168.1.100 -p 6380 GET key
./bin/cli --host server.example.com --port 6379 SET key value

# With config file
./bin/cli -config /path/to/cli.ini GET key

# Short flags
./bin/cli -h localhost -p 6379 PING
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

## Configuration

### Config File (INI format)

**Server config (`server/mini-kv.ini`):**
```ini
[server]
host = 0.0.0.0
port = 6379

[security]
password = yourpassword
tls_cert = /path/to/cert.pem
tls_key = /path/to/key.pem

[limits]
max_key_size = 256
max_value_size = 10485760
max_connections = 100
read_timeout = 30

[cluster]
enabled = true
peers = localhost:16380,localhost:16381
node_id = my-node-1
gossip_port = 16379

[console]
enabled = true
port = 8080
```

**CLI config (`cli/cli.ini`):**
```ini
[server]
host = localhost
port = 6379

[security]
password = yourpassword

[tls]
enabled = true
skip_verify = true
```

### CLI Flags

**Server:**
- `-host` - Server host (default: localhost, use 0.0.0.0 for all interfaces)
- `-port` - Server port (default: 6379)
- `-config` - Config file path
- `-password` - Authentication password
- `-tls-cert` - TLS certificate file
- `-tls-key` - TLS key file
- `-max-key-size` - Maximum key size (default: 256)
- `-max-value-size` - Maximum value size (default: 10MB)
- `-max-connections` - Maximum connections (default: 100)
- `-read-timeout` - Read timeout in seconds (default: 30)
- `-cluster` - Enable clustering
- `-peers` - Cluster peers
- `-node-id` - Cluster node ID
- `-gossip-port` - Gossip port (default: 16379)
- `-console` - Enable management console
- `-console-port` - Console port (default: 8080)

**CLI:**
- `-h, --host` - Server hostname (default: localhost)
- `-p, --port` - Server port (default: 6379)
- `-config` - Config file path
- `-password` - Authentication password
- `-tls` - Use TLS
- `-tls-skip-verify` - Skip TLS verification

## Security Features

### Authentication
```bash
./bin/server -password "yourpassword"
./bin/cli -password "yourpassword" GET key
```

### TLS/SSL
```bash
./bin/server -tls-cert cert.pem -tls-key key.pem
./bin/cli --tls GET key
```

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

## Management Console

The management console provides a real-time web UI for monitoring and managing your Mini-KV instance.

### Features
- Real-time connection count
- Real-time KV pair count
- Add/Edit/Delete keys from UI
- Live updates via WebSocket

### Usage
```bash
./bin/server -console -console-port 8080 -host 0.0.0.0
```

Then visit `http://localhost:8080` in your browser.

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
