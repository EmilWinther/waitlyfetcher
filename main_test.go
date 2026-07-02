package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func intp(n int) *int           { return &n }
func floatp(f float64) *float64 { return &f }

func TestParseUnits(t *testing.T) {
	tests := []struct {
		in   string
		want *int
	}{
		{"114 enheder", intp(114)},
		{"1 enhed", intp(1)},
		{"enheder", nil},
		{"", nil},
	}
	for _, tt := range tests {
		got := parseUnits(tt.in)
		switch {
		case got == nil && tt.want != nil:
			t.Errorf("parseUnits(%q) = nil, want %d", tt.in, *tt.want)
		case got != nil && tt.want == nil:
			t.Errorf("parseUnits(%q) = %d, want nil", tt.in, *got)
		case got != nil && tt.want != nil && *got != *tt.want:
			t.Errorf("parseUnits(%q) = %d, want %d", tt.in, *got, *tt.want)
		}
	}
}

func TestExternalList(t *testing.T) {
	lists := []apiList{
		{Name: "Intern venteliste"},
		{Name: "A/B Heimdal EKSTERN venteliste"},
	}
	got := externalList(lists)
	if got == nil || got.Name != "A/B Heimdal EKSTERN venteliste" {
		t.Fatalf("externalList = %+v, want the external list", got)
	}
	if externalList([]apiList{{Name: "Intern venteliste"}}) != nil {
		t.Error("externalList should return nil when no external list exists")
	}
	if externalList(nil) != nil {
		t.Error("externalList(nil) should return nil")
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate short = %q", got)
	}
	if got := truncate("hello world", 5); got != "hello…" {
		t.Errorf("truncate long = %q", got)
	}
}

func TestFlattenFallbacks(t *testing.T) {
	item := apiItem{ID: 7, Name: "Test"}
	item.Stats.Price.Currency = "DKK"
	item.Stats.Address = "Stats Street 1"
	item.Stats.Apartments = "42 enheder"
	item.Lists = []apiList{{Name: "Ekstern venteliste"}}
	item.Lists[0].Signups.Total = intp(250)
	item.Lists[0].Signups.Active = intp(200)

	places := flatten([]apiItem{item})
	if len(places) != 1 {
		t.Fatalf("flatten returned %d places, want 1", len(places))
	}
	p := places[0]
	if p.Currency != "DKK" {
		t.Errorf("Currency = %q, want fallback to stats price currency", p.Currency)
	}
	if p.Address != "Stats Street 1" {
		t.Errorf("Address = %q, want fallback to stats address", p.Address)
	}
	if p.Units == nil || *p.Units != 42 {
		t.Errorf("Units = %v, want 42", p.Units)
	}
	if p.ExtTotal == nil || *p.ExtTotal != 250 {
		t.Errorf("ExtTotal = %v, want 250", p.ExtTotal)
	}
	if p.ExtActive == nil || *p.ExtActive != 200 {
		t.Errorf("ExtActive = %v, want 200", p.ExtActive)
	}

	// Market currency and input address take precedence when present.
	item.Market.Currency = "EUR"
	item.Address.InputAddress = "Input Street 2"
	p = flatten([]apiItem{item})[0]
	if p.Currency != "EUR" {
		t.Errorf("Currency = %q, want EUR from market", p.Currency)
	}
	if p.Address != "Input Street 2" {
		t.Errorf("Address = %q, want input address", p.Address)
	}
}

func TestFetchWaitingLists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/similarWaitingLists" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("addressId") != "a-b-test" || q.Get("locale") != "da" || q.Get("limitOfItems") != "5" {
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
		w.Write([]byte(`[{"id": 1, "name": "A/B Test", "stats": {"apartments": "10 enheder"}}]`))
	}))
	defer srv.Close()

	items, err := fetchWaitingLists(context.Background(), srv.URL, "a-b-test", "da", 5)
	if err != nil {
		t.Fatalf("fetchWaitingLists: %v", err)
	}
	if len(items) != 1 || items[0].ID != 1 || items[0].Name != "A/B Test" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestFetchWaitingListsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := fetchWaitingLists(context.Background(), srv.URL, "x", "da", 5)
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("want 500 error, got %v", err)
	}
}

func TestFetchWaitingListsBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"not": "an array"}`))
	}))
	defer srv.Close()

	_, err := fetchWaitingLists(context.Background(), srv.URL, "x", "da", 5)
	if err == nil || !strings.Contains(err.Error(), "parse response") {
		t.Fatalf("want parse error, got %v", err)
	}
}

func TestRenderPage(t *testing.T) {
	places := []Place{
		{ID: 1, Name: "A/B <Test>", City: "København", Lat: floatp(55.7), Lng: floatp(12.5), Units: intp(10)},
		{ID: 2, Name: "No Coords"},
	}
	var b strings.Builder
	if err := renderPage(&b, places, "a-b-test"); err != nil {
		t.Fatalf("renderPage: %v", err)
	}
	out := b.String()
	for _, want := range []string{
		"<!DOCTYPE html>",
		"a-b-test",                        // source shown in header
		`<b>2</b><span>Foreninger</span>`, // total count
		`<b>1</b><span>På kort</span>`,    // with-coords count
		`"name":"A/B \u003cTest\u003e"`,   // data embedded, HTML-escaped by json.Marshal
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered page missing %q", want)
		}
	}
}
