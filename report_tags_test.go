package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReportTagLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alliance.db")
	store, err := newSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.db.Close()
	seed := State{Users: []User{{Name: "member", Role: "member", SteamID: "76561190000000001"}, {Name: "admin", Role: "admin"}}, Communities: []Community{{Name: "Test", Approved: true}}}
	if err = store.Save(context.Background(), seed, 0); err != nil {
		t.Fatal(err)
	}
	app := newApp(store)
	call := func(method, path string, body any, user string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, "http://localhost/api/v1/"+path, bytes.NewReader(raw))
		r.Header.Set("Origin", "http://localhost")
		if user != "" {
			cookie, csrf := fixtureSession(app, user)
			r.AddCookie(cookie)
			r.Header.Set("X-CSRF-Token", csrf)
		}
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "report-categories", nil, ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	rec := Record{Player: "Synthetic fixture", Account: "76561190000000001", Community: "Test", Reason: "Test allegation only", Evidence: "https://youtu.be/dQw4w9WgXcQ", Status: "active", Public: true}
	if w := call("POST", "reports", rec, "member"); w.Code != 400 {
		t.Fatal("missing tags accepted", w.Code)
	}
	rec.Tags = []string{"<script>"}
	if w := call("POST", "reports", rec, "member"); w.Code != 400 {
		t.Fatal("unknown tag accepted", w.Code)
	}
	rec.Tags = []string{"wallhack", "aimbot"}
	if w := call("POST", "reports", rec, "member"); w.Code != 400 {
		t.Fatal("multiple tags accepted", w.Code)
	}
	rec.Tags = []string{"wallhack"}
	w := call("POST", "reports", rec, "member")
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var created Record
	if err = json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	want := []string{"wallhack"}
	if w = call("GET", "players?tag=wallhack", nil, ""); w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(created.ID)) {
		t.Fatal("category filter omitted matching case", w.Code, w.Body.String())
	}
	if w = call("GET", "players?tag=insult", nil, ""); w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte(created.ID)) {
		t.Fatal("category filter included wrong case", w.Code, w.Body.String())
	}
	if w = call("GET", "players?tag=unknown", nil, ""); w.Code != 400 {
		t.Fatal("unknown category filter accepted", w.Code)
	}
	if created.Status != "pending" || !reflect.DeepEqual(created.Tags, want) {
		t.Fatal("tags changed adjudication or duplicates saved", created)
	}
	// Load through a separate SQLite connection, proving persistence independent of the app.
	reloaded, err := newSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.db.Close()
	state, _, err := reloaded.Load(context.Background())
	if err != nil || len(state.Records) != 1 || !reflect.DeepEqual(state.Records[0].Tags, want) {
		t.Fatal("tag persistence failed", err, state.Records)
	}
	w = call("GET", "player/"+created.ID, nil, "")
	var detail struct {
		Record Record `json:"record"`
	}
	json.Unmarshal(w.Body.Bytes(), &detail)
	if w.Code != 200 || !reflect.DeepEqual(detail.Record.Tags, want) {
		t.Fatal("public case lost tags", w.Code, w.Body.String())
	}
	created.Tags = nil // Existing API clients that omit tags preserve them on edits.
	w = call("PATCH", "admin/records/"+created.ID, created, "admin")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &rec)
	if !reflect.DeepEqual(rec.Tags, want) {
		t.Fatal("legacy edit deleted tags", rec.Tags)
	}
	created.Tags = []string{"unknown"}
	if w = call("PATCH", "admin/records/"+created.ID, created, "admin"); w.Code != 400 {
		t.Fatal("admin bypassed tag validation", w.Code)
	}
	created.Tags = []string{"boosting"}
	w = call("PATCH", "admin/records/"+created.ID, created, "admin")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &rec)
	if rec.Status != "pending" || !reflect.DeepEqual(rec.Tags, created.Tags) {
		t.Fatal("editing tags changed verdict", rec)
	}
	created.Tags = []string{}
	w = call("PATCH", "admin/records/"+created.ID, created, "admin")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &rec)
	if len(rec.Tags) != 0 {
		t.Fatal("explicit tag clearing failed", rec.Tags)
	}
}
