# Dungeon Books Tools

Tools for booksellers, served at `tools.dungeonbooks.com`. Go, single static binaries,
one service per concern.

## Services

| Service     | Path            | Role                                                  |
|-------------|-----------------|-------------------------------------------------------|
| `discovery` | `cmd/discovery` | Exa trending-book search + Open Library ISBN lookup   |
| `catalog`   | `cmd/catalog`   | tracks discovered books worth sourcing                |

## Endpoints

Both expose `GET /health`.

**discovery** (needs `EXA_API_KEY`)
- `POST /v1/discover` — `{"query": "...", "numResults": 15}` → `{"books": [...]}`

**catalog**
- `GET  /v1/candidates?status=&ingram_status=` → `{"candidates": [...]}`
- `POST /v1/candidates` — `{"books": [...]}` → tracked candidates (idempotent by ISBN-13)
- `POST /v1/candidates/{id}/dismiss`

## Develop

```sh
cp .env.example .env        # set EXA_API_KEY
go build ./...
go test ./...
go run ./cmd/discovery      # :8080
go run ./cmd/catalog        # :8080
```
