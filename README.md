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

The project is ready for adding distributed systems capabilities:
- Transaction support
- Replication
- Recovery
- Clustering
