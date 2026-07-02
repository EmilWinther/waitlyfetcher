package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// ---- Raw API response types (only the fields we use) ----

type apiItem struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	AreaSlug    string    `json:"area_slug"`
	CompanyType string    `json:"company_type"`
	SignUpURL   string    `json:"sign_up_url"`
	SignUpText  string    `json:"sign_up_text"`
	Stats       apiStat   `json:"stats"`
	Address     apiAddr   `json:"address"`
	Lists       []apiList `json:"lists"`
	Market      struct {
		Currency string `json:"currency"`
	} `json:"market"`
}

type apiList struct {
	Name    string `json:"name"`
	Signups struct {
		Total   *int `json:"total"`
		Active  *int `json:"active"`
		Passive *int `json:"passive"`
	} `json:"signups"`
	PublicSignup int `json:"public_signup"`
}

type apiStat struct {
	Address    string   `json:"address"`
	Apartments string   `json:"apartments"`
	Price      apiPrice `json:"price"`
	Rooms      apiRange `json:"rooms"`
	Floors     apiRange `json:"floors"`
}

type apiPrice struct {
	Onetime   apiMinMaxF `json:"onetime"`
	Recurring apiMinMaxF `json:"recurring"`
	Currency  string     `json:"currency"`
}

type apiMinMaxF struct {
	Min *float64 `json:"min"`
	Max *float64 `json:"max"`
}

type apiRange struct {
	Min *int `json:"min"`
	Max *int `json:"max"`
}

type apiAddr struct {
	InputAddress string   `json:"input_address_text"`
	Zip          string   `json:"zip"`
	Lat          *float64 `json:"lat"`
	Lng          *float64 `json:"lng"`
	City         struct {
		CityName string `json:"city_name"`
	} `json:"city"`
}

// ---- Flattened model passed to the page (JSON-encoded for JS) ----

type Place struct {
	ID         int      `json:"id"`
	Name       string   `json:"name"`
	Address    string   `json:"address"`
	Zip        string   `json:"zip"`
	City       string   `json:"city"`
	Lat        *float64 `json:"lat"`
	Lng        *float64 `json:"lng"`
	Units      *int     `json:"units"`  // parsed from "114 enheder"
	BuyMin     *float64 `json:"buyMin"` // one-time purchase price
	BuyMax     *float64 `json:"buyMax"`
	RentMin    *float64 `json:"rentMin"` // monthly recurring
	RentMax    *float64 `json:"rentMax"`
	RoomsMin   *int     `json:"roomsMin"`
	RoomsMax   *int     `json:"roomsMax"`
	ExtTotal   *int     `json:"extTotal"`  // "Ekstern venteliste" signups.total
	ExtActive  *int     `json:"extActive"` // active signups on the external list
	Currency   string   `json:"currency"`
	URL        string   `json:"url"`
	SignUpText string   `json:"signUpText"`
}

var leadingInt = regexp.MustCompile(`\d+`)

func parseUnits(s string) *int {
	m := leadingInt.FindString(s)
	if m == "" {
		return nil
	}
	if n, err := strconv.Atoi(m); err == nil {
		return &n
	}
	return nil
}

// externalList returns the association's external waiting list ("Ekstern
// venteliste"), matched case-insensitively since names vary in casing and
// are sometimes prefixed with the association name.
func externalList(lists []apiList) *apiList {
	for i := range lists {
		if strings.Contains(strings.ToLower(lists[i].Name), "ekstern") {
			return &lists[i]
		}
	}
	return nil
}

func flatten(items []apiItem) []Place {
	out := make([]Place, 0, len(items))
	for _, it := range items {
		currency := it.Market.Currency
		if currency == "" {
			currency = it.Stats.Price.Currency
		}
		addr := it.Address.InputAddress
		if addr == "" {
			addr = it.Stats.Address
		}
		var extTotal, extActive *int
		if ext := externalList(it.Lists); ext != nil {
			extTotal = ext.Signups.Total
			extActive = ext.Signups.Active
		}
		out = append(out, Place{
			ID:         it.ID,
			Name:       it.Name,
			Address:    addr,
			Zip:        it.Address.Zip,
			City:       it.Address.City.CityName,
			Lat:        it.Address.Lat,
			Lng:        it.Address.Lng,
			Units:      parseUnits(it.Stats.Apartments),
			BuyMin:     it.Stats.Price.Onetime.Min,
			BuyMax:     it.Stats.Price.Onetime.Max,
			RentMin:    it.Stats.Price.Recurring.Min,
			RentMax:    it.Stats.Price.Recurring.Max,
			RoomsMin:   it.Stats.Rooms.Min,
			RoomsMax:   it.Stats.Rooms.Max,
			ExtTotal:   extTotal,
			ExtActive:  extActive,
			Currency:   currency,
			URL:        it.SignUpURL,
			SignUpText: it.SignUpText,
		})
	}
	return out
}

// ---- fetch ----

func fetchWaitingLists(addressID, locale string, limit int) ([]apiItem, error) {
	url := fmt.Sprintf(
		"https://waitly.eu/api/similarWaitingLists?addressId=%s&limitOfItems=%d&locale=%s",
		addressID, limit, locale,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "en-GB,en;q=0.6")
	req.Header.Set("referer", fmt.Sprintf("https://waitly.eu/%s/foreninger/0/%s", locale, addressID))
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-origin")
	req.Header.Set("user-agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36")
	req.AddCookie(&http.Cookie{Name: "i18n_redirected", Value: locale})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	var items []apiItem
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("parse response: %w (body: %s)", err, truncate(string(body), 300))
	}
	return items, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ---- main ----

func main() {
	addressID := "a-b-heimdal"
	locale := "da"
	limit := 1000
	outFile := "index.html"

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-address":
			if i+1 < len(args) {
				addressID = args[i+1]
				i++
			}
		case "-locale":
			if i+1 < len(args) {
				locale = args[i+1]
				i++
			}
		case "-limit":
			if i+1 < len(args) {
				limit, _ = strconv.Atoi(args[i+1])
				i++
			}
		case "-out":
			if i+1 < len(args) {
				outFile = args[i+1]
				i++
			}
		case "-h", "-help", "--help":
			fmt.Println("Usage: waitlyfetcher [-address ID] [-locale da] [-limit 1000] [-out index.html]")
			return
		}
	}

	log.Printf("Fetching waiting lists for %q (locale=%s, limit=%d)…", addressID, locale, limit)

	items, err := fetchWaitingLists(addressID, locale, limit)
	if err != nil {
		log.Fatalf("fetch error: %v", err)
	}
	log.Printf("Got %d associations", len(items))

	places := flatten(items)
	rawJSON, err := json.Marshal(places)
	if err != nil {
		log.Fatalf("json marshal: %v", err)
	}

	withCoords := 0
	for _, p := range places {
		if p.Lat != nil && p.Lng != nil {
			withCoords++
		}
	}

	data := struct {
		Total      int
		WithCoords int
		Source     string
		RawJSON    template.JS
	}{
		Total:      len(places),
		WithCoords: withCoords,
		Source:     addressID,
		RawJSON:    template.JS(rawJSON),
	}

	f, err := os.Create(outFile)
	if err != nil {
		log.Fatalf("create file: %v", err)
	}
	defer f.Close()

	tmpl := template.Must(template.New("page").Parse(htmlTemplate))
	if err := tmpl.Execute(f, data); err != nil {
		log.Fatalf("render: %v", err)
	}

	log.Printf("Wrote %s (%d associations, %d with coordinates)", outFile, len(places), withCoords)
}
