package crypto

import (
	"os"
	"runtime"
	"strconv"
	"sync"
)

// Every argon2 call allocates 64 MiB and the endpoints that hash are unauthenticated, so without a cap a few hundred concurrent requests exhaust the machine
// hashSlots bounds how many run at once; the rest wait for a slot instead of allocating
var hashSlots = make(chan struct{}, maxConcurrentHashes())

// One hash per CPU by default: argon2 is CPU- and memory-bound, so more in flight buys no throughput
// Lower it on small hosts with ARGON2_MAX_CONCURRENCY
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
// Without it login answers immediately for an unknown username and spends ~100 ms for a real one, which alone says who is registered
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
