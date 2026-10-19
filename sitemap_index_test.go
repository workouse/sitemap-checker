package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestFindFileName(t *testing.T) {
	tests := []struct {
		name    string
		loc     string
		want    string
		wantErr bool
	}{
		{"root file", "http://x/sitemap.xml", "sitemap.xml", false},
		{"nested file", "http://x/maps/a.xml", filepath.Join("maps", "a.xml"), false},
		{"query is ignored", "http://x/maps/a.xml?page=2", filepath.Join("maps", "a.xml"), false},
		{"traversal stays relative", "http://x/../../etc/cron.d/job.xml", filepath.Join("etc", "cron.d", "job.xml"), false},
		{"encoded traversal stays relative", "http://x/%2e%2e/%2e%2e/job.xml", "job.xml", false},
		{"no file name", "http://x/", "", true},
		{"no path", "http://x", "", true},
		{"backslash", `http://x/a%5C..%5Cb.xml`, "", true},
		{"unparsable url", "http://x/%zz", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Sitemap{Loc: tt.loc}.findFileName()
			if (err != nil) != tt.wantErr {
				t.Fatalf("findFileName() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("findFileName() = %q, want %q", got, tt.want)
			}
			if err == nil && (filepath.IsAbs(got) || filepath.Clean(got) != got) {
				t.Errorf("findFileName() = %q, want a clean relative path", got)
			}
		})
	}
}

func TestSitemapIndexValidate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok.xml" {
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	defer withSettings(0, false)()

	// A closed server gives a connection error instead of an HTTP status.
	gone := httptest.NewServer(http.NotFoundHandler())
	goneURL := gone.URL
	gone.Close()

	index := SitemapIndex{
		XMLNs: "ns",
		Sitemap: []Sitemap{
			{Loc: srv.URL + "/ok.xml"},
			{Loc: srv.URL + "/missing.xml"},
			{Loc: goneURL + "/unreachable.xml"},
			{Loc: srv.URL + "/ok.xml?again"},
		},
	}

	done := make(chan SitemapIndex, 1)
	go func() { done <- index.validate() }()

	select {
	case got := <-done:
		if len(got.Sitemap) != 2 {
			t.Fatalf("validate() kept %d sitemaps, want 2: %+v", len(got.Sitemap), got.Sitemap)
		}
		if got.Sitemap[0].Loc != index.Sitemap[0].Loc || got.Sitemap[1].Loc != index.Sitemap[3].Loc {
			t.Errorf("validate() did not keep the live sitemaps in order: %+v", got.Sitemap)
		}
		if got.XMLNs != "ns" {
			t.Errorf("validate() XMLNs = %q, want %q", got.XMLNs, "ns")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("validate() blocked on an unreachable sitemap")
	}
}
