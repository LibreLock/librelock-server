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

const codeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// IssueCode returns an n-character random alphanumeric string
// Uses rejection sampling so every character is uniformly distributed (no modulo bias)
func IssueCode(n int) string {
	out := make([]byte, n)
	buf := make([]byte, 1)
	const max = 256 - (256 % len(codeAlphabet)) // largest unbiased byte value
	for i := 0; i < n; {
		rand.Read(buf)
		if int(buf[0]) >= max {
			continue
		}
		out[i] = codeAlphabet[int(buf[0])%len(codeAlphabet)]
		i++
	}
	return string(out)
}

func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
