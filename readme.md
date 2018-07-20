## Sitemap Checker

This tool helps you to make sure for url is valid in your sites sitemap file. 

Installation (linux):

- Download released binary
- `sudo mv checker-linux-amd64 /usr/local/bin/sitemap-checker`
- `sudo chmod +x /usr/local/bin/sitemap-checker`

Usage: 

Single sitemap validation:

`sitemap-checker -uri=http://sitename.com/sitemap.xml -out=output.xml `

Sitemap index file validation with connected sitemaps:

`sitemap-checker -uri=http://sitename.com/sitemap.xml -index`

