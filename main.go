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
	"path/filepath"
	"time"
)

// maxSitemapBytes is the uncompressed size limit of the sitemap protocol.
const maxSitemapBytes = 50 << 20

// Exit codes: 0 success, 1 failure, 2 bad usage, 3 urls were removed with -fail-on-dead.
const (
	exitError   = 1
	exitUsage   = 2
	exitRemoved = 3
)

// outputFilePerm is the mode of written sitemap files.
const outputFilePerm = 0644

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

var (
	URI            string
	IsIndex        bool
	OutputFileName string
	Verbose        bool
	Retries        int
	KeepUnverified bool
	FailOnDead     bool
	ShowVersion    bool
)

var transport = &http.Transport{
	Proxy:               http.ProxyFromEnvironment,
	TLSHandshakeTimeout: 10 * time.Second,
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
}

// fetchClient downloads sitemap documents, checkClient probes the urls inside them.
var (
	fetchClient = &http.Client{Transport: transport, Timeout: 100 * time.Second}
	checkClient = &http.Client{Transport: transport, Timeout: 10 * time.Second}
)

func init() {
	flag.StringVar(&URI, "uri", "", "Sitemap uri full path")
	flag.BoolVar(&IsIndex, "index", false, "Is this uri sitemap index file?")
	flag.StringVar(&OutputFileName, "out", "sitemap.xml", "Output file name for valid sitemap file")
	flag.BoolVar(&Verbose, "verbose", false, "Verbose mode")
	flag.IntVar(&Retries, "retries", 2, "Retries for urls that time out, fail to connect, get 429 or 5xx")
	flag.BoolVar(&KeepUnverified, "keep-unverified", false, "Keep urls that could not be verified after all retries")
	flag.BoolVar(&FailOnDead, "fail-on-dead", false, fmt.Sprintf("Exit with code %d when urls were removed from the output", exitRemoved))
	flag.BoolVar(&ShowVersion, "version", false, "Print the version and exit")
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if ShowVersion {
		fmt.Println("sitemap-checker", version)
		return
	}
	if URI == "" {
		usageError("-uri is required")
	}
	if OutputFileName == "" {
		usageError("-out must not be empty")
	}
	if Retries < 0 {
		usageError("-retries must not be negative")
	}

	var stats Stats
	var err error
	if IsIndex {
		if Verbose {
			fmt.Println("Batch process started for index file")
		}
		stats, err = batchProcess(URI)
	} else {
		stats, err = singleProcess(URI, OutputFileName)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitError)
	}

	fmt.Println(stats)
	fmt.Println("Completed")
	if FailOnDead && stats.Dropped > 0 {
		os.Exit(exitRemoved)
	}
}

func usageError(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	flag.Usage()
	os.Exit(exitUsage)
}

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(),
		`Usage: sitemap-checker -uri=http://sitename.com/sitemap.xml -out=sitemap.xml

Writes a copy of the sitemap (or sitemap index) that only has the urls which respond with 200.
Exit codes: 0 success, 1 failure, %d bad usage, %d urls were removed (with -fail-on-dead).

Parameters:
`, exitUsage, exitRemoved)
	flag.PrintDefaults()
}

// doRequest sends a request that identifies the tool.
func doRequest(client *http.Client, method, uri string) (*http.Response, error) {
	req, err := http.NewRequest(method, uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sitemap-checker/"+version)
	return client.Do(req)
}

// fetchXML returns the body of a successful response, at most maxSitemapBytes long.
func fetchXML(uri string) ([]byte, error) {
	resp, err := doRequest(fetchClient, http.MethodGet, uri)
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

// writeFileAtomic writes to a temporary file next to filename and renames it into
// place, so a failed write never leaves a truncated output file behind.
func writeFileAtomic(filename string, data []byte) error {
	f, err := ioutil.TempFile(filepath.Dir(filename), ".sitemap-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()

	_, err = f.Write(data)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(tmp, outputFilePerm)
	}
	if err == nil {
		err = os.Rename(tmp, filename)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}
