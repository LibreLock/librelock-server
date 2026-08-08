<div align="center">
  <img src="logo.svg" alt="LibreLock logo" height="70" />
  <h1 align="center">LibreLock Server</h1>
</div>

REST API for LibreLock, a secure, self-hosted password manager. Built with [Go](https://go.dev/) and [Gin](https://gin-gonic.com/), backed by [SQLite](https://sqlite.org/).

> **Just want to run LibreLock?** One command brings up the whole stack - see [Get Started](https://github.com/LibreLock/) and the [self-hosting guide](https://github.com/LibreLock/.github/blob/main/docs/self-hosting.md).

## Development

Requires Go 1.25+.

```bash
cp .env.example .env
go run .
```

Or in Docker:

```bash
cp .env.example .env
docker compose up -d --build
```

The API listens on [localhost:8000](http://localhost:8000) (`PORT`, or `API_PORT` for the published port in Compose). The SQLite file is created automatically at `DB_PATH` - a Docker volume under Compose - and persists across restarts. Settings are read from the environment; see [`.env.example`](.env.example) for all of them.

In a normal deployment the frontend proxies `/api` to this server on the same origin, so `ALLOWED_ORIGIN` (CORS) only matters when the API is served from its own origin. Behind any proxy, set `TRUSTED_PROXIES` or every client shares one rate-limit bucket.

## Backups

The database file holds the entire instance: users, categories, vault entries, sessions, and organization data. Vault contents stay encrypted - the server has never held the keys - but the file also contains every user's Argon2id password hash, so a stolen backup is an offline attack on those hashes, and cracking one eventually unlocks that user's vault. **Keep snapshots off the server and encrypted at rest.**

The commands below take the volume name from `$VOLUME`. Set it once for your setup - `librelock_sqlite_data` under the full [stack](https://github.com/LibreLock/.github/blob/main/compose.yaml), `librelock-server_sqlite_data` when running this repository's Compose file on its own. `docker volume ls` settles it:

```bash
VOLUME=librelock_sqlite_data
```

The database runs in WAL mode, so `librelock.db` on its own is not the whole database - recent writes live in `librelock.db-wal` until they are checkpointed. Take snapshots with `VACUUM INTO`, which writes one consistent, self-contained file while the server keeps running:

```bash
docker run --rm -i \
  -v "$VOLUME:/data" \
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

### Restoring

Restoring replaces the whole instance, so stop the server first. Any stale `-wal` / `-shm` files must go with the old database: they belong to it, and SQLite must not try to replay them against the restored file.

```bash
docker compose stop api
docker run --rm \
  -v "$VOLUME:/data" \
  -v "$PWD:/in" \
  alpine sh -c 'rm -f /data/librelock.db*; cp /in/librelock-2026-01-01.db /data/librelock.db'
docker compose start api
```

The instance returns to exactly its state at snapshot time: entries created since are gone, deleted ones are back, and sessions that were active then work again. Schema differences are handled on startup by AutoMigrate, so restoring an older snapshot into a newer server is fine; the reverse is not.

Note that this is separate from the per-user **Export** in the app, which decrypts one vault in the browser into a portable file. That one protects a user who wants their data elsewhere; this one protects the operator whose disk died. Neither substitutes for the other.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, code style, and security guidelines.
