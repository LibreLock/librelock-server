package handlers_test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"librelock-server/appmode"
	"librelock-server/db"
	"librelock-server/handlers"
	"librelock-server/middleware"
)

// Covers the properties the docs promise about /auth/kdf, login and the KDF floor
// They are all invisible in normal use, so nothing else would catch a regression

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	database := db.Connect(filepath.Join(t.TempDir(), "test.db"))
	mode := appmode.New(database)
	secret := db.EnsureServerSecret(database, mode.Current())
	authH := handlers.NewAuthHandler(database, 3600, "test", mode, secret)

	r := gin.New()
	auth := r.Group("/auth", middleware.RateLimit(20, 1.0/3.0))
	auth.GET("/kdf", authH.KDF)
	auth.POST("/register", authH.Register)
	auth.POST("/login", authH.Login)
	return r
}

func do(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func registerBody(username string, overrides map[string]any) string {
	payload := map[string]any{
		"username":        username,
		"auth_credential": strings.Repeat("a", 64),
		"protected_key":   strings.Repeat("b", 64),
		"kdf_salt":        strings.Repeat("c", 64),
		"kdf_iter":        4,
		"kdf_memory":      65536,
		"kdf_parallelism": 4,
	}
	for k, v := range overrides {
		payload[k] = v
	}
	out, _ := json.Marshal(payload)
	return string(out)
}

func kdfSalt(t *testing.T, r *gin.Engine, username string) string {
	t.Helper()
	w := do(r, http.MethodGet, "/auth/kdf?username="+username, "")
	if w.Code != http.StatusOK {
		t.Fatalf("kdf %s: status %d", username, w.Code)
	}
	var body struct {
		Salt string `json:"kdf_salt"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("kdf %s: %v", username, err)
	}
	return body.Salt
}

// An unknown username must be indistinguishable from a registered one: same salt length, same hex shape, and the same answer every time
func TestKDFDecoyIsIndistinguishable(t *testing.T) {
	r := newTestRouter(t)

	if w := do(r, http.MethodPost, "/auth/register", registerBody("real", nil)); w.Code != http.StatusCreated {
		t.Fatalf("register: status %d body %s", w.Code, w.Body.String())
	}

	real := kdfSalt(t, r, "real")
	decoy := kdfSalt(t, r, "ghost")

	if len(decoy) != len(real) {
		t.Errorf("decoy salt length %d, real %d", len(decoy), len(real))
	}
	if _, err := hex.DecodeString(decoy); err != nil {
		t.Errorf("decoy salt is not hex: %q", decoy)
	}
	if again := kdfSalt(t, r, "ghost"); again != decoy {
		t.Errorf("decoy salt changed between calls: %q then %q", decoy, again)
	}
	if other := kdfSalt(t, r, "ghost2"); other == decoy {
		t.Error("every unknown username got the same salt")
	}
}

func TestLoginRejectsUnknownUserLikeABadPassword(t *testing.T) {
	r := newTestRouter(t)
	if w := do(r, http.MethodPost, "/auth/register", registerBody("real", nil)); w.Code != http.StatusCreated {
		t.Fatalf("register: status %d", w.Code)
	}

	body := `{"username":%q,"auth_credential":%q}`
	unknown := do(r, http.MethodPost, "/auth/login", fmt.Sprintf(body, "ghost", strings.Repeat("a", 64)))
	wrongPassword := do(r, http.MethodPost, "/auth/login", fmt.Sprintf(body, "real", strings.Repeat("z", 64)))

	if unknown.Code != http.StatusUnauthorized || wrongPassword.Code != http.StatusUnauthorized {
		t.Fatalf("unknown %d, wrong password %d; want 401 for both", unknown.Code, wrongPassword.Code)
	}
	if unknown.Body.String() != wrongPassword.Body.String() {
		t.Errorf("responses differ: %s vs %s", unknown.Body.String(), wrongPassword.Body.String())
	}
}

func TestRegisterRejectsWeakKDFParams(t *testing.T) {
	cases := map[string]map[string]any{
		"one iteration":  {"kdf_iter": 1},
		"8 MiB memory":   {"kdf_memory": 8192},
		"truncated salt": {"kdf_salt": strings.Repeat("c", 32)},
	}
	for name, overrides := range cases {
		t.Run(name, func(t *testing.T) {
			r := newTestRouter(t)
			w := do(r, http.MethodPost, "/auth/register", registerBody("weak", overrides))
			if w.Code != http.StatusUnprocessableEntity {
				t.Errorf("status %d, want 422; body %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestAuthEndpointsAreRateLimited(t *testing.T) {
	r := newTestRouter(t)
	for i := 0; i < 20; i++ {
		if w := do(r, http.MethodGet, "/auth/kdf?username=ghost", ""); w.Code != http.StatusOK {
			t.Fatalf("request %d: status %d, want 200", i+1, w.Code)
		}
	}
	w := do(r, http.MethodGet, "/auth/kdf?username=ghost", "")
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("request 21: status %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("429 without Retry-After")
	}
}
