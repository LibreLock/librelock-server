package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

func IssueToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
