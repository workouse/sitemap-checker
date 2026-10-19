package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
		if _, err := singleProcess(live.URL+"/sitemap.xml", out); err != nil {
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
			if _, err := singleProcess(live.URL+path, out); err == nil {
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

	if _, err := batchProcess(srv.URL); err == nil {
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
	locs := func(us URLSet, stats Stats) string {
		var paths []string
		for _, u := range us.URL {
			paths = append(paths, strings.TrimPrefix(u.Loc, srv.URL))
		}
		return strings.Join(paths, ",")
	}

	restore := withSettings(2, false)
	got, stats := urls.validate()
	if got := locs(got, stats); got != "/flaky" {
		t.Errorf("validate() kept %q, want /flaky", got)
	}
	if want := (Stats{Checked: 4, Alive: 1, Dead: 1, Unverified: 2, Dropped: 3}); stats != want {
		t.Errorf("validate() stats = %+v, want %+v", stats, want)
	}
	// Every probe is a HEAD followed by a confirming GET.
	if downHits != 6 {
		t.Errorf("/down requested %d times, want 6 (3 attempts)", downHits)
	}
	if goneHits != 2 {
		t.Errorf("/gone requested %d times, want 2 (1 attempt, 404 is not retried)", goneHits)
	}

	restore()
	defer withSettings(0, true)()
	got, stats = urls.validate()
	if got := locs(got, stats); got != "/flaky,/down,/limited" {
		t.Errorf("validate() with keep-unverified kept %q, want /flaky,/down,/limited", got)
	}
	if stats.Dropped != 1 {
		t.Errorf("validate() with keep-unverified dropped %d, want 1", stats.Dropped)
	}
}

const extendedSitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:image="http://www.google.com/schemas/sitemap-image/1.1" xmlns:xhtml="http://www.w3.org/1999/xhtml">
  <url>
    <loc>%[1]s/keep</loc>
    <priority>0.0</priority>
    <image:image><image:loc>%[1]s/img.png</image:loc></image:image>
    <xhtml:link rel="alternate" hreflang="tr" href="%[1]s/tr"/>
  </url>
  <url>
    <loc>%[1]s/gone</loc>
    <image:image><image:loc>%[1]s/gone.png</image:loc></image:image>
  </url>
</urlset>`

func TestSingleProcessPreservesExtensions(t *testing.T) {
	var mu sync.Mutex
	var methods []string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sitemap.xml" {
			fmt.Fprintf(w, extendedSitemap, srv.URL)
			return
		}
		if r.Header.Get("User-Agent") == "" || strings.HasPrefix(r.Header.Get("User-Agent"), "Go-http-client") {
			t.Errorf("request to %s has User-Agent %q", r.URL.Path, r.Header.Get("User-Agent"))
		}
		mu.Lock()
		methods = append(methods, r.Method+" "+r.URL.Path)
		mu.Unlock()
		switch {
		case r.URL.Path == "/keep" && r.Method == http.MethodHead:
			w.WriteHeader(http.StatusMethodNotAllowed) // forces the GET fallback
		case r.URL.Path == "/keep":
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	defer withSettings(0, false)()

	dir, err := ioutil.TempDir("", "sitemap")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "out.xml")

	if _, err := singleProcess(srv.URL+"/sitemap.xml", out); err != nil {
		t.Fatal(err)
	}
	data, err := ioutil.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		`xmlns:image="http://www.google.com/schemas/sitemap-image/1.1"`,
		`<priority>0.0</priority>`,
		`<image:loc>` + srv.URL + `/img.png</image:loc>`,
		`<xhtml:link rel="alternate" hreflang="tr" href="` + srv.URL + `/tr"/>`,
		"</urlset>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "/gone") {
		t.Errorf("output still has the dead url:\n%s", got)
	}
	if _, err := parseURLSet(data); err != nil {
		t.Errorf("output does not parse again: %v", err)
	}

	joined := strings.Join(methods, ",")
	if !strings.Contains(joined, "HEAD /keep") || !strings.Contains(joined, "GET /keep") {
		t.Errorf("requests = %v, want a HEAD and a GET for /keep", methods)
	}
	if info, err := os.Stat(out); err != nil || info.Mode().Perm() != 0644 && os.PathSeparator == '/' {
		t.Errorf("output stat = %v, %v; want mode 0644", info, err)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".sitemap-*")); len(left) != 0 {
		t.Errorf("temporary files left behind: %v", left)
	}
}

func TestSingleProcessNoLiveURLs(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sitemap.xml" {
			fmt.Fprintf(w, `<urlset xmlns="ns"><url><loc>%s/gone</loc></url></urlset>`, srv.URL)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	defer withSettings(0, false)()

	dir, err := ioutil.TempDir("", "sitemap")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "out.xml")

	stats, err := singleProcess(srv.URL+"/sitemap.xml", out)
	if err == nil {
		t.Fatal("singleProcess() succeeded without live urls, want an error")
	}
	if stats.Dead != 1 || stats.Dropped != 1 {
		t.Errorf("stats = %+v, want 1 dead and 1 dropped", stats)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("output file was written: %v", err)
	}
}
