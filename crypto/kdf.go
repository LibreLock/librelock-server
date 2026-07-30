package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// DecoyKDFSalt returns the salt GET /auth/kdf hands out for a username that has no account
// It has to be indistinguishable from a real salt, which the client generates as 32 random bytes rendered as 64 hex characters
// Deriving it from a per-instance secret keeps it stable across calls and restarts, so repeating the request can't expose it as a decoy either
func DecoyKDFSalt(secret, username string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(username))
	return hex.EncodeToString(mac.Sum(nil))
}
