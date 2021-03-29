package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
)

var (
	URI            string
	IsIndex        bool
	OutputFileName string
    Verbose        bool
)

func init() {
	flag.StringVar(&URI, "uri", "", "Sitemap uri full path")
	flag.BoolVar(&IsIndex, "index", false, "Is this uri sitemap index file?")
	flag.StringVar(&OutputFileName, "out", "sitemap.xml", "Output file name for valid sitemap file")
    flag.BoolVar(&Verbose,"verbose",false,"Verbose mode")
}
func main() {
    flag.Parse()
    if (URI == "" && OutputFileName == "" && IsIndex == false) || (URI == "" && IsIndex == false) {
        help()
    }
    if(Verbose){
        fmt.Println(IsIndex)
    }
    if IsIndex {
        if(Verbose){
            fmt.Println("Batch process started for index file")
        }
        batchProcess(URI)
    } else {
        singleProcess(URI, OutputFileName)
    }

    fmt.Println("Completed")
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

func help() {
	fmt.Printf(
		`You have to type sitemap url and output file name
Usage: checker -uri=http://sitename.com/sitemap.xml -out=sitemap.xml
Parameters:
		-out: (string) output file name for valid xml
		-uri: (string) sitemap or sitemapindex uri
		-index: (bool) uri is sitemapindex or not
`,
	)
	os.Exit(1)
}
