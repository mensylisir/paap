# Real Fullstack PostgreSQL Redis Demo

This example is the real application used for PAAP canvas demos. It has:

- `frontend`: a Go HTTP frontend that renders a small status page and proxies `/api/status` to the backend.
- `backend`: a Go API that reads runtime config from env vars or `/etc/paap/application-paap.yml`, writes to PostgreSQL, and AUTH/SET/GETs Redis.
- Expected PAAP topology: frontend component -> backend component -> PostgreSQL service + Redis service.

## Backend config

The backend accepts either env vars:

```text
POSTGRES_HOST
POSTGRES_PORT
POSTGRES_DATABASE
POSTGRES_USERNAME
POSTGRES_PASSWORD
REDIS_HOST
REDIS_PORT
REDIS_PASSWORD
```

or a Spring-style PAAP file mounted at `/etc/paap/application-paap.yml`:

```yaml
spring:
  datasource:
    url: jdbc:postgresql://postgresql:5432/postgres
    username: postgres
    password: ${POSTGRES_PASSWORD}
  data:
    redis:
      host: redis-master
      port: 6379
      password: ${REDIS_PASSWORD}
```

## Frontend config

The frontend only needs:

```text
BACKEND_URL=http://backend-1
```

It calls `BACKEND_URL + /api/status` and exposes the same result at `/api/status`.

## Build

```bash
docker build -t paap-real-backend:v1.0.0 ./backend
docker build -t paap-real-frontend:v1.0.0 ./frontend
```

For an offline kind cluster, pull/build locally, load images into kind, push to the environment registry if needed, then configure components in PAAP through the browser.
