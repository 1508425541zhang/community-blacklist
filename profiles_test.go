package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPublicProfilePrivacyGroupingAndPagination(t *testing.T) {
	id := "76561190000000001"
	store := &fileStore{path: filepath.Join(t.TempDir(), "profile-fixture.json")}
	future := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	past := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	records := []Record{
		{ID: "active", Account: id, Player: "Public player", Community: "A", Status: "active", Public: true, UpdatedAt: "2025-01-01T00:00:00Z", ExpiresAt: future},
		{ID: "expired", Account: id, Community: "A", Status: "active", Public: true, ExpiresAt: past},
		{ID: "pending", Account: id, Community: "B", Status: "needvotes", Public: true},
		{ID: "revoked", Account: id, Community: "B", Status: "revoked", Public: true},
		{ID: "SECRET-CASE", Account: id, Player: "SECRET-NAME", Community: "SECRET-COMMUNITY", Status: "active", Public: false, UpdatedAt: "2099-01-01T00:00:00Z"},
		{ID: "different-account", Account: "76561190000000002", Player: id, Community: "Other", Status: "active", Public: true},
	}
	if err := store.Save(context.Background(), State{Records: records, Users: []User{{Name: "admin", Role: "admin"}}}, 0); err != nil {
		t.Fatal(err)
	}
	app := newApp(store)
	cookie, _ := fixtureSession(app, "admin")
	read := func(path string, authenticated bool) (int, string, map[string]any) {
		r := httptest.NewRequest("GET", "http://localhost"+path, nil)
		if authenticated {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		var body map[string]any
		json.Unmarshal(w.Body.Bytes(), &body)
		return w.Code, w.Body.String(), body
	}
	for _, authenticated := range []bool{false, true} {
		code, raw, body := read("/api/v1/profiles/"+id, authenticated)
		if code != 200 || body["publicTotal"] != float64(4) || body["displayName"] != "Public player" {
			t.Fatal(code, body)
		}
		if strings.Contains(raw, "SECRET-") || strings.Contains(raw, "different-account") {
			t.Fatal("private or other-account data leaked")
		}
		counts := body["counts"].(map[string]any)
		for _, key := range []string{"active", "expired", "pending", "resolved"} {
			if counts[key] != float64(1) {
				t.Fatal(counts)
			}
		}
		if len(body["communities"].([]any)) != 2 {
			t.Fatal(body)
		}
	}
	code, _, body := read("/api/v1/profiles/"+id+"?group=expired&size=1", false)
	if code != 200 || body["total"] != float64(1) || body["items"].([]any)[0].(map[string]any)["id"] != "expired" {
		t.Fatal(code, body)
	}
	code, _, body = read("/api/v1/profiles/"+id+"?page=2&size=1", false)
	if code != 200 || body["total"] != float64(4) || len(body["items"].([]any)) != 1 {
		t.Fatal(code, body)
	}
	code, _, body = read("/api/v1/profiles/76561190000000003", false)
	if code != 200 || body["publicTotal"] != float64(0) || body["displayName"] != "" || len(body["items"].([]any)) != 0 {
		t.Fatal(code, body)
	}
	for _, suffix := range []string{"?group=unknown", "?size=51", "?size=-1", "?page=12abc", "?page=1&page=2", "?page=1000001"} {
		if code, _, _ := read("/api/v1/profiles/"+id+suffix, false); code != 400 {
			t.Fatal(suffix, code)
		}
	}
	for _, account := range []string{"123", id + "/extra", id + "0"} {
		if code, _, _ := read("/api/v1/profiles/"+account, false); code != 400 {
			t.Fatal(account, code)
		}
	}
	r := httptest.NewRequest(http.MethodPost, "http://localhost/api/v1/profiles/"+id, nil)
	r.Header.Set("Origin", "http://localhost")
	w := httptest.NewRecorder()
	app.ServeHTTP(w, r)
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
}

func TestProfileExpiryBoundaryAndUnknownDates(t *testing.T) {
	at := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct{ status, expiry, want string }{
		{"active", "", "active"}, {"active", at.Format(time.RFC3339), "expired"},
		{"active", at.Add(time.Second).Format(time.RFC3339), "active"}, {"active", "bad-date", "resolved"},
		{"appealing", at.Add(-time.Hour).Format(time.RFC3339), "pending"}, {"boosting", "", "resolved"},
	} {
		if got := profileGroup(Record{Status: tc.status, ExpiresAt: tc.expiry}, at); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}
