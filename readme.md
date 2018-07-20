## Sitemap Checker

[![Maintainability](https://api.codeclimate.com/v1/badges/04358a82abbbc1f7bb2c/maintainability)](https://codeclimate.com/github/delirehberi/sitemap-checker/maintainability)
[![Test Coverage](https://api.codeclimate.com/v1/badges/04358a82abbbc1f7bb2c/test_coverage)](https://codeclimate.com/github/delirehberi/sitemap-checker/test_coverage)

This tool helps you to make sure for url is valid in your sites sitemap file. 

#### Installation (linux):

- Download released binary
- `sudo mv checker-linux-amd64 /usr/local/bin/sitemap-checker`
- `sudo chmod +x /usr/local/bin/sitemap-checker`

### Usage: 

Single sitemap validation:

`sitemap-checker -uri=http://sitename.com/sitemap.xml -out=output.xml `

Sitemap index file validation with connected sitemaps:

`sitemap-checker -uri=http://sitename.com/sitemap.xml -index`

