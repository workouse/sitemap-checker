# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

- Build: `go build -o sitemap-checker` (module `sitemap-checker`, `go 1.13`; the release workflow builds with Go 1.13, so avoid newer language/stdlib features)
- Cross-compile all platforms into `build/`: `make` (uses `.version` for the filename suffix; `make build-linux|build-windows|build-darwin` for one)
- Test: `go test ./...`; single test: `go test -run ^TestName$` (tests live in `sitemap_index_test.go` and `sitemap_test.go` and use `httptest` servers)
- Format/lint: `go fmt ./...`, `goimports -w .`, `golangci-lint run`
- Run: `./sitemap-checker -uri=http://site/sitemap.xml -out=out.xml`, or add `-index` when the URI is a sitemap index. `-verbose` prints extra progress.
- Releases are triggered by pushing a `v*.*.*` tag (`.github/workflows/release.yml`); `.version` is only used by the Makefile.

## Architecture

A small flat `package main` CLI (three files, no subpackages, stdlib only) that fetches a sitemap, HEAD-less `GET`s every `<loc>`, and writes a new sitemap containing only URLs that returned HTTP 200.

- `main.go` — flag parsing into package-level globals (`URI`, `IsIndex`, `OutputFileName`, `Verbose`, `Retries`, `KeepUnverified`), dispatch to `batchProcess` (index) or `singleProcess` (urlset), plus `fetchXML` (shared fetch: requires HTTP 200, reads at most 50MB) and `rootElement` (detects `urlset` vs `sitemapindex`).
- `sitemap.go` — `URLSet`/`URL` XML types, `validate()` (bounded worker pool: 10 concurrent requests, 10s timeout, results appended under a mutex), `saveToFile`, and `singleProcess`.
- `sitemap_index.go` — `SitemapIndex`/`Sitemap` XML types, concurrent validation of each child sitemap URL, and `batchProcess`/`sitemapIndexValidate`.

### Control flow worth knowing (spans files)

- Single mode: `singleProcess` fetches via `fetchXML`, picks the path by root element (`urlset` → validate and write; `sitemapindex` → `sitemapIndexValidate`, so `-index` is effectively optional; anything else is an error). Functions return errors and only `main` exits (code 1); nothing is written when the fetch or parse fails.
- URL checks (`checkURL`) retry timeouts, connection errors, 429 and 5xx up to `-retries` times with exponential backoff. URLs still unverifiable afterwards are dropped unless `-keep-unverified`; 4xx (not 429) is dead immediately. Output keeps input order.
- Index mode: `sitemapIndexValidate` first checks each child sitemap is reachable (200), then for each live child calls `singleProcess(sitemap.Loc, filename)` — where `filename` comes from `Sitemap.findFileName()`, which **mirrors the URL path onto a path relative to the working directory** (always cleaned and rooted so remote input can't escape it; children that would overwrite `-out` are skipped) — sleeping 2s between children. A child whose processing fails is dropped from the index. Finally the filtered index is written to `OutputFileName` (default `sitemap.xml`).
- The index path is therefore recursive-ish through `singleProcess`, and uses `OutputFileName`/`Verbose` globals rather than parameters.

### Known quirks (don't assume they're intentional)

- `saveToFile` in both files writes before checking the `OpenFile` error.
