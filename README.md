# taskOS

> Your tasks. Your server. Your rules.

taskOS is a self-hosted task management server. You run it on your own machine — your phone, a VPS, a Raspberry Pi — and it pushes tasks, reminders, and TODOs directly to you. No third-party cloud. No data leaving your hands.

Built with Go. Designed to be simple to deploy and easy to build on.

---

## Why taskOS?

Most task managers store your data on someone else's server. taskOS flips that: **you are the server**. Tasks come to you via push — you get pinged when something needs your attention, rather than having to remember to check an app.

It's personal-first, but built for sharing. You can send tasks to friends, delegate to colleagues, and collaborate — all server-to-server, without a central authority in the middle.

---

## Features

- **Self-hosted** — run your own instance, own your data completely
- **Push model** — get pinged on tasks and reminders rather than polling
- **Sharing & collaboration** — send tasks to other taskOS users
- **REST API** — designed as a clean API backend; clients (mobile, web, CLI) connect to it
- **Auth & user accounts** — secure access to your server
- **Tasks, reminders & due dates** — the full todo lifecycle

---

## Architecture

taskOS is a REST API server written in Go, backed by PostgreSQL.

```
[Your Device / VPS]
  └── taskOS server (Go + Postgres)
        ├── REST API  ←──  mobile app / web UI / CLI
        └── push notifications / pings  ──→  connected clients
```

Each user hosts their own instance. When sharing tasks with another user, servers communicate directly.

---

## Tech Stack

| Layer    | Technology        |
|----------|-------------------|
| Language | Go                |
| Database | PostgreSQL        |
| API      | REST (JSON)       |

---

## Status

🚧 **Early planning / pre-development.** The server-side is being designed first.

Planned client interfaces:
- [ ] Mobile app
- [ ] Web UI
- [ ] CLI

---

## Getting Started

### Prerequisites

- Go 1.22+ (developed against 1.26)
- Docker + Docker Compose (for the bundled Postgres)
- `make` (optional, but the targets are tiny — see `Makefile`)

### Run it

```bash
git clone https://github.com/brandonthis-that/taskOS-server
cd taskOS-server

cp .env.example .env
# edit .env: set DB_USER / DB_PASSWORD / DB_NAME

make up         # start Postgres
make run        # start the API server on :8080
```

Sanity check:

```bash
curl -s localhost:8080/healthz
# ok
```

### Try the API

```bash
# 1. Sign up
TOKEN=$(curl -s -X POST localhost:8080/api/auth/signup \
  -H 'content-type: application/json' \
  -d '{"email":"me@example.com","password":"hunter2hunter2"}' | jq -r .token)

# 2. Create a task
curl -s -X POST localhost:8080/api/tasks \
  -H "authorization: Bearer $TOKEN" \
  -H 'content-type: application/json' \
  -d '{"title":"Buy milk","due_at":"2026-06-01T09:00:00Z"}' | jq

# 3. List your tasks
curl -s localhost:8080/api/tasks -H "authorization: Bearer $TOKEN" | jq
```

### API

| Method   | Path                  | Auth | Description                             |
|----------|-----------------------|:----:|-----------------------------------------|
| `GET`    | `/healthz`            |  —   | Liveness + DB ping                      |
| `POST`   | `/api/auth/signup`    |  —   | Create account, returns bearer token    |
| `POST`   | `/api/auth/login`     |  —   | Exchange credentials for a bearer token |
| `POST`   | `/api/auth/logout`    |  ✓   | Revoke current token                    |
| `GET`    | `/api/auth/me`        |  ✓   | Current user                            |
| `GET`    | `/api/tasks`          |  ✓   | List your tasks                         |
| `POST`   | `/api/tasks`          |  ✓   | Create a task                           |
| `GET`    | `/api/tasks/{id}`     |  ✓   | Fetch one task                          |
| `PATCH`  | `/api/tasks/{id}`     |  ✓   | Partial update (any subset of fields)   |
| `DELETE` | `/api/tasks/{id}`     |  ✓   | Delete a task                           |

Authenticated requests use `Authorization: Bearer <token>`.

### Project layout

```
.
├── main.go                  # entry point (~70 lines)
├── docker-compose.yml       # Postgres only
├── .env.example
├── Makefile
└── internal/
    ├── config/              # env loading + DSN
    ├── store/               # pgx pool, embedded schema.sql, queries
    └── server/              # routes, middleware, handlers
```

The schema lives in `internal/store/schema.sql` and is applied on startup
(idempotent `CREATE TABLE IF NOT EXISTS`). For now, that's the migration
story — once the data model stabilizes we can switch to a real migration
tool.

---

## Roadmap

- [ ] Core task CRUD (create, read, update, delete)
- [ ] Reminders & due date notifications
- [ ] User auth
- [ ] Task sharing between servers
- [ ] Push notification support
- [ ] Mobile app
- [ ] Web UI
- [ ] CLI

---

## Philosophy

- **You own your data** — taskOS never phones home
- **Push over pull** — tasks come to you
- **Simple to host** — a single binary + a database should be enough to get started
- **API-first** — the server is the core; UIs are clients
