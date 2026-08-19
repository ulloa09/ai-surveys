# 01 — Project Scaffold (SvelteKit + Go + PostgreSQL + CI)

## What to build

Bootstrap the full monorepo with both the SvelteKit frontend and Go backend wired together end-to-end. This slice has no user-visible feature but establishes the foundation every other slice builds on. A health-check endpoint and a single rendered page prove the stack is wired correctly.

Deliver:
- Monorepo structure with `frontend/` (SvelteKit) and `backend/` (Go)
- PostgreSQL connection from the Go backend with a database migration runner
- A `GET /api/health` endpoint that returns database connectivity status
- A SvelteKit home page that fetches and renders the health status
- CI pipeline that runs backend tests, frontend type-check, and lints on every push
- `docker-compose.yml` for local development (app + postgres)

## Acceptance criteria

- [ ] `docker-compose up` starts the full stack locally with no manual steps beyond copying `.env.example`
- [ ] `GET /api/health` returns `200` with database status when postgres is up, `503` when it is down
- [ ] SvelteKit home page fetches and displays the health status
- [ ] Database migration runner applies and rolls back migrations cleanly
- [ ] CI pipeline passes on a clean checkout
- [ ] Frontend type-check and backend `go vet` both pass with zero errors

## Blocked by

None — can start immediately.
