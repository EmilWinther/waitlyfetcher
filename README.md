# waitlyfetcher

Fetches Danish housing-association ("andelsboligforening") waiting lists from the
[Waitly](https://waitly.eu) API and generates a self-contained HTML page with a
searchable, filterable list and an interactive [Leaflet](https://leafletjs.com) map.

## Usage

```sh
go run . [flags]
```

| Flag | Default | Description |
| --- | --- | --- |
| `-address` | `a-b-heimdal` | Waitly address/association ID to search around |
| `-locale` | `da` | Locale for the API request |
| `-limit` | `1000` | Maximum number of associations to fetch |
| `-out` | `index.html` | Output HTML file |
| `-timeout` | `30s` | HTTP request timeout |

Example:

```sh
go run . -address a-b-heimdal -limit 500 -out heimdal.html
```

Then open the generated file in a browser. The page works offline apart from the
Leaflet library and map tiles, which load from CDNs.

## Features of the generated page

- Interactive map with a marker and popup for each association that has coordinates
- Search by name, address, ZIP code, or city
- Filter by city and minimum number of rooms
- Sort by name, waiting-list length, unit count, rent, or purchase price
- Waiting-list size ("Ekstern venteliste"), unit count, room range, rent, and
  share price shown per association

## Development

```sh
go test ./...   # run tests
go vet ./...    # static checks
go build        # build the binary
```

The page markup lives in [`page.tmpl.html`](page.tmpl.html) and is embedded into
the binary at build time.
