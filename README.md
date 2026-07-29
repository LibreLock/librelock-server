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

## Backups

The database file holds the entire instance: users, categories, vault entries, sessions, and organization data. Vault contents stay encrypted - the server has never held the keys - but the file also contains every user's Argon2id password hash, so a stolen backup is an offline attack on those hashes, and cracking one eventually unlocks that user's vault. **Keep snapshots off the server and encrypted at rest.**

The database runs in WAL mode, so `librelock.db` on its own is not the whole database - recent writes live in `librelock.db-wal` until they are checkpointed. Take snapshots with `VACUUM INTO`, which writes one consistent, self-contained file while the server keeps running:

```bash
docker run --rm -i \
  -v librelock-server_sqlite_data:/data \
  -v "$PWD:/out" \
  alpine sh -c "apk add -q --no-cache sqlite && sqlite3 /data/librelock.db" <<EOF
VACUUM INTO '/out/librelock-$(date +%F).db';
EOF
```

Running without Docker, it is simply:

```bash
sqlite3 "$DB_PATH" "VACUUM INTO '/backups/librelock-$(date +%F).db'"
```

Check the snapshot before trusting it - `sqlite3 librelock-2026-01-01.db "PRAGMA integrity_check"` should print `ok`. For scheduled backups, put either command in a script and run it from cron; there is nothing to coordinate with the server.

> The volume is named after the Compose project, so it is `librelock-server_sqlite_data` only when the directory is named `librelock-server`. Run `docker volume ls` to confirm.

### Restoring

Restoring replaces the whole instance, so stop the server first. Any stale `-wal` / `-shm` files must go with the old database: they belong to it, and SQLite must not try to replay them against the restored file.

```bash
docker compose stop api
docker run --rm \
  -v librelock-server_sqlite_data:/data \
  -v "$PWD:/in" \
  alpine sh -c 'rm -f /data/librelock.db*; cp /in/librelock-2026-01-01.db /data/librelock.db'
docker compose start api
```

The instance returns to exactly its state at snapshot time: entries created since are gone, deleted ones are back, and sessions that were active then work again. Schema differences are handled on startup by AutoMigrate, so restoring an older snapshot into a newer server is fine; the reverse is not.

Note that this is separate from the per-user **Export** in the app, which decrypts one vault in the browser into a portable file. That one protects a user who wants their data elsewhere; this one protects the operator whose disk died. Neither substitutes for the other.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, code style, and security guidelines.
