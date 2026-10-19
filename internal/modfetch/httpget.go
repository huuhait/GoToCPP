package modfetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// A NotFoundError is a 404 or 410 answer: the archive, or the tag, is not
// there. The next source is tried after it.
type NotFoundError struct {
	URL    string
	Status int
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s: %d %s", redact(e.URL), e.Status, http.StatusText(e.Status))
}

// An HTTPError is any other answer than 200.
type HTTPError struct {
	URL    string
	Status int
	Hint   string
}

func (e *HTTPError) Error() string {
	s := fmt.Sprintf("%s: %d %s", redact(e.URL), e.Status, http.StatusText(e.Status))
	if e.Hint != "" {
		s += " (" + e.Hint + ")"
	}
	return s
}

// A TooLargeError is a download over the limit.
type TooLargeError struct {
	URL   string
	Limit int64
	What  string
}

func (e *TooLargeError) Error() string {
	return fmt.Sprintf("%s: %s is larger than the limit of %d bytes (%d MiB)", redact(e.URL), e.What, e.Limit, e.Limit>>20)
}

const maxRedirects = 5

// retryDelay is the wait before the first retry of an answer that says "try
// again later"; tests make it short.
var retryDelay = 1500 * time.Millisecond

// get downloads the URL into w, at most limit bytes, and returns the number
// of bytes. The headers extra are sent too. It follows at most 5 redirects, all
// of them to https URLs; the credentials for the host are sent to that host
// only, also after a redirect.
func (f *Fetcher) get(ctx context.Context, rawurl string, w io.Writer, limit int64, extra http.Header) (int64, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(retryDelay << (attempt - 1)):
			}
			switch w := w.(type) {
			case *os.File:
				if err := w.Truncate(0); err != nil {
					return 0, err
				}
				if _, err := w.Seek(0, io.SeekStart); err != nil {
					return 0, err
				}
			case *bytes.Buffer:
				w.Reset()
			default:
				return 0, lastErr // can't start again on this writer
			}
		}
		n, err := f.getOnce(ctx, rawurl, w, limit, extra)
		var he *HTTPError
		if errors.As(err, &he) && (he.Status == 429 || he.Status == 502 || he.Status == 503 || he.Status == 504) {
			lastErr = err
			continue
		}
		return n, err
	}
	return 0, lastErr
}

func (f *Fetcher) getOnce(ctx context.Context, rawurl string, w io.Writer, limit int64, extra http.Header) (int64, error) {
	u, err := url.Parse(rawurl)
	if err == nil && u.Scheme == "file" && f.isFileProxy(rawurl) {
		return f.readLocal(u, w, limit)
	}
	if err != nil || u.Scheme != "https" {
		return 0, fmt.Errorf("refusing to download %s: not an https URL", redact(rawurl))
	}
	req, err := http.NewRequestWithContext(ctx, "GET", rawurl, nil)
	if err != nil {
		return 0, fmt.Errorf("%s: %v", redact(rawurl), err)
	}
	req.Header.Set("User-Agent", userAgent())
	req.Header.Set("Accept-Encoding", "identity")
	for k, v := range extra {
		req.Header[k] = v
	}
	if cred := f.credential(u); cred != "" {
		if user, pass, ok := strings.Cut(cred, ":"); ok {
			req.SetBasicAuth(user, pass)
		} else {
			req.Header.Set("Authorization", "Bearer "+cred)
		}
	}
	base, err := f.httpClient()
	if err != nil {
		return 0, err
	}
	client := *base
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		if req.URL.Scheme != "https" {
			return fmt.Errorf("refusing the redirect to %s: not an https URL", redact(req.URL.String()))
		}
		if req.URL.Host != via[0].URL.Host {
			// Credentials belong to the host that they were configured for.
			req.Header.Del("Authorization")
			req.Header.Del("Cookie")
		}
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, cleanURLError(err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == 404 || resp.StatusCode == 410:
		return 0, &NotFoundError{URL: rawurl, Status: resp.StatusCode}
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return 0, &HTTPError{URL: rawurl, Status: resp.StatusCode, Hint: "access denied: for a private repository set " + EnvModAuth + "=" + u.Hostname() + "=<token>"}
	case resp.StatusCode != 200:
		return 0, &HTTPError{URL: rawurl, Status: resp.StatusCode}
	}
	if resp.ContentLength > limit {
		return 0, &TooLargeError{URL: rawurl, Limit: limit, What: "the download"}
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return n, fmt.Errorf("%s: download failed after %d bytes: %v", redact(rawurl), n, err)
	}
	if n > limit {
		return n, &TooLargeError{URL: rawurl, Limit: limit, What: "the download"}
	}
	return n, nil
}

// credential returns the credentials for the host of u, or "".
func (f *Fetcher) credential(u *url.URL) string {
	if len(f.cfg.Auth) == 0 {
		return ""
	}
	if c, ok := f.cfg.Auth[strings.ToLower(u.Host)]; ok {
		return c
	}
	return f.cfg.Auth[strings.ToLower(u.Hostname())]
}

// cleanURLError removes the query and the user name of the URL in a
// *url.Error, so that a secret in a URL does not reach a message.
func cleanURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("%s %s: %v", ue.Op, redact(ue.URL), ue.Err)
	}
	return err
}

// isFileProxy reports whether the URL is below a file:// element of
// GOCPP_MODPROXY: no other file URL is ever read.
func (f *Fetcher) isFileProxy(rawurl string) bool {
	for _, p := range f.cfg.Proxies {
		if strings.HasPrefix(p.URL, "file://") && strings.HasPrefix(rawurl, p.URL+"/") {
			return true
		}
	}
	return false
}

// readLocal copies a file of a file:// proxy to w, at most limit bytes. A
// missing file is "not found", as a 404 is.
func (f *Fetcher) readLocal(u *url.URL, w io.Writer, limit int64) (int64, error) {
	name := fileURLPath(u)
	fh, err := os.Open(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, &NotFoundError{URL: u.String(), Status: 404}
		}
		return 0, fmt.Errorf("%s: %v", name, err)
	}
	defer fh.Close()
	st, err := fh.Stat()
	if err != nil {
		return 0, err
	}
	if !st.Mode().IsRegular() {
		return 0, &NotFoundError{URL: u.String(), Status: 404}
	}
	if st.Size() > limit {
		return 0, &TooLargeError{URL: u.String(), Limit: limit, What: "the file"}
	}
	n, err := io.Copy(w, io.LimitReader(fh, limit+1))
	if err != nil {
		return n, fmt.Errorf("%s: %v", name, err)
	}
	if n > limit {
		return n, &TooLargeError{URL: u.String(), Limit: limit, What: "the file"}
	}
	return n, nil
}
