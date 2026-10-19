package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestOperationalFlow(t *testing.T) {
	store := &fileStore{path: filepath.Join(t.TempDir(), "data.json")}
	ctx := context.Background()
	if e := store.Save(ctx, State{Users: []User{{Name: "admin", Hash: hash("test-password-long"), Role: "admin"}, {Name: "reviewer", Hash: hash("reviewer-password-long"), Role: "reviewer", Community: "B", SteamID: "76561190000000002"}}}, 0); e != nil {
		t.Fatal(e)
	}
	app := newApp(store)
	var cookie *http.Cookie
	csrf := ""
	call := func(method, path string, body any, auth bool) (int, map[string]any) {
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(method, "http://localhost"+path, bytes.NewReader(b))
		r.Header.Set("Origin", "http://localhost")
		if auth {
			r.AddCookie(cookie)
			r.Header.Set("X-CSRF-Token", csrf)
		}
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		var result map[string]any
		json.Unmarshal(w.Body.Bytes(), &result)
		if path == "/api/v1/login" && w.Code == 200 {
			cookie = w.Result().Cookies()[0]
			csrf = result["csrf"].(string)
		}
		return w.Code, result
	}
	if code, _ := call("POST", "/api/v1/admin/records", nil, false); code != 401 {
		t.Fatal(code)
	}
	if code, _ := call("POST", "/api/v1/login", map[string]string{"name": "admin", "password": "bad"}, false); code != 401 {
		t.Fatal(code)
	}
	if code, _ := call("POST", "/api/v1/login", map[string]string{"name": "admin", "password": "test-password-long"}, false); code != 200 {
		t.Fatal(code)
	}
	rec := Record{Player: "Test", Account: "steam-test", Community: "A", Reason: "Rule", Evidence: "Evidence", Status: "pending"}
	code, data := call("POST", "/api/v1/admin/records", rec, true)
	if code != 200 {
		t.Fatal(code, data)
	}
	id := data["id"].(string)
	_, data = call("GET", "/api/v1/records", nil, false)
	if data["total"] != float64(0) {
		t.Fatal("private record leaked")
	}
	rec.Public = true
	rec.Status = "active"
	if code, _ = call("PATCH", "/api/v1/admin/records/"+id, rec, true); code != 200 {
		t.Fatal(code)
	}
	_, data = call("GET", "/api/v1/records", nil, false)
	if data["total"] != float64(1) {
		t.Fatal(data)
	}
	if code, _ = call("POST", "/api/v1/appeals", map[string]string{"recordId": id, "account": "steam-test", "content": "This is a meaningful appeal."}, true); code != 201 {
		t.Fatal(code)
	}
	if code, _ = call("POST", "/api/v1/login", map[string]string{"name": "reviewer", "password": "reviewer-password-long"}, false); code != 401 {
		t.Fatal(code)
	}
	cookie, csrf = fixtureSession(app, "reviewer")
	if code, _ = call("PATCH", "/api/v1/admin/records/"+id, rec, true); code != 403 {
		t.Fatal("community boundary", code)
	}
	if code, _ = call("GET", "/api/v1/admin/users", nil, true); code != 404 {
		t.Fatal("privilege escalation", code)
	}
	before := csrf
	csrf = "invalid"
	if code, _ = call("POST", "/api/v1/logout", nil, true); code != 403 {
		t.Fatal("csrf", code)
	}
	csrf = before
	s, rev, e := store.Load(ctx)
	if e != nil || len(s.Records) != 1 || len(s.Appeals) != 1 || len(s.Audit) < 3 {
		t.Fatal(s, e)
	}
	if e = store.Save(ctx, s, rev-1); e != conflict {
		t.Fatal("missing concurrency protection")
	}
	r := httptest.NewRequest("POST", "http://localhost/api/v1/login", bytes.NewBufferString(`{}`))
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	app.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("origin bypass")
	}
}
func TestD1CAS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" {
			t.Error("auth missing")
		}
		var b struct {
			SQL    string `json:"sql"`
			Params []any  `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&b)
		if b.SQL == "SELECT payload, revision FROM alliance_state WHERE id = 1" {
			w.Write([]byte(`{"success":true,"result":[{"success":true,"results":[{"payload":"{\"users\":[]}","revision":2}]}]}`))
		} else {
			if len(b.Params) != 2 {
				t.Error("CAS revision missing")
			}
			w.Write([]byte(`{"success":true,"result":[{"success":true,"meta":{"changes":0}}]}`))
		}
	}))
	defer server.Close()
	d := &d1Store{endpoint: server.URL, token: "test", client: server.Client()}
	_, rev, e := d.Load(context.Background())
	if e != nil || rev != 2 {
		t.Fatal(rev, e)
	}
	if d.Save(context.Background(), State{}, rev) != conflict {
		t.Fatal("D1 CAS did not reject stale update")
	}
}
