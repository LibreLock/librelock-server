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

The commands below use the volume name `librelock_sqlite_data`, which is what the full [stack](https://github.com/LibreLock/.github/blob/main/compose.yaml) creates. Running this repository's Compose file on its own it is `librelock-server_sqlite_data` instead. Substitute yours in the commands below.

The database runs in WAL mode, so `librelock.db` on its own is not the whole database - recent writes live in `librelock.db-wal` until they are checkpointed. Snapshot it with SQLite's online-backup API, which reads through an open connection and writes one self-contained file while the server keeps running:

```bash
docker run --rm -v librelock_sqlite_data:/data -v .:/out alpine sh -c "apk add -q --no-cache sqlite && sqlite3 /data/librelock.db '.backup /out/librelock-backup.db'"
```

The snapshot lands in the folder you run this from. Every expansion happens inside the container, so that line is identical on Linux, macOS, PowerShell and `cmd` (it needs Docker 23+ for the relative `-v .:/out` mount).

Running without Docker, it is simply:

```bash
sqlite3 data/librelock.db ".backup librelock-backup.db"
```

Check the snapshot before trusting it - `sqlite3 librelock-backup.db "PRAGMA integrity_check"` should print `ok`. For scheduled backups with a dated filename, see [Backups](https://github.com/LibreLock/.github/blob/main/docs/self-hosting.md#backups), which has the cron and Task Scheduler forms; there is nothing to coordinate with the server either way.

### Restoring

Restoring replaces the whole instance, so stop the server first. Any stale `-wal` / `-shm` files must go with the old database: they belong to it, and SQLite must not try to replay them against the restored file.

```bash
docker compose stop api
docker run --rm -v librelock_sqlite_data:/data -v .:/in alpine sh -c "rm -f /data/librelock.db*; cp /in/librelock-backup.db /data/librelock.db"
docker compose start api
```

The instance returns to exactly its state at snapshot time: entries created since are gone, deleted ones are back, and sessions that were active then work again. Schema differences are handled on startup, so restoring an older snapshot into a newer server is fine; the reverse is not - a database migrated by a newer release refuses to boot on an older binary rather than let it write.

The server also snapshots itself before any version change migrates the schema, into `backups/` next to the database (`librelock-<old version>-<timestamp>.db`, three kept, disabled with `UPGRADE_BACKUPS=false`). Restore one exactly as above. It lives on the same disk as the original, so it undoes a bad upgrade and nothing else - it is not a substitute for the off-server backups above.

Note that this is separate from the per-user **Export** in the app, which decrypts one vault in the browser into a portable file. That one protects a user who wants their data elsewhere; this one protects the operator whose disk died. Neither substitutes for the other.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, code style, and security guidelines.
