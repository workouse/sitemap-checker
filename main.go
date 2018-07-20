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

func main() {
	if len(os.Args) < 3 {
		help()
	}
	sitemapURL := os.Args[1]
	outputFileName := os.Args[2]
	resp, err := http.Get(sitemapURL)
	if err != nil {
		log.Printf("Urls cannot fetched: %s\n", sitemapURL)
		log.Println(err)
		os.Exit(1)
	}
	rawXMLData := readXMLFromResponse(resp)
	urlSet := URLSet{}

	err = xml.Unmarshal(rawXMLData, &urlSet)
	if err != nil {
		log.Printf("Sitemap cannot parsed. Because: %s", err)
		os.Exit(1)
	}
	c := make(chan string)
	validURLs := []URL{}
	for _, url := range urlSet.URL {
		go checkURL(url, c, &validURLs)
	}

	for range urlSet.URL {
		fmt.Println(<-c)
	}

	newURLSet := URLSet{
		XMLNs: urlSet.XMLNs,
	}
	for _, url := range validURLs {
		newURLSet.URL = append(newURLSet.URL, url)
	}
	newRawXML, err := xml.Marshal(newURLSet)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = saveValidSiteMap(outputFileName, newRawXML)
	if err != nil {
		fmt.Println("I can`₺ write valid sitemap. Error: ", err)
		os.Exit(1)
	}
	fmt.Println("File writed to ", outputFileName, "and closed")
}

func readXMLFromResponse(resp *http.Response) []byte {
	var rawXMLData []byte
	for {
		content := make([]byte, 1024)
		n, _ := resp.Body.Read(content)
		for _, d := range content {
			rawXMLData = append(rawXMLData, d)
		}
		if n == 0 {
			break
		}
	}
	return rawXMLData
}
func checkURL(url URL, c chan string, validURLs *[]URL) {
	resp, err := http.Get(url.Loc)
	if err != nil {
		c <- err.Error()
	}
	c <- fmt.Sprintf("Response code is %d for %s", resp.StatusCode, url)
	if resp.StatusCode == 200 {
		(*validURLs) = append((*validURLs), url)
	}
}
func saveValidSiteMap(filename string, data []byte) error {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	file.Write([]byte(xml.Header))
	file.Write(data)
	file.Close()
	return err
}

func help() {
	fmt.Printf(
		`You have to type sitemap url and output file name
Usage: checker http://sitename.com/sitemap.xml sitemap.xml
`,
	)
	os.Exit(1)
}
