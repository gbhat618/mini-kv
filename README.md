# Mini-KV

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

## Commands

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

## Development

The project is ready for adding distributed systems capabilities:
- Transaction support
- Replication
- Recovery
- Clustering
