<div align="center">
  <img src="logo.svg" alt="LibreLock logo" height="70" />
  <h1 align="center">LibreLock Server</h1>
</div>

REST API for LibreLock, a secure, self-hosted password manager. Built with [Go](https://go.dev/) and [Gin](https://gin-gonic.com/), backed by [SQLite](https://sqlite.org/).

## Get started

Refer to _Get started_ section in this [README](https://github.com/LibreLock/) for one command setup of the entire application. Keep reading if you wish to run the backend separately.

The preferable way to run LibreLock Server is via Docker Compose.

```bash
cp .env.example .env
docker compose up -d --build
```

To run without Docker just install Go, then run the server with:

```bash
go run main.go
```

The API is now running at [localhost:8000](http://localhost:8000). The SQLite database file is created automatically at `DB_PATH` (a Docker volume when using Compose) and persists across restarts.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, code style, and security guidelines.
