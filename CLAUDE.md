# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

- Build: `go build -o sitemap-checker` (module `sitemap-checker`, `go 1.13`; the release workflow builds with Go 1.13, so avoid newer language/stdlib features)
- Cross-compile all platforms into `build/`: `make` (uses `.version` for the filename suffix; `make build-linux|build-windows|build-darwin` for one)
- Test: `go test ./...`; single test: `go test -run ^TestName$` (there are currently no test files)
- Format/lint: `go fmt ./...`, `goimports -w .`, `golangci-lint run`
- Run: `./sitemap-checker -uri=http://site/sitemap.xml -out=out.xml`, or add `-index` when the URI is a sitemap index. `-verbose` prints extra progress.
- Releases are triggered by pushing a `v*.*.*` tag (`.github/workflows/release.yml`); `.version` is only used by the Makefile.

## Architecture

A small flat `package main` CLI (three files, no subpackages, stdlib only) that fetches a sitemap, HEAD-less `GET`s every `<loc>`, and writes a new sitemap containing only URLs that returned HTTP 200.

- `main.go` — flag parsing into package-level globals (`URI`, `IsIndex`, `OutputFileName`, `Verbose`), dispatch to `batchProcess` (index) or `singleProcess` (urlset), plus `readXMLFromResponse`, the shared body reader.
- `sitemap.go` — `URLSet`/`URL` XML types, `validate()` (bounded worker pool: 10 concurrent requests, 10s timeout, results appended under a mutex), `saveToFile`, and `singleProcess`.
- `sitemap_index.go` — `SitemapIndex`/`Sitemap` XML types, concurrent validation of each child sitemap URL, and `batchProcess`/`sitemapIndexValidate`.

### Control flow worth knowing (spans files)

- Single mode: `singleProcess` → `newURLSetFromXML`. If the XML fails to unmarshal as a `<urlset>`, it **falls back to treating the document as a sitemap index** (`newSitemapIndexFromXML` + `sitemapIndexValidate`) and returns `isJumped=true`, so `singleProcess` skips writing output. So `-index` is effectively optional.
- Index mode: `sitemapIndexValidate` first checks each child sitemap is reachable (200), then for each live child calls `singleProcess(sitemap.Loc, filename)` — where `filename` comes from `Sitemap.findFileName()`, which **mirrors the URL path onto the local filesystem** (creating directories) — sleeping 2s between children. Finally the filtered index is written to `OutputFileName` (default `sitemap.xml`).
- The index path is therefore recursive-ish through `singleProcess`, and uses `OutputFileName`/`Verbose` globals rather than parameters.

### Known quirks (don't assume they're intentional)

- `readXMLFromResponse` appends the whole 1024-byte buffer each iteration (not `content[:n]`), so the returned data is padded with trailing NUL bytes, and ignores read errors.
- `Sitemap.validate` in `sitemap_index.go` returns without sending on the channel when the request errors, which would block the collector loop in `SitemapIndex.validate`; it also never closes response bodies and uses `http.Get` without a timeout.
- `saveToFile` in both files writes before checking the `OpenFile` error.
