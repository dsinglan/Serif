// Package fetch downloads big files and saves them on disk.
package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"
)

// Options say how patient a download is, and who hears about its progress.
type Options struct {
	StallTimeout time.Duration // stop a try when no data arrives for this long
	Tries        int           // how many tries in all
	Wait         time.Duration // pause before the second try; it doubles after that

	// Progress, if set, is told how many bytes have arrived as they come in.
	// total is -1 when the server doesn't say. A new try starts again from 0.
	Progress func(done, total int64)
}

// DefaultOptions suit Centene's server, which has run as slow as 0.4 MB/s
// and once froze partway through without closing the connection.
var DefaultOptions = Options{StallTimeout: 30 * time.Second, Tries: 3, Wait: 2 * time.Second}

// NewClient returns an HTTP client for big downloads. Its limits cover
// connecting and waiting for the server to start answering. Nothing limits
// the whole download, which can take minutes on a slow server; the freeze
// rule in Download catches a server that goes quiet instead. It follows a
// redirect only to another https URL.
func NewClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{Transport: transport, CheckRedirect: httpsOnly}
}

// httpsOnly follows a redirect only to https, as LocalPath asks of the
// first URL, and stops after 10 redirects, as Go does by default.
func httpsOnly(req *http.Request, via []*http.Request) error {
	if req.URL.Scheme != "https" {
		return insecureRedirect{req.URL.String()}
	}
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	return nil
}

// insecureRedirect is a redirect away from https, refused.
type insecureRedirect struct{ url string }

func (e insecureRedirect) Error() string {
	return "redirected to " + e.url + ", which doesn't use https"
}

// LocalPath returns where the file at fileURL is saved: in dataDir, under
// the file's own name. The URL comes from the index, which we don't control,
// so it must use https and its name must stay inside dataDir.
func LocalPath(dataDir, fileURL string) (string, error) {
	u, err := url.Parse(fileURL)
	if err != nil {
		return "", fmt.Errorf("bad file URL: %w", err)
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("file URL %q does not use https", fileURL)
	}
	name := path.Base(u.Path)
	if name == "." || name == "/" || !filepath.IsLocal(name) {
		return "", fmt.Errorf("file URL %q has no usable file name", fileURL)
	}
	return filepath.Join(dataDir, name), nil
}

// Ensure returns where fileURL is saved in dataDir. It downloads the file
// first when no copy is saved yet, or when refresh is set; otherwise it
// reuses the saved copy, so a restart works without the network.
func Ensure(ctx context.Context, client *http.Client, fileURL, dataDir string, refresh bool, opt Options) (string, error) {
	dest, err := LocalPath(dataDir, fileURL)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(dest); err == nil && !refresh {
		return dest, nil // already saved
	}
	log.Printf("downloading %s to %s", fileURL, dest)
	if err := Download(ctx, client, fileURL, dest, opt); err != nil {
		return "", err
	}
	if info, err := os.Stat(dest); err == nil {
		log.Printf("saved %s (%.2f MB)", filepath.Base(dest), float64(info.Size())/1e6)
	}
	return dest, nil
}

// Download saves fileURL to dest. Each try writes to dest+".part", which is
// renamed to dest only once every byte has arrived, so dest is never half a
// file. If every try fails, an older copy at dest stays as it was.
func Download(ctx context.Context, client *http.Client, fileURL, dest string, opt Options) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	part := dest + ".part"
	wait := opt.Wait
	for try := 1; ; try++ {
		err := save(ctx, client, fileURL, part, opt)
		if err == nil {
			return os.Rename(part, dest)
		}
		os.Remove(part) // don't leave half a file behind
		if try >= opt.Tries || !worthRetrying(err) || ctx.Err() != nil {
			return fmt.Errorf("downloading %s: %w", filepath.Base(dest), err)
		}
		log.Printf("download try %d of %d failed (%v); trying again in %s", try, opt.Tries, err, wait)
		sleep(ctx, wait)
		wait *= 2
	}
}

// save makes one try. It writes the response body to file, stops if no data
// arrives for opt.StallTimeout, and checks that every byte arrived.
func save(ctx context.Context, client *http.Client, fileURL, file string, opt Options) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return statusError{resp.StatusCode, resp.Status}
	}

	f, err := os.Create(file)
	if err != nil {
		return err
	}
	// The freeze rule: the timer restarts whenever data arrives. If it runs
	// out, it cancels this try.
	timer := time.AfterFunc(opt.StallTimeout, func() {
		cancel(fmt.Errorf("no data for %s", opt.StallTimeout))
	})
	var body io.Reader = freezeWatch{resp.Body, timer, opt.StallTimeout}
	if opt.Progress != nil {
		body = &progressReader{r: body, total: resp.ContentLength, report: opt.Progress}
	}
	n, copyErr := io.Copy(f, body)
	timer.Stop()
	closeErr := f.Close()
	if copyErr != nil {
		if cause := context.Cause(ctx); cause != nil {
			return cause // the freeze rule fired, or the caller cancelled
		}
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return fmt.Errorf("got %d of %d bytes", n, resp.ContentLength)
	}
	return nil
}

// freezeWatch passes reads through and restarts the timer whenever data
// arrives.
type freezeWatch struct {
	r       io.Reader
	timer   *time.Timer
	timeout time.Duration
}

func (w freezeWatch) Read(p []byte) (int, error) {
	n, err := w.r.Read(p)
	if n > 0 {
		w.timer.Reset(w.timeout)
	}
	return n, err
}

// progressReader passes reads through and reports the running total.
type progressReader struct {
	r           io.Reader
	done, total int64
	report      func(done, total int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.done += int64(n)
		p.report(p.done, p.total)
	}
	return n, err
}

// statusError is any answer from the server other than 200 OK.
type statusError struct {
	code   int
	status string
}

func (e statusError) Error() string { return "server answered " + e.status }

// worthRetrying says whether another try might work. A freeze, a dropped
// connection or a server error (5xx) can clear up on its own. A 404, a
// redirect away from https, or a problem writing to our own disk won't.
func worthRetrying(err error) bool {
	var status statusError
	if errors.As(err, &status) {
		return status.code >= 500 || status.code == http.StatusTooManyRequests
	}
	if errors.As(err, new(insecureRedirect)) {
		return false
	}
	var diskErr *fs.PathError
	return !errors.As(err, &diskErr)
}

// sleep waits for d, or less if ctx is cancelled first.
func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}
