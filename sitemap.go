package main

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
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

func (us *URLSet) validate() URLSet {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	newURLSet := URLSet{
		XMLNs: us.XMLNs,
	}

	n := len((*us).URL)
	for i, url := range (*us).URL {
		// time.Sleep(5 * time.Second)
		resp, err := client.Get(url.Loc)
		if err == nil {
			statusCode := (*resp).StatusCode
			if statusCode == 200 {
				fmt.Printf("Url %d/%d check (%d): %s \n", i, n, statusCode, url.Loc)
				newURLSet.URL = append(newURLSet.URL, url)
			} else {
				fmt.Printf("Url %d/%d dead (%d): %s \n", i, n, statusCode, url.Loc)
			}
		} else {
			fmt.Printf("Url %d/%d error: %s \n", i, n, url.Loc)
		}
	}

	return newURLSet
}

//i will use first parameter to determine sitemapIndex or not.
func newURLSetFromXML(rawXMLData []byte) (bool, URLSet) {
	us := URLSet{}

	err := xml.Unmarshal(rawXMLData, &us)

	if err != nil { //some kind of goto
		sitemapIndex := newSitemapIndexFromXML(rawXMLData)
		sitemapIndexValidate(sitemapIndex)
		return true, URLSet{}
	}
	return false, us
}

func singleProcess(uri string, filename string) {
	client := &http.Client{
		Timeout: 100 * time.Second,
	}

	resp, err := client.Get(uri)
	if err != nil {
		fmt.Printf("Url cannot fetched: %s\n", uri)
		fmt.Println(err)
		os.Exit(1)
	}

	rawXMLData := readXMLFromResponse(resp)

	isJumped, urlSet := newURLSetFromXML(rawXMLData)
	if !isJumped {

		newURLSet := urlSet.validate()

		err = newURLSet.saveToFile(filename)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
}
