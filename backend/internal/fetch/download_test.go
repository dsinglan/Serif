package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fast keeps the tests quick: one try and a one-second freeze limit.
var fast = Options{StallTimeout: time.Second, Tries: 1, Wait: time.Millisecond}

func TestLocalPath(t *testing.T) {
	got, err := LocalPath("data", "https://example.com/files/2026-08-31_test_in-network.json")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("data", "2026-08-31_test_in-network.json"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	for _, bad := range []string{
		"http://example.com/files/rates.json", // not https
		"https://example.com/",                // no file name
		"https://example.com/files/..",        // would climb out of data/
	} {
		if _, err := LocalPath("data", bad); err == nil {
			t.Errorf("%q: want an error, got none", bad)
		}
	}
}

func TestDownload(t *testing.T) {
	body := strings.Repeat("negotiated rates ", 1000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, body)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "rates.json")
	if err := Download(context.Background(), srv.Client(), srv.URL+"/rates.json", dest, fast); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Errorf("saved %d bytes, want %d", len(got), len(body))
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Error("the .part file was left behind")
	}
}

func TestDownloadFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cut-off.json" {
			w.Header().Set("Content-Length", "1000")
			w.Write(make([]byte, 500)) // then hang up halfway
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	for _, name := range []string{"cut-off.json", "missing.json"} {
		dest := filepath.Join(t.TempDir(), name)
		err := Download(context.Background(), srv.Client(), srv.URL+"/"+name, dest, fast)
		if err == nil {
			t.Errorf("%s: want an error, got none", name)
		}
		t.Logf("%s: %v", name, err)
		for _, leftover := range []string{dest, dest + ".part"} {
			if _, err := os.Stat(leftover); !os.IsNotExist(err) {
				t.Errorf("%s: %s should not exist", name, filepath.Base(leftover))
			}
		}
	}
}

func TestFailedDownloadKeepsOldCopy(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "rates.json")
	if err := os.WriteFile(dest, []byte("old copy"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Download(context.Background(), srv.Client(), srv.URL+"/rates.json", dest, fast); err == nil {
		t.Fatal("want an error, got none")
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old copy" {
		t.Errorf("the old copy changed to %q", got)
	}
}

func TestFreezeIsCaught(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.Write(make([]byte, 100))
		w.(http.Flusher).Flush()
		<-r.Context().Done() // then go quiet until the client gives up
	}))
	defer srv.Close()

	opt := Options{StallTimeout: 50 * time.Millisecond, Tries: 1}
	dest := filepath.Join(t.TempDir(), "rates.json")
	start := time.Now()
	err := Download(context.Background(), srv.Client(), srv.URL+"/rates.json", dest, opt)
	if err == nil || !strings.Contains(err.Error(), "no data for 50ms") {
		t.Fatalf("want a freeze error, got %v", err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("took %s to notice the freeze", took)
	}
}

func TestRetryAfterServerError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "busy", http.StatusInternalServerError) // the first try fails
			return
		}
		io.WriteString(w, "negotiated rates")
	}))
	defer srv.Close()

	opt := Options{StallTimeout: time.Second, Tries: 3, Wait: time.Millisecond}
	dest := filepath.Join(t.TempDir(), "rates.json")
	if err := Download(context.Background(), srv.Client(), srv.URL+"/rates.json", dest, opt); err != nil {
		t.Fatal(err)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("the server was asked %d times, want 2", n)
	}
}

// A redirect from https to plain http is refused, once: trying again would
// be sent the same way. One to another https URL is followed.
func TestHTTPSOnlyRedirects(t *testing.T) {
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"from":"plain http"}`)
	}))
	defer plain.Close()
	var calls atomic.Int32
	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/moved.json" {
			http.Redirect(w, r, "/rates.json", http.StatusFound)
			return
		}
		if r.URL.Path == "/rates.json" {
			io.WriteString(w, `{"from":"https"}`)
			return
		}
		http.Redirect(w, r, plain.URL+"/rates.json", http.StatusFound)
	}))
	defer secure.Close()
	client := NewClient() // with its redirect rule, trusting the test server
	client.Transport.(*http.Transport).TLSClientConfig = secure.Client().Transport.(*http.Transport).TLSClientConfig

	opt := Options{StallTimeout: time.Second, Tries: 3, Wait: time.Millisecond}
	dest := filepath.Join(t.TempDir(), "rates.json")
	err := Download(context.Background(), client, secure.URL+"/to-plain.json", dest, opt)
	if err == nil || !strings.Contains(err.Error(), "which doesn't use https") {
		t.Fatalf("redirect to plain http: err %v, want it refused", err)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("the server was asked %d times, want 1", n)
	}
	if err := Download(context.Background(), client, secure.URL+"/moved.json", dest, opt); err != nil {
		t.Fatalf("redirect to https: %v", err)
	}
	if got, _ := os.ReadFile(dest); string(got) != `{"from":"https"}` {
		t.Errorf("saved %q, want the https server's file", got)
	}
}

func TestNoRetryAfter404(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.NotFound(w, r)
	}))
	defer srv.Close()

	opt := Options{StallTimeout: time.Second, Tries: 3, Wait: time.Millisecond}
	dest := filepath.Join(t.TempDir(), "rates.json")
	if err := Download(context.Background(), srv.Client(), srv.URL+"/rates.json", dest, opt); err == nil {
		t.Fatal("want an error, got none")
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("the server was asked %d times, want 1", n)
	}
}

func TestProgress(t *testing.T) {
	body := strings.Repeat("negotiated rates ", 1000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		io.WriteString(w, body)
	}))
	defer srv.Close()

	var lastDone, lastTotal int64
	opt := fast
	opt.Progress = func(done, total int64) { lastDone, lastTotal = done, total }
	if err := Download(context.Background(), srv.Client(), srv.URL+"/rates.json", filepath.Join(t.TempDir(), "rates.json"), opt); err != nil {
		t.Fatal(err)
	}
	if lastDone != int64(len(body)) || lastTotal != int64(len(body)) {
		t.Errorf("last progress report: %d of %d, want %d of %d", lastDone, lastTotal, len(body), len(body))
	}
}

func TestEnsure(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		io.WriteString(w, "negotiated rates")
	}))
	defer srv.Close()
	dir := t.TempDir()
	url := srv.URL + "/rates.json"

	for _, c := range []struct {
		name    string
		refresh bool
		calls   int32
	}{
		{"the first time, it downloads", false, 1},
		{"after that, it reuses the saved copy", false, 1},
		{"with refresh, it downloads again", true, 2},
	} {
		path, err := Ensure(context.Background(), srv.Client(), url, dir, c.refresh, fast)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if path != filepath.Join(dir, "rates.json") || calls.Load() != c.calls {
			t.Errorf("%s: path %s after %d downloads; want %s after %d", c.name, path, calls.Load(), filepath.Join(dir, "rates.json"), c.calls)
		}
	}
}
