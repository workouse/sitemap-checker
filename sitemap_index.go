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
type SitemapValidation struct {
    IsValid bool
    Sitemap Sitemap
}

func (s Sitemap) findFileName() string {
	u, _ := url.Parse(s.Loc)

	dir := path.Dir(u.Path)

    if dir=="/" {
        dir="."
    }

    filename := u.Path[len(dir):]

	if _, err := os.Stat(dir); os.IsNotExist(err) != false {
		os.MkdirAll(dir, 0777)
	}
	filename = dir + string(os.PathSeparator) + filename
	return filename
}
func (si *SitemapIndex) validate() SitemapIndex {
	validatedSitemapChannel := make(chan SitemapValidation)

    for _, sitemap := range (*si).Sitemap {
        go func(s Sitemap){
            s.validate(validatedSitemapChannel)
        }(sitemap)
    }

	newSitemapIndex := SitemapIndex{
		XMLNs: si.XMLNs,
	}

    for i:=0;i<len((*si).Sitemap);i++ {
        validatedSitemap := <-validatedSitemapChannel
        if validatedSitemap.IsValid {
            newSitemapIndex.Sitemap = append(newSitemapIndex.Sitemap, validatedSitemap.Sitemap)
        }else{
            fmt.Printf("Url is dead: %s\n",validatedSitemap.Sitemap.Loc)
        }
	}

    close(validatedSitemapChannel)

	return newSitemapIndex
}

func (s *Sitemap) validate(sitemapChannel chan SitemapValidation) {

    resp,err := http.Get((*s).Loc)
    if err!=nil {
        fmt.Println(err.Error)
        return
    }

    validateSitemap := SitemapValidation {
        Sitemap: (*s),
        IsValid: true,
    }

    if resp.StatusCode != 200 {
        validateSitemap.IsValid = false;
    }
    sitemapChannel <- validateSitemap

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

	sitemapIndex := newSitemapIndexFromXML(rawXMLData)
    sitemapIndexValidate(sitemapIndex)
}

func sitemapIndexValidate(sitemapIndex SitemapIndex) {
	newSitemapIndex := sitemapIndex.validate()

	for _, sitemap := range newSitemapIndex.Sitemap {
		filename := sitemap.findFileName()
        if Verbose {fmt.Printf("Filename is %s\n",filename)}
		singleProcess(sitemap.Loc, filename)
		time.Sleep(time.Second * 2)
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
