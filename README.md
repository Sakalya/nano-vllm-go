# nano-vllm-go

A lightweight Go load-balancing proxy for [nano-vllm](https://github.com/GeeeekExplorer/nano-vllm) that exposes an OpenAI-compatible HTTP API.

## Architecture

```
Client
  │
  ▼
Go Proxy (this repo)          ← OpenAI-compatible API
  │  load balancer
  ├──▶ Python backend :9000   ← wraps nano-vllm, one process per GPU
  └──▶ Python backend :9001
```

The Go proxy handles routing, load balancing, and health checking. Each Python backend (`backend/server.py`) wraps a single nano-vllm `LLM` instance and serves a simple `/generate` endpoint.

## Features

- OpenAI-compatible endpoints: `/v1/chat/completions`, `/v1/completions`, `/v1/models`
- Load balancing strategies: **round-robin** and **least-connections**
- Periodic health checks with automatic backend removal and recovery
- Configurable connection pool per backend
- Graceful shutdown on `SIGINT`/`SIGTERM`

## Requirements

- Go 1.22+
- Python 3.10+ with [nano-vllm](https://github.com/GeeeekExplorer/nano-vllm) installed

## Quick Start

**1. Start the Python backends** (one per GPU):

```bash
python backend/server.py --model /path/to/model --port 9000
python backend/server.py --model /path/to/model --port 9001
```

**2. Build and run the Go proxy:**

```bash
make run
```

Or manually:

```bash
go build -o bin/server ./cmd/server
./bin/server -config config.yaml
```

The proxy starts on `0.0.0.0:8080` by default.

## Configuration

Edit `config.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 300s

backends:
  - url: "http://localhost:9000"
    weight: 1
  - url: "http://localhost:9001"
    weight: 1

load_balancer:
  strategy: "least_conn"   # round_robin | least_conn

health_check:
  interval: 10s
  timeout: 5s
  path: "/health"

pool:
  max_idle_conns_per_host: 10
  max_conns_per_host: 100
  idle_conn_timeout: 90s
```

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Proxy health + backend counts |
| `GET` | `/v1/models` | List available models |
| `POST` | `/v1/completions` | Text completion |
| `POST` | `/v1/chat/completions` | Chat completion (ChatML format) |

### Example

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nano-vllm",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 256
  }'
```

## Project Structure

```
cmd/server/         # entrypoint
internal/
  api/              # HTTP handlers and request/response types
  backend/          # backend pool and health checking
  balancer/         # round-robin and least-connections strategies
  config/           # YAML config loading
backend/server.py   # Python nano-vllm wrapper (one per GPU)
config.yaml         # default configuration
```
