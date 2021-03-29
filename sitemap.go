package main

import (
	"encoding/xml"
	"fmt"
    "time"
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

type ValidURL struct{
    IsValid bool
    URL URL
    StatusCode int
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
    client := &http.Client{
        Timeout: 10*time.Second,
    }
    validURLChannel := make(chan ValidURL)


	for _, url := range (*us).URL {
		go func(url URL, validURLChannel chan ValidURL) {
			resp, err := client.Get(url.Loc)
            statusCode := (*resp).StatusCode
            validURL := ValidURL {
                IsValid: err == nil && statusCode == 200,
                URL: url,
                StatusCode: statusCode,
            }
            validURLChannel <- validURL
		}(url, validURLChannel)
	}

	newURLSet := URLSet{
		XMLNs: us.XMLNs,
	}

	for range us.URL {
        validURL:= <-validURLChannel
        if validURL.IsValid {
		    newURLSet.URL = append(newURLSet.URL, validURL.URL)
        }else{
            fmt.Printf("Url is dead (%s): %s \n",validURL.StatusCode,validURL.URL.Loc)
        }
	}
        close(validURLChannel)

	return newURLSet
}
//i will use first parameter to determine sitemapIndex or not.
func newURLSetFromXML(rawXMLData []byte) (bool,URLSet) {
	us := URLSet{}

	err := xml.Unmarshal(rawXMLData, &us)

	if err != nil { //some kind of goto
        sitemapIndex := newSitemapIndexFromXML(rawXMLData)
        sitemapIndexValidate(sitemapIndex)
        return true, URLSet{}
	}
	return false,us
}

func singleProcess(uri string, filename string) {
    client := &http.Client{
        Timeout: 10*time.Second,
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
