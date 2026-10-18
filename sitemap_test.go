package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// withSettings sets the retry globals and returns a function that restores them.
func withSettings(retries int, keep bool) func() {
	oldRetries, oldKeep, oldDelay := Retries, KeepUnverified, retryDelay
	Retries, KeepUnverified, retryDelay = retries, keep, time.Millisecond
	return func() { Retries, KeepUnverified, retryDelay = oldRetries, oldKeep, oldDelay }
}

func TestRootElement(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    string
		wantErr bool
	}{
		{"urlset", `<?xml version="1.0"?><urlset xmlns="x"></urlset>`, "urlset", false},
		{"sitemapindex", `<sitemapindex></sitemapindex>`, "sitemapindex", false},
		{"html", `<!DOCTYPE html><html><body>Not found</body></html>`, "html", false},
		{"plain text", `Not found`, "", true},
		{"empty", ``, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rootElement([]byte(tt.data))
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Errorf("rootElement() = %q, %v; want %q, error %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestSingleProcess(t *testing.T) {
	var live *httptest.Server
	live = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			fmt.Fprintf(w, `<?xml version="1.0"?><urlset xmlns="ns"><url><loc>%s/b</loc></url><url><loc>%s/gone</loc></url><url><loc>%s/a</loc></url></urlset>`, live.URL, live.URL, live.URL)
		case "/a", "/b":
		case "/page.html":
			fmt.Fprint(w, "<!DOCTYPE html><html><body>hello</body></html>")
		default:
			http.NotFound(w, r)
		}
	}))
	defer live.Close()
	defer withSettings(0, false)()

	dir, err := ioutil.TempDir("", "sitemap")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	t.Run("urlset keeps live urls in order without padding", func(t *testing.T) {
		out := filepath.Join(dir, "ok.xml")
		if err := singleProcess(live.URL+"/sitemap.xml", out); err != nil {
			t.Fatal(err)
		}
		data, err := ioutil.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		got := string(data)
		if strings.Contains(got, "\x00") || strings.Contains(got, "/gone") {
			t.Errorf("unexpected output: %q", got)
		}
		if b, a := strings.Index(got, "/b"), strings.Index(got, "/a"); b < 0 || a < 0 || b > a {
			t.Errorf("urls missing or out of order: %q", got)
		}
	})

	for _, path := range []string{"/page.html", "/missing.xml"} {
		t.Run("no output for "+path, func(t *testing.T) {
			out := filepath.Join(dir, "bad"+strings.Replace(path, "/", "_", -1))
			if err := singleProcess(live.URL+path, out); err == nil {
				t.Fatal("singleProcess() succeeded, want an error")
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Errorf("output file was written: %v", err)
			}
		})
	}
}

func TestBatchProcessRejectsNonIndex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<urlset xmlns="ns"></urlset>`)
	}))
	defer srv.Close()

	oldOut := OutputFileName
	dir, err := ioutil.TempDir("", "sitemap")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	OutputFileName = filepath.Join(dir, "out.xml")
	defer func() { OutputFileName = oldOut }()

	if err := batchProcess(srv.URL); err == nil {
		t.Fatal("batchProcess() succeeded for a urlset, want an error")
	}
	if _, err := os.Stat(OutputFileName); !os.IsNotExist(err) {
		t.Errorf("output file was written: %v", err)
	}
}

func TestURLSetValidateRetries(t *testing.T) {
	var flakyHits, downHits, goneHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/flaky": // recovers on the third request
			if atomic.AddInt32(&flakyHits, 1) < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
		case "/down":
			atomic.AddInt32(&downHits, 1)
			w.WriteHeader(http.StatusInternalServerError)
		case "/limited":
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			atomic.AddInt32(&goneHits, 1)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	urls := URLSet{XMLNs: "ns"}
	for _, p := range []string{"/flaky", "/down", "/limited", "/gone"} {
		urls.URL = append(urls.URL, URL{Loc: srv.URL + p})
	}
	locs := func(us URLSet) string {
		var paths []string
		for _, u := range us.URL {
			paths = append(paths, strings.TrimPrefix(u.Loc, srv.URL))
		}
		return strings.Join(paths, ",")
	}

	restore := withSettings(2, false)
	if got := locs(urls.validate()); got != "/flaky" {
		t.Errorf("validate() kept %q, want /flaky", got)
	}
	if downHits != 3 {
		t.Errorf("/down requested %d times, want 3", downHits)
	}
	if goneHits != 1 {
		t.Errorf("/gone requested %d times, want 1 (404 is not retried)", goneHits)
	}

	restore()
	defer withSettings(0, true)()
	if got := locs(urls.validate()); got != "/flaky,/down,/limited" {
		t.Errorf("validate() with keep-unverified kept %q, want /flaky,/down,/limited", got)
	}
}
