# Contributing

## Setting the environment

Requirements: Go 1.25+

```bash
git clone <repo-url>
cd librelock-server

cp .env.example .env # edit as needed

go run .
```

The SQLite file is created automatically at `DB_PATH`, and the server applies schema changes on startup via GORM AutoMigrate. No separate migration step needed in development.

Organization tables (roles, invites, audit log, shared vault) are migrated lazily, only once an instance switches to organization mode - so organization-specific code must tolerate those tables not existing at all in personal mode.

### Schema changes

AutoMigrate only diffs the current models against the current tables: it creates tables, adds columns and indexes, and does nothing else. It cannot move data, and it has no memory of what it has already done. Two rules follow.

1. **Anything additive is free.** A new table, a new index, or a new field that is nullable or carries a `default:` lands on existing installs with no extra work. A `not null` field without a default does not - SQLite refuses `ADD COLUMN NOT NULL` with no default, the migration calls `log.Fatalf`, and every instance that pulls it crashloops. **Always give new columns a default.**

2. **Everything else goes in `db/migrations.go`.** Renames, backfills, drops, row rewrites. The list is append-only and its index is the schema version recorded in `app_state.schema_version`, so an install that jumps several releases replays exactly the steps it missed, once each, in order. Never reorder or edit an entry that has shipped; add to the end. Each step runs in its own transaction (SQLite has transactional DDL, so a failure leaves nothing behind) and must be safe on a personal database, where the org tables do not exist - guard with `hasColumn` / a table check the way `dropColumn` does.

A database created by the current boot is stamped straight to head, so historical steps never run against a fresh install. A database stamped *ahead* of the running binary aborts the boot: the schema has moved under code that does not know about it.

Encrypted blobs are the exception to all of this. The server holds no keys, so no migration can touch entry contents - a format change has to happen client-side, keyed off the `version` column on the row, with both formats readable while installs catch up.

## Versioning

`GET /version` reports the running version. Nothing in this repository records a number: `version/version.go` says `dev`, and CI stamps the real value at link time from the git tag. To stamp a local build the same way:

```bash
docker compose build --build-arg VERSION="$(git describe --tags)"
```

LibreLock has one version number for the whole project: the same tag goes on this repository and on [librelock-web](https://github.com/LibreLock/librelock-web), and Settings → About shows a single version. Tag both with [`release.sh`](https://github.com/LibreLock/.github/blob/main/release.sh) rather than by hand:

```bash
./release.sh 0.1.0   # checks both repos are clean and in sync, then tags and pushes v0.1.0 in each
```

Pushing a `v*` tag runs `.github/workflows/publish.yml`, which builds `linux/amd64` + `linux/arm64` and pushes `ghcr.io/librelock/librelock-server` as `1.2.3`, `1.2`, `1`. Pushes to `main` publish `latest` and `main-<sha>`. Self-hosters run whatever `LIBRELOCK_VERSION` in their `.env` points at.

## Code style

- Format with `gofmt` before committing
- Keep handlers thin: validation -> DB call -> JSON response, nothing more

## Security considerations

Before making changes, understand the cryptographic model described in [Cryptography & Session Handling](https://github.com/LibreLock/.github/blob/main/docs/cryptography.md). Key rules:

- **The server must never decrypt vault entries**
<br>
`encrypted_blob` and `iv` are passed through as opaque strings, in both the personal and the shared organization vault
- **The server must never see key material it could unwrap**
<br>
`protected_key`, `encrypted_private_key`, and a membership's `wrapped_key` are opaque; `public_key` is the only key stored in the clear
- **The server must never store a plaintext `auth_credential`**
<br>
Always hash with Argon2id before writing to the DB
- **Token comparison must use the hash.**
<br>
Never store or compare raw tokens
- **Argon2id hash format** in `crypto/password.go` uses the standard PHC string format (`$argon2id$v=19$m=...`). Changing the params is fine - `VerifyPassword` reads `m`/`t`/`p` from the stored hash, so existing hashes keep working

## Submitting a pull request

1. Fork the repository and create a branch off `main`
2. Make your changes; ensure `go build ./...` and `go vet ./...` pass cleanly
3. Open a PR with a clear description of the changes
