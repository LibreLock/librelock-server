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
