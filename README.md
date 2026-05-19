# taskOS Server

A small, secure HTTP API for tasks, reminders, and trusted contacts who can assign tasks or ping reminders on your behalf.

Built for self-hosting on Ubuntu (or anywhere Go runs). Other apps integrate with a single API key and JSON over HTTPS.

## Quick start

```bash
go run .
```

Server listens on `:8080` by default. Requires PostgreSQL — set `TASKOS_DATABASE_URL` before starting.

## Authentication

Every protected request uses a Bearer API key:

```http
Authorization: Bearer tos_xxxxxxxx
```

Register once to receive your key (shown only at registration):

```bash
curl -s -X POST http://localhost:8080/v1/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"brandon"}'
```

Response:

```json
{
  "user": { "id": "...", "username": "brandon", "created_at": "..." },
  "api_key": "tos_...",
  "note": "Store this API key securely. It is shown only once."
}
```

## Core concepts

| Concept | Description |
|--------|-------------|
| **User** | Owns tasks and reminders; authenticates with an API key |
| **Task** | Something to do; can be assigned to a trusted contact |
| **Reminder** | A nudge with optional due time |
| **Trusted contact** | Another user you allow to assign tasks and/or ping reminders |

Trusted contacts are **one-way**: you add someone and grant permissions. They can act on **your** data if you allow it.

## API reference

Base URL: `https://your-server.example.com`

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/health` | No | Health check |
| POST | `/v1/register` | No | Create account + API key |
| GET | `/v1/me` | Yes | Current user profile |
| GET | `/v1/users/{username}` | Yes | Resolve user id by username |
| GET/POST | `/v1/tasks` | Yes | List / create your tasks |
| GET/PATCH/DELETE | `/v1/tasks/{id}` | Yes | Read / update / delete task |
| GET/POST | `/v1/reminders` | Yes | List / create reminders |
| GET/PATCH/DELETE | `/v1/reminders/{id}` | Yes | Read / update / delete reminder |
| GET | `/v1/reminders/pings` | Yes | Recent reminder pings for you |
| GET/POST | `/v1/contacts` | Yes | List / add trusted contacts |
| DELETE | `/v1/contacts/{userId}` | Yes | Remove trusted contact |
| POST | `/v1/users/{userId}/tasks` | Yes | Assign task (requires trust) |
| POST | `/v1/users/{userId}/reminders/{id}/ping` | Yes | Ping reminder (requires trust) |

### Example: create a task

```bash
curl -s -X POST http://localhost:8080/v1/tasks \
  -H "Authorization: Bearer $TASKOS_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Ship v1","due_at":"2026-05-20T17:00:00Z"}'
```

### Example: allow a friend to assign tasks

```bash
# Look up their user id
curl -s http://localhost:8080/v1/users/alice \
  -H "Authorization: Bearer $TASKOS_API_KEY"

# Grant permissions
curl -s -X POST http://localhost:8080/v1/contacts \
  -H "Authorization: Bearer $TASKOS_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","can_assign_tasks":true,"can_ping_reminders":true}'
```

### Example: friend assigns you a task

```bash
curl -s -X POST "http://localhost:8080/v1/users/$YOUR_USER_ID/tasks" \
  -H "Authorization: Bearer $ALICE_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Review PR #42"}'
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `TASKOS_ADDR` | `:8080` | Listen address |
| `TASKOS_DATABASE_URL` | `postgres://localhost:5432/taskos?sslmode=disable` | PostgreSQL connection URL |
| `TASKOS_CORS_ORIGINS` | `*` | Allowed CORS origins (comma-separated) |
| `TASKOS_RATE_LIMIT_PER_MIN` | `120` | Per-IP rate limit |
| `TASKOS_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown timeout |

Copy `.env.example` and export variables, or use systemd `EnvironmentFile`.

## PostgreSQL setup

Create a database and user on your Ubuntu server:

```sql
CREATE USER taskos WITH PASSWORD 'your-secure-password';
CREATE DATABASE taskos OWNER taskos;
GRANT ALL PRIVILEGES ON DATABASE taskos TO taskos;
```

The server runs migrations automatically on startup (requires permission to create the `citext` extension — grant superuser once, or pre-create it):

```sql
CREATE EXTENSION IF NOT EXISTS citext;
```

Set the connection URL:

```bash
export TASKOS_DATABASE_URL='postgres://taskos:your-secure-password@localhost:5432/taskos?sslmode=disable'
```

Use `sslmode=require` (or `verify-full`) when connecting over the network in production.

## Deploy on Ubuntu

```bash
go build -o taskos-server .
sudo useradd --system --no-create-home taskos
sudo cp taskos-server /usr/local/bin/

# systemd unit at /etc/systemd/system/taskos.service
```

Example `taskos.service`:

```ini
[Unit]
Description=taskOS API Server
After=network.target

[Service]
Type=simple
User=taskos
EnvironmentFile=/etc/taskos/env
ExecStart=/usr/local/bin/taskos-server
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

Put TLS in front with **Caddy** or **nginx** so clients always use HTTPS.

## Security notes

- API keys are bcrypt-hashed; only a SHA-256 lookup index is stored for fast auth
- Keys use the `tos_` prefix and are shown once at registration
- Rate limiting and request timeouts are enabled by default
- Run behind HTTPS in production; restrict `TASKOS_CORS_ORIGINS`

## Project layout

```
main.go
internal/
  config/      Environment configuration
  server/      HTTP router and lifecycle
  handlers/    REST handlers
  store/       PostgreSQL persistence
  auth/        API key generation and validation
  middleware/  Logging, CORS, rate limit, auth
  models/      Domain types
```

## License

MIT (add your preferred license)
