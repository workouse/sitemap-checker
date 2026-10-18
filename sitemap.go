package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// URLSet is root for site mite
type URLSet struct {
	XMLName xml.Name `xml:"urlset"`
	XMLNs   string   `xml:"xmlns,attr"`
	URL     []URL    `xml:"url"`

	// rawRoot is the original root start tag, with all its namespace declarations.
	rawRoot []byte
}

// URL is for every single location url
type URL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`

	// raw is the original <url> element, kept so that extensions such as
	// image:image or xhtml:link survive in the output.
	raw []byte
}

type ValidURL struct {
	IsValid    bool
	URL        URL
	StatusCode int
}

// Stats counts what happened to the urls of one or more sitemaps.
type Stats struct {
	Checked    int // urls requested
	Alive      int // answered 200
	Dead       int // answered with a final non-200 status
	Unverified int // still timing out, failing or answering 429/5xx after all retries
	Dropped    int // left out of the output (dead, plus unverified unless -keep-unverified)
}

func (s *Stats) add(o Stats) {
	s.Checked += o.Checked
	s.Alive += o.Alive
	s.Dead += o.Dead
	s.Unverified += o.Unverified
	s.Dropped += o.Dropped
}

func (s Stats) String() string {
	return fmt.Sprintf("Checked %d urls: %d alive, %d dead, %d unverified, %d removed from the output",
		s.Checked, s.Alive, s.Dead, s.Unverified, s.Dropped)
}

// parseURLSet parses a <urlset> document and remembers the original bytes of the
// root tag and of every <url>, so that the output can reproduce them verbatim.
func parseURLSet(data []byte) (URLSet, error) {
	us := URLSet{}
	d := xml.NewDecoder(bytes.NewReader(data))
	seenRoot := false

	for {
		start := d.InputOffset()
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return URLSet{}, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}

		switch {
		case !seenRoot:
			if se.Name.Local != "urlset" {
				return URLSet{}, fmt.Errorf("unexpected root element <%s>", se.Name.Local)
			}
			seenRoot = true
			us.XMLName = se.Name
			us.rawRoot = append([]byte(nil), data[start:d.InputOffset()]...)
			for _, a := range se.Attr {
				if a.Name.Space == "" && a.Name.Local == "xmlns" {
					us.XMLNs = a.Value
				}
			}
		case se.Name.Local == "url":
			var u URL
			if err := d.DecodeElement(&u, &se); err != nil {
				return URLSet{}, err
			}
			u.raw = append([]byte(nil), data[start:d.InputOffset()]...)
			us.URL = append(us.URL, u)
		default:
			if err := d.Skip(); err != nil {
				return URLSet{}, err
			}
		}
	}

	if !seenRoot {
		return URLSet{}, fmt.Errorf("no root element")
	}
	return us, nil
}

func (us *URLSet) saveToFile(filename string) error {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)

	if len(us.rawRoot) > 0 {
		buf.Write(us.rawRoot)
		buf.WriteString("\n")
		for _, u := range us.URL {
			raw := u.raw
			if len(raw) == 0 {
				m, err := xml.Marshal(struct {
					XMLName xml.Name `xml:"url"`
					URL
				}{URL: u})
				if err != nil {
					return err
				}
				raw = m
			}
			buf.WriteString("  ")
			buf.Write(raw)
			buf.WriteString("\n")
		}
		buf.WriteString("</urlset>\n")
	} else {
		m, err := xml.MarshalIndent(us, "", "  ")
		if err != nil {
			return err
		}
		buf.Write(m)
		buf.WriteString("\n")
	}

	return writeFileAtomic(filename, buf.Bytes())
}

var retryDelay = 500 * time.Millisecond

// retryable reports whether a check result may change on a later attempt.
func retryable(status int, err error) bool {
	return err != nil || status == http.StatusTooManyRequests || status >= 500
}

// statusOf sends one request and drains a bounded part of the body so the
// connection can be reused.
func statusOf(client *http.Client, method, loc string) (int, error) {
	resp, err := doRequest(client, method, loc)
	if err != nil {
		return 0, err
	}
	io.Copy(ioutil.Discard, io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
	return resp.StatusCode, nil
}

// probe asks for loc with HEAD. Servers answer HEAD wrongly often enough (405,
// 501, even 404) that anything but 200 is confirmed with a GET.
func probe(client *http.Client, loc string) (int, error) {
	status, err := statusOf(client, http.MethodHead, loc)
	if err != nil || status == http.StatusOK {
		return status, err
	}
	return statusOf(client, http.MethodGet, loc)
}

// checkURL probes loc, retrying with exponential backoff while the result is retryable.
func checkURL(client *http.Client, loc string) (int, error) {
	for attempt := 0; ; attempt++ {
		status, err := probe(client, loc)
		if !retryable(status, err) || attempt >= Retries {
			return status, err
		}
		time.Sleep(retryDelay << uint(attempt))
	}
}

const (
	outcomeAlive = iota
	outcomeDead
	outcomeUnverified
)

func (us *URLSet) validate() (URLSet, Stats) {
	// Initialize the new URLSet
	newURLSet := URLSet{
		XMLNs:   us.XMLNs,
		rawRoot: us.rawRoot,
	}

	// Create a semaphore to limit the number of concurrent requests
	maxConcurrentRequests := 10
	sem := make(chan struct{}, maxConcurrentRequests)

	// Use a WaitGroup to wait for all goroutines
	var wg sync.WaitGroup

	// Results are written by index so the output keeps the input order.
	outcomes := make([]int, len(us.URL))
	var completed int32

	n := len(us.URL)
	for i, url := range us.URL {
		wg.Add(1)
		sem <- struct{}{} // Acquire a semaphore slot

		go func(i int, url URL) {
			defer wg.Done()
			defer func() { <-sem }() // Release the semaphore slot

			status, err := checkURL(checkClient, url.Loc)
			done := atomic.AddInt32(&completed, 1)
			switch {
			case err == nil && status == http.StatusOK:
				fmt.Printf("Url %d/%d check (200): %s\n", done, n, url.Loc)
				outcomes[i] = outcomeAlive
			case retryable(status, err):
				cause := fmt.Sprintf("status %d", status)
				if err != nil {
					cause = err.Error()
				}
				fmt.Printf("Url %d/%d unverified (%s): %s\n", done, n, cause, url.Loc)
				outcomes[i] = outcomeUnverified
			default:
				fmt.Printf("Url %d/%d dead (%d): %s\n", done, n, status, url.Loc)
				outcomes[i] = outcomeDead
			}
		}(i, url)
	}

	wg.Wait() // Wait for all requests to complete

	stats := Stats{Checked: n}
	for i, url := range us.URL {
		switch outcomes[i] {
		case outcomeAlive:
			stats.Alive++
			newURLSet.URL = append(newURLSet.URL, url)
		case outcomeUnverified:
			stats.Unverified++
			if KeepUnverified {
				newURLSet.URL = append(newURLSet.URL, url)
			} else {
				stats.Dropped++
			}
		default:
			stats.Dead++
			stats.Dropped++
		}
	}

	return newURLSet, stats
}

// singleProcess validates the sitemap at uri and writes the live urls to filename.
// A sitemap index is handed over to sitemapIndexValidate.
func singleProcess(uri string, filename string) (Stats, error) {
	rawXMLData, err := fetchXML(uri)
	if err != nil {
		return Stats{}, err
	}

	root, err := rootElement(rawXMLData)
	if err != nil {
		return Stats{}, fmt.Errorf("%s is not a sitemap: %v", uri, err)
	}

	switch root {
	case "urlset":
		us, err := parseURLSet(rawXMLData)
		if err != nil {
			return Stats{}, fmt.Errorf("%s cannot be parsed: %v", uri, err)
		}
		newURLSet, stats := us.validate()
		if len(newURLSet.URL) == 0 {
			return stats, fmt.Errorf("%s has no live urls; %s was not written", uri, filename)
		}
		return stats, newURLSet.saveToFile(filename)
	case "sitemapindex":
		sitemapIndex, err := newSitemapIndexFromXML(rawXMLData)
		if err != nil {
			return Stats{}, fmt.Errorf("%s cannot be parsed: %v", uri, err)
		}
		return sitemapIndexValidate(sitemapIndex)
	default:
		return Stats{}, fmt.Errorf("%s is not a sitemap: unexpected root element <%s>", uri, root)
	}
}
