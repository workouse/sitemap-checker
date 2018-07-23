package main

import (
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"os"
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

func (us *URLSet) saveToFile(filename string) error {
	m, err := xml.Marshal((*us))
	if err != nil {
		return err
	}

	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	file.Write([]byte(xml.Header))
	file.Write(m)
	file.Close()
	return err
}

func (us *URLSet) validate() URLSet {
	c := make(chan string, 20)

	validURLs := []URL{}
	for _, url := range (*us).URL {
		go func(url URL, c chan string) {
			resp, err := http.Get(url.Loc)
			defer func() { <-c }()
			if err != nil {
				c <- err.Error()
				return
			}
			c <- fmt.Sprintf("Response code is %d for %s", resp.StatusCode, url.Loc)
			if resp.StatusCode == 200 {
				validURLs = append(validURLs, url)
			}
		}(url, c)
	}

	for range us.URL {
		fmt.Println(<-c)
	}
	newURLSet := URLSet{
		XMLNs: us.XMLNs,
	}
	for _, url := range validURLs {
		newURLSet.URL = append(newURLSet.URL, url)
	}
	return newURLSet
}

func newURLSetFromXML(rawXMLData []byte) URLSet {
	us := URLSet{}

	err := xml.Unmarshal(rawXMLData, &us)

	if err != nil {
		log.Printf("Sitemap cannot parsed. Because: %s", err)
		return URLSet{}
	}
	return us
}

func singleProcess(uri string, filename string) {
	resp, err := http.Get(uri)
	if err != nil {
		log.Printf("Url cannot fetched: %s\n", uri)
		log.Println(err)
		os.Exit(1)
	}

	rawXMLData := readXMLFromResponse(resp)

	urlSet := newURLSetFromXML(rawXMLData)

	newURLSet := urlSet.validate()

	err = newURLSet.saveToFile(filename)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
