package main

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

// URLSet is root for site mite
type URLSet struct {
	XMLName xml.Name `xml:"urlset"`
	XMLNs   string   `xml:"xmlns,attr"`
	URL     []URL    `xml:"url"`
}

// URL is for every single location url
type URL struct {
	Loc        string  `xml:"loc"`
	LastMod    string  `xml:"lastmod,omitempty"`
	ChangeFreq string  `xml:"changefreq,omitempty"`
	Priority   float32 `xml:"priority,omitempty"`
}

type ValidURL struct {
	IsValid    bool
	URL        URL
	StatusCode int
}

func (us *URLSet) saveToFile(filename string) error {
	m, err := xml.MarshalIndent((*us), "\r\n", "    ")
	if err != nil {
		return err
	}

	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	file.Write([]byte(xml.Header))
	file.Write(m)
	file.Close()
	return err
}

var retryDelay = 500 * time.Millisecond

// retryable reports whether a check result may change on a later attempt.
func retryable(status int, err error) bool {
	return err != nil || status == http.StatusTooManyRequests || status >= 500
}

// checkURL requests loc, retrying with exponential backoff while the result is retryable.
func checkURL(client *http.Client, loc string) (int, error) {
	for attempt := 0; ; attempt++ {
		status := 0
		resp, err := client.Get(loc)
		if err == nil {
			status = resp.StatusCode
			resp.Body.Close()
		}
		if !retryable(status, err) || attempt >= Retries {
			return status, err
		}
		time.Sleep(retryDelay << uint(attempt))
	}
}

func (us *URLSet) validate() URLSet {
	// Create an HTTP client with a timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Initialize the new URLSet
	newURLSet := URLSet{
		XMLNs: us.XMLNs,
	}

	// Create a semaphore to limit the number of concurrent requests
	maxConcurrentRequests := 10
	sem := make(chan struct{}, maxConcurrentRequests)

	// Use a WaitGroup to wait for all goroutines
	var wg sync.WaitGroup

	// Results are written by index so the output keeps the input order.
	keep := make([]bool, len(us.URL))

	n := len(us.URL)
	for i, url := range us.URL {
		wg.Add(1)
		sem <- struct{}{} // Acquire a semaphore slot

		go func(i int, url URL) {
			defer wg.Done()
			defer func() { <-sem }() // Release the semaphore slot

			status, err := checkURL(client, url.Loc)
			switch {
			case err == nil && status == http.StatusOK:
				fmt.Printf("Url %d/%d check (200): %s\n", i, n, url.Loc)
				keep[i] = true
			case retryable(status, err):
				cause := fmt.Sprintf("status %d", status)
				if err != nil {
					cause = err.Error()
				}
				if KeepUnverified {
					fmt.Printf("Url %d/%d unverified, kept (%s): %s\n", i, n, cause, url.Loc)
					keep[i] = true
				} else {
					fmt.Printf("Url %d/%d unverified, dropped (%s): %s\n", i, n, cause, url.Loc)
				}
			default:
				fmt.Printf("Url %d/%d dead (%d): %s\n", i, n, status, url.Loc)
			}
		}(i, url)
	}

	wg.Wait() // Wait for all requests to complete

	for i, url := range us.URL {
		if keep[i] {
			newURLSet.URL = append(newURLSet.URL, url)
		}
	}

	return newURLSet
}

// singleProcess validates the sitemap at uri and writes the live urls to filename.
// A sitemap index is handed over to sitemapIndexValidate.
func singleProcess(uri string, filename string) error {
	rawXMLData, err := fetchXML(uri)
	if err != nil {
		return err
	}

	root, err := rootElement(rawXMLData)
	if err != nil {
		return fmt.Errorf("%s is not a sitemap: %v", uri, err)
	}

	switch root {
	case "urlset":
		us := URLSet{}
		if err := xml.Unmarshal(rawXMLData, &us); err != nil {
			return fmt.Errorf("%s cannot be parsed: %v", uri, err)
		}
		newURLSet := us.validate()
		return newURLSet.saveToFile(filename)
	case "sitemapindex":
		sitemapIndex, err := newSitemapIndexFromXML(rawXMLData)
		if err != nil {
			return fmt.Errorf("%s cannot be parsed: %v", uri, err)
		}
		return sitemapIndexValidate(sitemapIndex)
	default:
		return fmt.Errorf("%s is not a sitemap: unexpected root element <%s>", uri, root)
	}
}
