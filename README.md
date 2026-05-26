# taskOS-server

The HTTP API server for [taskOS](https://github.com/brandonthis-that/taskOS) — a
self-hosted task manager. This repository contains the server only; clients
(mobile, web, CLI) live in separate repos.

- Go, stdlib HTTP, no web framework
- PostgreSQL via [`pgx/v5`](https://github.com/jackc/pgx)
- Schema applied on startup from an embedded `schema.sql`
- Bearer-token auth (opaque tokens, SHA-256 hashed at rest, bcrypt passwords)

## Prerequisites

- Go 1.22+ (developed against 1.26)
- Docker + Docker Compose (for the bundled Postgres)

## Run it

```bash
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

If Postgres is already running with stale credentials from a previous
`docker compose up`, run `make reset` to drop the data volume and start
fresh. The `POSTGRES_*` env vars only take effect on a fresh data dir.

## API

All endpoints return JSON. Authenticated requests use
`Authorization: Bearer <token>`.

| Method   | Path                | Auth | Description                             |
|----------|---------------------|:----:|-----------------------------------------|
| `GET`    | `/healthz`          |  —   | Liveness + DB ping                      |
| `POST`   | `/api/auth/signup`  |  —   | Create account, returns bearer token    |
| `POST`   | `/api/auth/login`   |  —   | Exchange credentials for a bearer token |
| `POST`   | `/api/auth/logout`  |  ✓   | Revoke current token                    |
| `GET`    | `/api/auth/me`      |  ✓   | Current user                            |
| `GET`    | `/api/tasks`        |  ✓   | List your tasks                         |
| `POST`   | `/api/tasks`        |  ✓   | Create a task                           |
| `GET`    | `/api/tasks/{id}`   |  ✓   | Fetch one task                          |
| `PATCH`  | `/api/tasks/{id}`   |  ✓   | Partial update (any subset of fields)   |
| `DELETE` | `/api/tasks/{id}`   |  ✓   | Delete a task                           |

### Task fields

```jsonc
{
  "id":          "uuid",
  "user_id":     "uuid",
  "title":       "string, required",
  "description": "string",
  "done":        false,
  "due_at":      "RFC3339 timestamp, optional",
  "created_at":  "RFC3339 timestamp",
  "updated_at":  "RFC3339 timestamp"
}
```

`PATCH /api/tasks/{id}` accepts any subset of `title`, `description`, `done`,
`due_at`. To clear an existing `due_at`, send `{"clear_due_at": true}`.

### Example

```bash
TOKEN=$(curl -s -X POST localhost:8080/api/auth/signup \
  -H 'content-type: application/json' \
  -d '{"email":"me@example.com","password":"hunter2hunter2"}' | jq -r .token)

curl -s -X POST localhost:8080/api/tasks \
  -H "authorization: Bearer $TOKEN" \
  -H 'content-type: application/json' \
  -d '{"title":"Buy milk","due_at":"2026-06-01T09:00:00Z"}' | jq

curl -s localhost:8080/api/tasks -H "authorization: Bearer $TOKEN" | jq
```

## Configuration

All config is read from the environment (with `.env` autoloaded if present).
See [`.env.example`](./.env.example) for the full list.

| Variable        | Default        | Notes                                            |
|-----------------|----------------|--------------------------------------------------|
| `DB_USER`       | —              | required                                         |
| `DB_PASSWORD`   | —              | required                                         |
| `DB_NAME`       | —              | required                                         |
| `DB_HOST`       | `localhost`    | set to `postgres_db` when running inside compose |
| `DB_PORT`       | `5432`         |                                                  |
| `HTTP_ADDR`     | `:8080`        |                                                  |
| `SESSION_HOURS` | `720` (30 days)| session token lifetime                           |

## Layout

```
.
├── main.go                  # entry point + graceful shutdown
├── docker-compose.yml       # Postgres only
├── Makefile                 # see `make help`
└── internal/
    ├── config/              # env loading + DSN
    ├── store/               # pgx pool, embedded schema.sql, queries
    │   ├── schema.sql
    │   ├── users.go         # users + sessions
    │   └── tasks.go
    └── server/              # routes, middleware, handlers
        ├── auth.go          # signup / login / logout / me
        └── tasks.go         # task CRUD
```

The schema lives in `internal/store/schema.sql` and is applied on every
startup (idempotent `CREATE TABLE IF NOT EXISTS`). Once the data model
stabilizes, this will move to a real migration tool.

## Not yet implemented

- Reminder/notification dispatch (the "push" half of the model)
- Server-to-server task sharing
- Tests
