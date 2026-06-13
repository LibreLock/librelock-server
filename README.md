<div align="center">
  <img src="logo.svg" alt="LibreLock logo" height="70" />
  <h1 align="center">LibreLock Server</h1>
</div>

REST API for LibreLock, a secure, self-hosted password manager. Built with [Go](https://go.dev/) and [Gin](https://gin-gonic.com/), backed by [PostgreSQL](https://www.postgresql.org/).

## Get started

Refer to _Get started_ section in this [README](https://github.com/LibreLock/) for one command setup of the entire application. Keep reading if you wish to run the backend separately.

The preferable way to run LibreLock Server is via Docker Compose.

```bash
cp .env.example .env  # then update with your DB credentials
docker compose up -d --build
```

To run without Docker first install Go and PostgreSQL, then set up a database and user matching the one if `.env`. Finally, run the server with:

```bash
go run main.go
```

The API is now running at [localhost:8000](http://localhost:8000). PostgreSQL data persists in a Docker volume across restarts.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for project structure, code style, and security guidelines.
