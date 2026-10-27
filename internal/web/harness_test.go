package web_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/VakZok/Cashflow/internal/web"
)

// fixedNow is the app's clock in tests.
var fixedNow = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

// testApp is the shared harness: the real app on a fresh temporary SQLite
// database, driven through httptest requests against its http.Handler (no
// network sockets, so it also runs in sandboxes). Tests only speak HTTP.
type testApp struct {
	t      *testing.T
	dbPath string
	app    *web.App
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	ta := &testApp{t: t, dbPath: filepath.Join(t.TempDir(), "test.db")}
	ta.start()
	t.Cleanup(ta.stop)
	return ta
}

func (ta *testApp) start() {
	ta.t.Helper()
	app, err := web.New(web.Config{DBPath: ta.dbPath, Now: func() time.Time { return fixedNow }})
	if err != nil {
		ta.t.Fatalf("start app: %v", err)
	}
	ta.app = app
}

func (ta *testApp) stop() {
	if ta.app != nil {
		_ = ta.app.Close()
		ta.app = nil
	}
}

// restart stops and starts the app on the same database file.
func (ta *testApp) restart() {
	ta.stop()
	ta.start()
}

// response is a fully read HTTP response.
type response struct {
	Status   int
	Body     string
	Location string
}

func (ta *testApp) do(req *http.Request) response {
	ta.t.Helper()
	rec := httptest.NewRecorder()
	ta.app.Handler().ServeHTTP(rec, req)
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		ta.t.Fatalf("read body: %v", err)
	}
	return response{Status: res.StatusCode, Body: string(b), Location: res.Header.Get("Location")}
}

func (ta *testApp) get(path string) response {
	ta.t.Helper()
	return ta.do(httptest.NewRequest(http.MethodGet, path, nil))
}

func (ta *testApp) post(path string, form url.Values) response {
	ta.t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return ta.do(req)
}

// follow GETs the Location of a redirect response, without its #fragment, as
// a browser does.
func (ta *testApp) follow(r response) response {
	ta.t.Helper()
	if r.Location == "" {
		ta.t.Fatalf("expected a redirect, got status %d", r.Status)
	}
	path, _, _ := strings.Cut(r.Location, "#")
	return ta.get(path)
}
