package crypto

import (
	"os"
	"runtime"
	"strconv"
	"sync"
)

// Every argon2 call allocates 64 MiB, and the endpoints that hash (login, register) are unauthenticated: without a cap, N concurrent requests hold N * 64 MiB and a few hundred of them exhaust the machine
// hashSlots bounds how many hashes run at once; the rest wait for a slot instead of allocating
// Requests are also rate limited per client, which keeps that queue short
var hashSlots = make(chan struct{}, maxConcurrentHashes())

// maxConcurrentHashes defaults to one hash per CPU (argon2 is CPU- and memory-bound, so more in flight than that buys no throughput) and can be lowered on small hosts via ARGON2_MAX_CONCURRENCY
func maxConcurrentHashes() int {
	n := runtime.NumCPU()
	if v := os.Getenv("ARGON2_MAX_CONCURRENCY"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			n = parsed
		}
	}
	if n < 2 {
		n = 2
	}
	return n
}

// acquireHashSlot blocks until a slot is free and returns the function that gives it back
func acquireHashSlot() func() {
	hashSlots <- struct{}{}
	return func() { <-hashSlots }
}

var (
	decoyOnce sync.Once
	decoyHash string
)

// DummyVerify burns the same work a real password check would
// Login for an unknown username would otherwise answer immediately while a real one spends ~100 ms in argon2, and that difference alone tells an attacker which usernames are registered
func DummyVerify(candidate string) {
	decoyOnce.Do(func() {
		if h, err := HashPassword(IssueToken()); err == nil {
			decoyHash = h
		}
	})
	if decoyHash == "" {
		return
	}
	_, _ = VerifyPassword(candidate, decoyHash)
}
