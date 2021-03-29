package main

import (
	"encoding/xml"
	"fmt"
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
	logChannel := make(chan string)
	validSitemapChannel := make(chan Sitemap)

    go func() {
        for _, sitemap := range (*si).Sitemap {
            sitemap.validate(logChannel,validSitemapChannel)
        }
        if Verbose {fmt.Println("Validation done")}
        close(logChannel)
        close(validSitemapChannel)
    }()

    go func() { 
        for {
            logMsg,isLogChannelOpen := <-logChannel
            if !isLogChannelOpen {
                break
            }
            fmt.Println(logMsg)
        }
    }()

    
	newSitemapIndex := SitemapIndex{
		XMLNs: si.XMLNs,
	}

	for {
        if Verbose { fmt.Println("Waits for sitemap data") }
        validSitemap, isValidSitemapChannelOpen := <-validSitemapChannel
        if !isValidSitemapChannelOpen {
            break
        }
		newSitemapIndex.Sitemap = append(newSitemapIndex.Sitemap, validSitemap)
	}

	return newSitemapIndex
}

func (s *Sitemap) validate(logChannel chan string,sitemapChannel chan Sitemap) {
    resp,err := http.Get((*s).Loc)
    if err!=nil {
        logChannel <- err.Error()
        return
    }
    logChannel <- fmt.Sprintf("Response code is %d for %s", resp.StatusCode, (*s).Loc)
    if resp.StatusCode == 200 {
       if Verbose { fmt.Println("Sitemap returning to channel") }
       sitemapChannel <- (*s)
       if Verbose { fmt.Println("Sitemap returned to channel") }
    }
    return
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
		fmt.Printf("Url cannot fetched: %s\n", uri)
		fmt.Println(err)
		os.Exit(1)
	}

	rawXMLData := readXMLFromResponse(resp)
    if Verbose {fmt.Printf("XML readed from response\n")}

	sitemapIndex := newSitemapIndexFromXML(rawXMLData)
    if Verbose {fmt.Printf("New sitemap created\n")}
	newSitemapIndex := sitemapIndex.validate()
    if Verbose {fmt.Printf("Sitemap validated\n")}

	for _, sitemap := range newSitemapIndex.Sitemap {
        if Verbose  {fmt.Printf("Wait for 2 sec.\n")}
		time.Sleep(time.Second * 2)
		filename := sitemap.findFileName()
        if Verbose {fmt.Printf("Filename is %s\n",filename)}
		singleProcess(sitemap.Loc, filename)
	}

	newSitemapIndex.saveToFile(OutputFileName)
}

func newSitemapIndexFromXML(rawXMLData []byte) SitemapIndex {
	sm := SitemapIndex{}
	err := xml.Unmarshal(rawXMLData, &sm)

	if err != nil {
		fmt.Printf("Sitemap index cannot parsed. Because: %s", err)
		return SitemapIndex{}
	}
	return sm
}
