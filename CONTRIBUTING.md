# Contributing

## Setting the environment

Requirements: Go 1.25+

```bash
git clone <repo-url>
cd librelock-server

cp .env.example .env # edit as needed

go run .
```

The server applies schema changes automatically on startup via GORM AutoMigrate. No separate migration step needed in development.

## Code style

- Format with `gofmt` before committing
- Keep handlers thin: validation -> DB call -> JSON response, nothing more

## Security considerations

Before making changes, understand the cryptographic model described in [README.md](README.md#cryptographic-design). Key rules:

- **The server must never decrypt vault entries**
<br>
`encrypted_blob` and `iv` are passed through as opaque strings
- **The server must never store a plaintext `auth_credential`**
<br>
Always hash with Argon2id before writing to the DB
- **Token comparison must use the hash.**
<br>
Never store or compare raw tokens
- **Argon2id hash format** in `crypto/password.go` uses the standard PHC string format (`$argon2id$v=19$m=...`). Changing the params is fine — `VerifyPassword` reads `m`/`t`/`p` from the stored hash, so existing hashes keep working

## Submitting a pull request

1. Fork the repository and create a branch off `main`
2. Make your changes; ensure `go build ./...` and `go vet ./...` pass cleanly
3. Open a PR with a clear description of the changes
