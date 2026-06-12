<div align="center">
  <img src="logo.svg" alt="LibreLock logo" height="70" />
  <h1 align="center">LibreLock Server</h1>
</div>

REST API for LibreLock, a secure, self-hosted password manager. Built with [Go](https://go.dev/) and [Gin](https://gin-gonic.com/), backed by [PostgreSQL](https://www.postgresql.org/).

## Get started

The preferable way to run LibreLock is via Docker Compose.

```bash
cp .env.example .env  # then update with your DB credentials
docker compose up -d --build
```

The API is now running at [localhost:8000](http://localhost:8000/api). PostgreSQL data persists in a Docker volume across restarts.

Refer to _Get started_ section in this [README](https://github.com/LibreLock/) for details.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for project structure, code style, and security guidelines.
