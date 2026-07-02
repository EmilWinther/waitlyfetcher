# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A single-purpose Go CLI that fetches Danish housing-association ("boligforening") waiting lists from Waitly's undocumented web API and renders a self-contained static `index.html` — an interactive Leaflet map + filterable list of associations. There is no server; the output HTML is the product.

## Commands

```bash
go run .                       # fetch defaults and write index.html
go build -o waitlyfetcher .    # build binary (committed binary is stale; rebuild after changes)
go run . -help                 # show flags

# flags (all optional):
go run . -address a-b-heimdal -locale da -limit 1000 -out index.html
```

There are no tests, linters configured, or module dependencies (stdlib only; `go.mod` lists no requires). Leaflet is loaded via CDN in the generated page.

## Architecture

Three files, one `package main`:

- **main.go** — the whole pipeline: flag parsing → `fetchWaitingLists` (hits `https://waitly.eu/api/similarWaitingLists`) → `flatten` → JSON-marshal → render template to a file.
- **template.go** — one big Go raw-string constant `htmlTemplate`. Contains all CSS and the frontend JS (Leaflet map, search/filter/sort). The place data is injected as `template.JS` into a `RAW` JS array; all filtering/sorting/rendering happens client-side in the browser, not in Go.
- **index.html** — generated output, committed to the repo. Regenerate by running the CLI, don't hand-edit.

Key transformation, `apiItem` → `Place` (`flatten` in main.go): the API response is deeply nested and inconsistent, so `Place` is a flat struct using `*` pointers for every optional numeric field (nil = "unknown", distinct from zero). Notable quirks handled there:
- Unit count is parsed out of a free-text string like `"114 enheder"` via `parseUnits` (regex `\d+`).
- The relevant waiting list is selected by `externalList`, matching the substring `"ekstern"` case-insensitively (Danish "Ekstern venteliste" = external list); naming/casing varies per association.
- `currency` and `address` each have a primary source and a fallback source.

When adding a field to the rendered page you typically touch all three layers: the API struct + `Place` + `flatten` in main.go, and the JS card/popup rendering in template.go.

## Conventions

- Output text and labels are Danish (`lang="da"`, locale defaults to `da`).
- Data injected into the page uses `html/template`'s `template.JS` — keep using it so the JSON is emitted as a JS literal rather than HTML-escaped.
