package main

import (
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
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

func (s Sitemap) findFileName() string {
	u, _ := url.Parse(s.Loc)
	dir := path.Dir(u.Path)[1:]
	filename := u.Path[len(dir)+1+1:]

	if _, err := os.Stat(dir); os.IsNotExist(err) != false {
		os.MkdirAll(dir, 0777)
	}
	filename = dir + string(os.PathSeparator) + filename
	return filename
}
func (si *SitemapIndex) validate() SitemapIndex {
	c := make(chan string)
	validSitemaps := []Sitemap{}

	for _, sitemap := range (*si).Sitemap {
		go func(sitemap Sitemap, c chan string) {
			resp, err := http.Get(sitemap.Loc)
			if err != nil {
				c <- err.Error()
			}
			c <- fmt.Sprintf("Response code is %d for %s", resp.StatusCode, sitemap.Loc)
			if resp.StatusCode == 200 {
				validSitemaps = append(validSitemaps, sitemap)
			}
		}(sitemap, c)
	}

	for range si.Sitemap {
		fmt.Println(<-c)
	}
	newSitemapIndex := SitemapIndex{
		XMLNs: si.XMLNs,
	}
	for _, sitemap := range validSitemaps {
		newSitemapIndex.Sitemap = append(newSitemapIndex.Sitemap, sitemap)
	}
	return newSitemapIndex
}

func (si *SitemapIndex) saveToFile(filename string) error {
	m, err := xml.Marshal((*si))
	if err != nil {
		return err
	}

	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	file.Write([]byte(xml.Header))
	file.Write(m)
	file.Close()
	return err
}

func batchProcess(uri string) {
	resp, err := http.Get(uri)
	if err != nil {
		log.Printf("Url cannot fetched: %s\n", uri)
		log.Println(err)
		os.Exit(1)
	}

	rawXMLData := readXMLFromResponse(resp)

	sitemapIndex := newSitemapIndexFromXML(rawXMLData)
	newSitemapIndex := sitemapIndex.validate()

	for _, sitemap := range newSitemapIndex.Sitemap {
		time.Sleep(time.Second * 2)
		filename := sitemap.findFileName()
		singleProcess(sitemap.Loc, filename)
	}

	newSitemapIndex.saveToFile(OutputFileName)
}

func newSitemapIndexFromXML(rawXMLData []byte) SitemapIndex {
	sm := SitemapIndex{}
	err := xml.Unmarshal(rawXMLData, &sm)

	if err != nil {
		log.Printf("Sitemap index cannot parsed. Because: %s", err)
		return SitemapIndex{}
	}
	return sm
}
