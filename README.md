# Go Laravel Long-Polling Service

High-performance long-polling service built with Go, designed to work with Laravel applications.

## Features

- **JWT Authentication**: Secure token-based authentication for clients
- **Redis Integration**: Real-time event notifications via Redis pub/sub
- **Worker Pool**: Concurrent request handling with configurable worker limits
- **Long-Polling**: Efficient long-polling with configurable timeout
- **Structured Logging**: JSON or text logging with configurable levels
- **Dependency Injection**: Built with uber.FX for clean architecture

## Prerequisites

- Go 1.21 or higher
- Redis server
- Laravel application with long-polling package installed

## Installation

1. Clone the repository
2. Copy `.env.example` to `.env` and configure
3. Install dependencies:
```bash
make deps
```

## Configuration

All configuration is done via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `LARAVEL_ADDR` | Laravel application URL | `http://localhost:8000` |
| `HTTP_ADDR` | HTTP server bind address | `:8085` |
| `HTTP_READ_TIMEOUT` | HTTP read timeout | `30s` |
| `HTTP_WRITE_TIMEOUT` | HTTP write timeout | `30s` |
| `JWT_SECRET` | JWT signing secret | Required |
| `JWT_EXPIRES_IN` | JWT expiration in seconds | `3600` |
| `JWT_ALGO` | JWT algorithm (HS256/HS384/HS512) | `HS256` |
| `REDIS_ADDR` | Redis server address | `redis:6379` |
| `REDIS_DB` | Redis database number | `0` |
| `REDIS_PASSWORD` | Redis password | Empty |
| `REDIS_CHANNEL` | Redis channel for events | `longpoll:events` |
| `POLL_TIMEOUT` | Long-polling timeout | `25s` |
| `ACCESS_TOKEN_SECRET` | Shared secret with Laravel | Required |
| `LOG_LEVEL` | Log level (debug/info/warn/error) | `info` |
| `LOG_FORMAT` | Log format (json/text) | `json` |
| `LARAVEL_UPSTREAM_WORKERS` | Max concurrent Laravel requests | `15` |
| `MAX_LIMIT` | Max events per request | `100` |

## Running

### Local Development

```bash
make run
```

### With Docker

```bash
make docker-build
make docker-run
```

### With Docker Compose (from project root)

```bash
docker-compose up longpoll-server
```

## API Endpoints

### POST /getAccessToken

Generate a JWT token for a channel.

**Query Parameters:**
- `channel_id` (required): Channel identifier
- `secret` (required): Shared secret for authentication

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### GET /getUpdates

Get updates for a channel (long-polling).

**Query Parameters:**
- `token` (required): JWT token
- `offset` (optional): Last event ID (default: 0)
- `limit` (optional): Max events to return (default: 100, max: MAX_LIMIT)

**Response:**
```json
{
  "events": [
    {
      "id": 1,
      "event": {"type": "message", "data": "..."},
      "created_at": 1699876543
    }
  ]
}
```

### GET /health

Health check endpoint.

**Response:**
```json
{
  "status": "ok"
}
```

## License

MIT
