package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAccountOwnershipAndSessionRevocation(t *testing.T) {
	store := &fileStore{path: filepath.Join(t.TempDir(), "state.json")}
	seed := State{Users: []User{{Name: "alice", Role: "member", SteamID: "76561190000000001"}, {Name: "bob", Role: "member", SteamID: "76561190000000002"}},
		Records:     []Record{{ID: "mine", Reporter: "alice", Account: "76561190000000002", Status: "pending", Community: "Test", Public: true}, {ID: "theirs", Reporter: "bob", Account: "76561190000000001", Status: "active", Community: "Test", Public: true}, {ID: "hidden", Reporter: "alice", Community: "Test", Evidence: "Alice-owned hidden evidence", Public: false}},
		Appeals:     []Appeal{{ID: "own-appeal", Account: "76561190000000001", RecordID: "theirs", Status: "open", Content: "Alice-only appeal"}, {ID: "other-appeal", Account: "76561190000000002", Content: "Bob private appeal"}},
		Communities: []Community{{ID: "community", Name: "Test", Approved: true}, {ID: "alice-app", Name: "Alice community", Applicant: "alice", Contact: "Alice private contact"}, {ID: "bob-app", Name: "Bob community", Applicant: "bob", Contact: "Bob private contact"}}}
	if err := store.Save(context.Background(), seed, 0); err != nil {
		t.Fatal(err)
	}
	app := newApp(store)
	a1, c1 := fixtureSession(app, "alice")
	a2, _ := fixtureSession(app, "alice")
	b1, bCSRF := fixtureSession(app, "bob")
	call := func(method, path string, body any, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, "http://localhost/api/v1/"+path, bytes.NewReader(raw))
		r.Header.Set("Origin", "http://localhost")
		r.Header.Set("X-CSRF-Token", csrf)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "account-summary", nil, nil, ""); w.Code != 401 {
		t.Fatal("anonymous account read", w.Code)
	}
	w := call("GET", "account-summary?user=bob", nil, a1, c1)
	if w.Code != 200 || strings.Contains(w.Body.String(), "Bob private") || strings.Contains(w.Body.String(), `"id":"other-appeal"`) || !strings.Contains(w.Body.String(), "Alice private contact") {
		t.Fatal("account ownership failed", w.Code, w.Body.String())
	}
	if w = call("GET", "player/hidden", nil, a1, c1); w.Code != 200 {
		t.Fatal("owner cannot read hidden submission", w.Code)
	}
	if w = call("GET", "player/hidden", nil, b1, bCSRF); w.Code != 404 {
		t.Fatal("foreign user read hidden submission", w.Code)
	}
	if w = call("GET", "player/hidden", nil, nil, ""); w.Code != 404 {
		t.Fatal("anonymous read hidden submission", w.Code)
	}
	w = call("GET", "communities", nil, nil, "")
	if strings.Contains(w.Body.String(), "contact") || strings.Contains(w.Body.String(), "applicant") {
		t.Fatal("directory leaked private fields", w.Body.String())
	}
	w = call("GET", "sessions", nil, a1, c1)
	if w.Code != 200 || strings.Contains(w.Body.String(), a1.Value) || strings.Contains(w.Body.String(), a2.Value) || strings.Contains(w.Body.String(), b1.Value) || strings.Contains(w.Body.String(), c1) {
		t.Fatal("session secret leaked", w.Code, w.Body.String())
	}
	var list struct {
		Items []struct {
			ID      string `json:"id"`
			Current bool   `json:"current"`
		} `json:"items"`
	}
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 2 {
		t.Fatal("foreign session listed", list)
	}
	if w = call("POST", "sessions/revoke", map[string]string{"id": sessionLabel(b1.Value)}, a1, c1); w.Code != 404 {
		t.Fatal("foreign session revoked", w.Code)
	}
	if w = call("POST", "sessions/revoke-others", nil, a1, "wrong"); w.Code != 403 {
		t.Fatal("CSRF accepted", w.Code)
	}
	w = call("POST", "sessions/revoke-others", nil, a1, c1)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = call("GET", "me", nil, a2, ""); w.Code != 401 {
		t.Fatal("revoked session works", w.Code)
	}
	if w = call("GET", "me", nil, a1, c1); w.Code != 200 {
		t.Fatal("current session revoked", w.Code)
	}
	if w = call("GET", "me", nil, b1, bCSRF); w.Code != 200 {
		t.Fatal("other user affected", w.Code)
	}
	application := Community{Name: "New application", Applicant: "bob", Approved: true, Description: "Synthetic application for local tests only", Contact: "Test private contact"}
	w = call("POST", "communities", application, a1, c1)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	s, _, _ := store.Load(context.Background())
	saved := s.Communities[len(s.Communities)-1]
	if saved.Applicant != "alice" || saved.Approved {
		t.Fatal("applicant controlled approval or ownership", saved)
	}
	rec := Record{Player: "Synthetic test", Account: "76561190000000003", Community: "Test", Reason: "Fixture reason", Evidence: "https://youtu.be/dQw4w9WgXcQ", Tags: []string{"aimbot"}}
	if w = call("POST", "reports", rec, a1, c1); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = call("POST", "reports", rec, a1, c1); w.Code != 409 {
		t.Fatal("duplicate open report accepted", w.Code, w.Body.String())
	}
	appeal := map[string]string{"recordId": "theirs", "account": "76561190000000001", "content": "Additional fixture appeal details"}
	if w = call("POST", "appeals", appeal, a1, c1); w.Code != 409 {
		t.Fatal("duplicate open appeal accepted", w.Code, w.Body.String())
	}
}

func TestRequestSecurityBoundary(t *testing.T) {
	t.Setenv("PUBLIC_ORIGIN", "http://127.0.0.1:8089")
	store := &fileStore{path: filepath.Join(t.TempDir(), "state.json")}
	app := newApp(store)
	r := httptest.NewRequest("GET", "http://foreign.example/", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("foreign hostname reached local service", w.Code)
	}
	r = httptest.NewRequest("POST", "http://127.0.0.1:8089/api/v1/login", strings.NewReader(`{}`))
	r.Header.Set("Origin", "http://127.0.0.1:8089")
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	w = httptest.NewRecorder()
	app.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-site write accepted", w.Code)
	}
	r = httptest.NewRequest("GET", "http://127.0.0.1:8089/api/v1/health", nil)
	w = httptest.NewRecorder()
	app.ServeHTTP(w, r)
	if w.Code != 200 || w.Header().Get("X-Frame-Options") != "DENY" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "object-src 'none'") || w.Header().Get("Permissions-Policy") == "" {
		t.Fatal("headers or origin boundary failed", w.Code, w.Header())
	}
	app.rates["api:"+ipOf(r)] = rate{Count: 240, Expires: time.Now().Add(30 * time.Second)}
	w = httptest.NewRecorder()
	app.ServeHTTP(w, r)
	retry, _ := strconv.Atoi(w.Header().Get("Retry-After"))
	if w.Code != 429 || retry < 1 || retry > 31 {
		t.Fatal("request throttle failed", w.Code, retry)
	}
}
