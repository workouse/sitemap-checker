package main

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type SitemapIndex struct {
	XMLName xml.Name  `xml:"sitemapindex"`
	XMLNs   string    `xml:"xmlns,attr"`
	Sitemap []Sitemap `xml:"sitemap"`
}
type Sitemap struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}
type SitemapValidation struct {
	IsValid bool
	Sitemap Sitemap
	Err     error
}

// findFileName maps the sitemap url path to a file path relative to the
// working directory. The url comes from a remote document, so the result must
// never be absolute or escape the working directory.
func (s Sitemap) findFileName() (string, error) {
	u, err := url.Parse(s.Loc)
	if err != nil {
		return "", fmt.Errorf("cannot parse sitemap url %q: %w", s.Loc, err)
	}

	// Cleaning the rooted path drops any ".." elements that would climb above it.
	name := path.Clean("/" + u.Path)
	if name == "/" {
		return "", fmt.Errorf("sitemap url %q has no file name", s.Loc)
	}
	if strings.Contains(name, `\`) {
		return "", fmt.Errorf("sitemap url %q has an unsupported path", s.Loc)
	}

	filename := filepath.FromSlash(name[1:])
	if filepath.IsAbs(filename) || filepath.VolumeName(filename) != "" {
		return "", fmt.Errorf("sitemap url %q has an unsupported path", s.Loc)
	}
	return filename, nil
}

func (si *SitemapIndex) validate() SitemapIndex {
	results := make([]SitemapValidation, len(si.Sitemap))

	// Limit the number of concurrent requests
	maxConcurrentRequests := 10
	sem := make(chan struct{}, maxConcurrentRequests)

	var wg sync.WaitGroup
	for i := range si.Sitemap {
		wg.Add(1)
		sem <- struct{}{} // Acquire a semaphore slot

		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }() // Release the semaphore slot

			results[i] = si.Sitemap[i].validate(checkClient)
		}(i)
	}
	wg.Wait()

	newSitemapIndex := SitemapIndex{
		XMLNs: si.XMLNs,
	}

	for _, r := range results {
		if r.IsValid {
			newSitemapIndex.Sitemap = append(newSitemapIndex.Sitemap, r.Sitemap)
		} else {
			fmt.Printf("Url is dead: %s (%v)\n", r.Sitemap.Loc, r.Err)
		}
	}

	return newSitemapIndex
}

// validate always returns a result, so a failed request cannot stall the caller.
func (s *Sitemap) validate(client *http.Client) SitemapValidation {
	result := SitemapValidation{Sitemap: *s}

	status, err := checkURL(client, s.Loc)
	if err == nil && status != http.StatusOK {
		err = fmt.Errorf("status %d", status)
	}
	if err != nil {
		result.Err = err
		return result
	}

	result.IsValid = true
	return result
}

func (si *SitemapIndex) saveToFile(filename string) error {
	m, err := xml.MarshalIndent(si, "", "  ")
	if err != nil {
		return err
	}
	data := append([]byte(xml.Header), m...)
	data = append(data, '\n')
	return writeFileAtomic(filename, data)
}

func batchProcess(uri string) (Stats, error) {
	rawXMLData, err := fetchXML(uri)
	if err != nil {
		return Stats{}, err
	}

	sitemapIndex, err := newSitemapIndexFromXML(rawXMLData)
	if err != nil {
		return Stats{}, fmt.Errorf("%s is not a sitemap index: %v", uri, err)
	}
	return sitemapIndexValidate(sitemapIndex)
}

func sitemapIndexValidate(sitemapIndex SitemapIndex) (Stats, error) {
	newSitemapIndex := sitemapIndex.validate()
	processed := SitemapIndex{XMLNs: newSitemapIndex.XMLNs}
	var total Stats

	for _, sitemap := range newSitemapIndex.Sitemap {
		filename, err := sitemap.findFileName()
		if err != nil {
			fmt.Printf("Skipping %s: %s\n", sitemap.Loc, err)
			processed.Sitemap = append(processed.Sitemap, sitemap)
			continue
		}
		if filepath.Clean(filename) == filepath.Clean(OutputFileName) {
			fmt.Printf("Skipping %s: it would overwrite the output file %s\n", sitemap.Loc, OutputFileName)
			processed.Sitemap = append(processed.Sitemap, sitemap)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
			fmt.Printf("Skipping %s: %s\n", sitemap.Loc, err)
			processed.Sitemap = append(processed.Sitemap, sitemap)
			continue
		}
		if Verbose {
			fmt.Printf("Filename is %s\n", filename)
		}
		stats, err := singleProcess(sitemap.Loc, filename)
		total.add(stats)
		if err != nil {
			fmt.Printf("Dropping %s: %s\n", sitemap.Loc, err)
		} else {
			processed.Sitemap = append(processed.Sitemap, sitemap)
		}
		time.Sleep(time.Second * 2)
	}

	if len(processed.Sitemap) == 0 {
		return total, fmt.Errorf("no live sitemaps in the index; %s was not written", OutputFileName)
	}
	return total, processed.saveToFile(OutputFileName)
}

func newSitemapIndexFromXML(rawXMLData []byte) (SitemapIndex, error) {
	sm := SitemapIndex{}
	if err := xml.Unmarshal(rawXMLData, &sm); err != nil {
		return SitemapIndex{}, err
	}
	return sm, nil
}
