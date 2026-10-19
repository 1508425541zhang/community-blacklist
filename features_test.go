package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSQLiteCommunityWorkflow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alliance.db")
	store, err := newSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.db.Close()
	seed := State{Users: []User{{Name: "admin", Hash: hash("admin-test-password"), Role: "admin"}, {Name: "testmember", Hash: hash("member-test-password"), Role: "member", SteamID: "76561190000000001"}, {Name: "outside", Hash: hash("review-test-password"), Role: "reviewer", Community: "Other", SteamID: "76561190000000002"}}, Communities: []Community{{ID: "A", Name: "Test community", Approved: true, Contact: "private contact"}}}
	if err = store.Save(context.Background(), seed, 0); err != nil {
		t.Fatal(err)
	}
	if err = store.Save(context.Background(), seed, 0); err == nil {
		t.Fatal("stale revision accepted")
	}
	app := newApp(store)
	var cookie *http.Cookie
	csrf := ""
	call := func(method, path string, body any, auth bool) (int, map[string]any) {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, "http://localhost"+path, bytes.NewReader(raw))
		r.Header.Set("Origin", "http://localhost")
		if auth && cookie != nil {
			r.AddCookie(cookie)
			r.Header.Set("X-CSRF-Token", csrf)
		}
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		var data map[string]any
		json.Unmarshal(w.Body.Bytes(), &data)
		if path == "/api/v1/login" && w.Code == 200 {
			cookie = w.Result().Cookies()[0]
			csrf = data["csrf"].(string)
		}
		return w.Code, data
	}
	expect := func(want int, method, path string, body any, auth bool) map[string]any {
		t.Helper()
		code, data := call(method, path, body, auth)
		if code != want {
			t.Fatalf("%s %s: %d want %d: %v", method, path, code, want, data)
		}
		return data
	}
	expect(403, "POST", "/api/v1/register", map[string]string{"name": "testmember", "password": "member-test-password"}, false)
	expect(401, "POST", "/api/v1/login", map[string]string{"name": "testmember", "password": "member-test-password"}, false)
	cookie, csrf = fixtureSession(app, "testmember")
	expect(403, "GET", "/api/v1/admin/records", nil, true)
	rec := Record{Player: "Synthetic test player", Account: "76561190000000001", Community: "Test community", Tags: []string{"wallhack"}, Reason: "Synthetic fixture only", Evidence: "https://www.bilibili.com/video/BV1La411N7Nd/", Status: "active", Public: true, Reporter: "admin"}
	created := expect(201, "POST", "/api/v1/reports", rec, true)
	id := created["id"].(string)
	if created["status"] != "pending" || created["reporter"] != "testmember" {
		t.Fatal("reporter controlled adjudication", created)
	}
	data := expect(200, "GET", "/api/v1/bans", nil, false)
	if len(data["bans"].([]any)) != 0 {
		t.Fatal("pending report exported")
	}
	comments := expect(201, "POST", "/api/v1/comments", map[string]string{"recordId": id, "content": "<script>alert(1)</script> test"}, true)
	commentID := comments["id"].(string)
	expect(401, "POST", "/api/v1/login", map[string]string{"name": "outside", "password": "review-test-password"}, false)
	cookie, csrf = fixtureSession(app, "outside")
	expect(403, "PATCH", "/api/v1/admin/records/"+id, rec, true)
	expect(404, "POST", "/api/v1/admin/votes", map[string]string{"recordId": id, "verdict": "active", "reason": "Evidence reviewed"}, true)
	expect(200, "POST", "/api/v1/login", map[string]string{"name": "admin", "password": "admin-test-password"}, false)
	expect(200, "POST", "/api/v1/admin/votes", map[string]string{"recordId": id, "verdict": "active", "reason": "Evidence reviewed"}, true)
	rec.ExpiresAt = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	expect(200, "PATCH", "/api/v1/admin/records/"+id, rec, true)
	rec.ExpiresAt = ""
	expect(200, "POST", "/api/v1/admin/records", rec, true)
	data = expect(200, "GET", "/api/v1/bans", nil, false)
	bans := data["bans"].([]any)
	if len(bans) != 1 || bans[0].(map[string]any)["expiresAt"] != nil {
		t.Fatal("duplicate ban must retain permanent punishment", data)
	}
	t.Setenv("EXPORT_TOKEN", "test-feed-only")
	expect(401, "GET", "/api/v1/bans", nil, false)
	t.Setenv("EXPORT_TOKEN", "")
	expect(201, "POST", "/api/v1/appeals", map[string]string{"recordId": id, "account": rec.Account, "content": "Synthetic appeal with sufficient details."}, true)
	expect(200, "PATCH", "/api/v1/admin/comments/"+commentID, map[string]bool{"hidden": true}, true)
	data = expect(200, "GET", "/api/v1/player/"+id, nil, false)
	if len(data["comments"].([]any)) != 0 {
		t.Fatal("hidden comment exposed")
	}
	data = expect(200, "GET", "/api/v1/communities", nil, false)
	if _, ok := data["items"].([]any)[0].(map[string]any)["contact"]; ok {
		t.Fatal("private contact leaked")
	}
	expect(400, "PATCH", "/api/v1/admin/users/admin", map[string]any{"role": "member", "disabled": true}, true)
	expect(200, "PATCH", "/api/v1/admin/users/testmember", map[string]any{"role": "member", "disabled": true}, true)
	expect(401, "POST", "/api/v1/login", map[string]string{"name": "testmember", "password": "member-test-password"}, false)
	expect(200, "POST", "/api/v1/password", map[string]string{"old": "admin-test-password", "password": "new-admin-test-password"}, true)
	expect(401, "GET", "/api/v1/me", nil, true)
	restored, e := newSQLiteStore(path)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.db.Close()
	state, _, e := restored.Load(context.Background())
	if e != nil || len(state.Records) != 2 || len(state.Appeals) != 1 || len(state.Votes) != 1 || len(state.Audit) < 5 {
		t.Fatal("persisted workflow incomplete", e, state)
	}
}

func TestSubpathAndProxyBoundary(t *testing.T) {
	t.Setenv("BASE_PATH", "/community")
	store := &fileStore{path: filepath.Join(t.TempDir(), "data.json")}
	app := newApp(store)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/players", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `href="/community/players"`) || !strings.Contains(w.Body.String(), `name="site-base" content="/community"`) {
		t.Fatal("subpath assets/navigation missing")
	}
	t.Setenv("BASE_PATH", `/bad"><script>`)
	if siteBase() != "" {
		t.Fatal("unvalidated base path")
	}
	t.Setenv("TRUST_PROXY_LOOPBACK", "true")
	captured := ""
	handler := proxyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { captured = r.RemoteAddr }))
	request := httptest.NewRequest("GET", "http://localhost/", nil)
	request.RemoteAddr = "203.0.113.9:1234"
	request.Header.Set("X-Real-IP", "198.51.100.2")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if captured != request.RemoteAddr {
		t.Fatal("untrusted peer spoofed client IP")
	}
	request.RemoteAddr = "127.0.0.1:1234"
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if captured != "198.51.100.2:0" {
		t.Fatal("trusted proxy client IP ignored")
	}
}
