//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/zulkhair/pustaka/backend/internal/adapter/store"
	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/pkg/hash"
)

const defaultRealOllamaHost = "http://localhost:11434"

// fixed bodies the fake Ollama returns, so mock-mode assertions are exact.
const (
	mockOCRText       = "# integration OCR text"
	mockTransformText = "# integration transform output"
)

type serverEnv struct {
	t      *testing.T
	base   string
	client *http.Client
	store  *store.Store
	real   bool
}

type apiEnv struct {
	Status  int             `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// startServer boots the real cmd/server binary against an ephemeral Postgres,
// a real temp-dir blob store, and either a fake Ollama (default) or the live
// msi Ollama (OLLAMA_REAL=1). It returns once GET /api/health is 200.
func startServer(t *testing.T) *serverEnv {
	t.Helper()
	ctx := context.Background()
	real := os.Getenv("OLLAMA_REAL") == "1"

	// 1. ephemeral Postgres
	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("pustaka"),
		postgres.WithUsername("pustaka"),
		postgres.WithPassword("pustaka"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// 2. AI: fake Ollama unless OLLAMA_REAL=1
	ollamaHost := os.Getenv("OLLAMA_HOST")
	if real {
		if ollamaHost == "" {
			ollamaHost = defaultRealOllamaHost
		}
	} else {
		fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/generate":
				_, _ = w.Write([]byte(`{"response":"` + mockOCRText + `"}`))
			case "/api/chat":
				_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"` + mockTransformText + `"}}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		t.Cleanup(fake.Close)
		ollamaHost = fake.URL
	}

	// 3. temp blob dir + free port
	blobDir := t.TempDir()
	port := freePort(t)
	addr := "127.0.0.1:" + port
	base := "http://" + addr

	// 4. build + run the real server binary
	root := backendRoot()
	bin := filepath.Join(t.TempDir(), "pustaka-server")
	build := exec.Command("go", "build", "-o", bin, "./cmd/server")
	build.Dir = root
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	require.NoError(t, build.Run(), "build server binary")

	var logs bytes.Buffer
	srv := exec.Command(bin)
	srv.Dir = root
	srv.Env = append(os.Environ(),
		"APP_ENV=dev", // dev -> server runs migrations on boot
		"HTTP_ADDR="+addr,
		"DATABASE_URL="+dsn,
		"JWT_SECRET=integration-secret-0123456789",
		"RESEND_API_KEY=re_integration_dummy", // never used: test seeds + logs in, no register
		"MAIL_FROM=Pustaka <no-reply@example.com>",
		"OLLAMA_HOST="+ollamaHost,
		"BLOB_DIR="+blobDir,
		"OCR_MODEL=glm-ocr",
		"TRANSFORM_MODEL=qwen2.5:14b-instruct",
		"BCRYPT_COST=4",
		"APP_VERSION=9.9.9",
	)
	srv.Stdout, srv.Stderr = &logs, &logs
	require.NoError(t, srv.Start(), "start server binary")
	t.Cleanup(func() {
		_ = srv.Process.Signal(syscall.SIGTERM)
		done := make(chan struct{})
		go func() { _, _ = srv.Process.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = srv.Process.Kill()
		}
		if t.Failed() {
			t.Logf("server logs:\n%s", logs.String())
		}
	})

	client := &http.Client{Timeout: 30 * time.Second}
	env := &serverEnv{t: t, base: base, client: client, real: real}

	// 5. wait for health
	waitHealthy(t, client, base, &logs)

	// 6. seed a pre-verified user (migrations are already applied by the server)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	env.store = store.New(pool)

	return env
}

func (s *serverEnv) seedVerifiedUser(username, email, pw string) string {
	s.t.Helper()
	ctx := context.Background()
	ph, err := hash.HashPassword(pw, 10)
	require.NoError(s.t, err)
	u, err := s.store.CreateUser(ctx, domain.CreateUserParams{
		ID: uuid.NewString(), Username: username, Email: email,
		PasswordHash: ph, Role: domain.RoleUser,
	})
	require.NoError(s.t, err)
	require.NoError(s.t, s.store.SetUserEmailVerified(ctx, u.ID))
	return u.ID
}

// do issues a request and returns the response (body already read + closed) plus the body bytes.
func (s *serverEnv) do(method, path, bearer, ctype string, body io.Reader) (*http.Response, []byte) {
	s.t.Helper()
	req, err := http.NewRequest(method, s.base+path, body)
	require.NoError(s.t, err)
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := s.client.Do(req)
	require.NoError(s.t, err)
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func (s *serverEnv) postJSON(path, bearer string, body any) (int, apiEnv) {
	raw, err := json.Marshal(body)
	require.NoError(s.t, err)
	resp, b := s.do(http.MethodPost, path, bearer, "application/json", bytes.NewReader(raw))
	return resp.StatusCode, s.parseEnv(b)
}

func (s *serverEnv) getJSON(path, bearer string) (int, apiEnv) {
	resp, b := s.do(http.MethodGet, path, bearer, "", nil)
	return resp.StatusCode, s.parseEnv(b)
}

func (s *serverEnv) postMultipart(path, bearer string, fileBytes []byte) (int, apiEnv) {
	s.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "page.jpg")
	require.NoError(s.t, err)
	_, err = fw.Write(fileBytes)
	require.NoError(s.t, err)
	require.NoError(s.t, w.Close())
	resp, b := s.do(http.MethodPost, path, bearer, w.FormDataContentType(), &buf)
	return resp.StatusCode, s.parseEnv(b)
}

func (s *serverEnv) parseEnv(b []byte) apiEnv {
	s.t.Helper()
	var e apiEnv
	if len(b) > 0 {
		require.NoError(s.t, json.Unmarshal(b, &e), "body: %s", string(b))
	}
	return e
}

func (s *serverEnv) decode(e apiEnv, out any) {
	s.t.Helper()
	require.NoError(s.t, json.Unmarshal(e.Data, out))
}

func waitHealthy(t *testing.T, client *http.Client, base string, logs *bytes.Buffer) {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(base + "/api/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("server did not become healthy in time; logs:\n%s", logs.String())
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = l.Close() }()
	_, port, err := net.SplitHostPort(l.Addr().String())
	require.NoError(t, err)
	return port
}

// backendRoot returns the backend module root (this file lives at backend/test/integration/).
func backendRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// syntheticJPEG returns a small, valid JPEG so the real FS blob store (which
// decodes + re-encodes on Put) accepts the uploaded page in photo mode.
func syntheticJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 96))
	for y := 0; y < 96; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 2), 120, 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}))
	return buf.Bytes()
}
