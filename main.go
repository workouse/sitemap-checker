package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

// maxSitemapBytes is the uncompressed size limit of the sitemap protocol.
const maxSitemapBytes = 50 << 20

var (
	URI            string
	IsIndex        bool
	OutputFileName string
	Verbose        bool
	Retries        int
	KeepUnverified bool
)

var fetchClient = &http.Client{
	Timeout: 100 * time.Second,
}

func init() {
	flag.StringVar(&URI, "uri", "", "Sitemap uri full path")
	flag.BoolVar(&IsIndex, "index", false, "Is this uri sitemap index file?")
	flag.StringVar(&OutputFileName, "out", "sitemap.xml", "Output file name for valid sitemap file")
	flag.BoolVar(&Verbose, "verbose", false, "Verbose mode")
	flag.IntVar(&Retries, "retries", 2, "Retries for urls that time out, fail to connect, get 429 or 5xx")
	flag.BoolVar(&KeepUnverified, "keep-unverified", false, "Keep urls that could not be verified after all retries")
}
func main() {
	var err error
	flag.Parse()
	if (URI == "" && OutputFileName == "" && IsIndex == false) || (URI == "" && IsIndex == false) {
		help()
	}
	if Verbose {
		fmt.Println(IsIndex)
	}
	if IsIndex {
		if Verbose {
			fmt.Println("Batch process started for index file")
		}
		err = batchProcess(URI)
	} else {
		err = singleProcess(URI, OutputFileName)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println("Completed")
}

// fetchXML returns the body of a successful response, at most maxSitemapBytes long.
func fetchXML(uri string) ([]byte, error) {
	resp, err := fetchClient.Get(uri)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch %s: %v", uri, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cannot fetch %s: status %d", uri, resp.StatusCode)
	}

	data, err := ioutil.ReadAll(io.LimitReader(resp.Body, maxSitemapBytes+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %v", uri, err)
	}
	if len(data) > maxSitemapBytes {
		return nil, fmt.Errorf("%s is larger than %d bytes", uri, maxSitemapBytes)
	}
	return data, nil
}

// rootElement returns the local name of the first element of an XML document.
func rootElement(data []byte) (string, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := d.Token()
		if err != nil {
			return "", err
		}
		if se, ok := tok.(xml.StartElement); ok {
			return se.Name.Local, nil
		}
	}
}

func help() {
	fmt.Printf(
		`You have to type sitemap url and output file name
Usage: checker -uri=http://sitename.com/sitemap.xml -out=sitemap.xml
Parameters:
		-out: (string) output file name for valid xml
		-uri: (string) sitemap or sitemapindex uri
		-index: (bool) uri is sitemapindex or not
		-retries: (int) retries for timeouts, connection errors, 429 and 5xx (default 2)
		-keep-unverified: (bool) keep urls that could not be verified after all retries
`,
	)
	os.Exit(1)
}
