package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVideoAllowlistContract(t *testing.T) {
	var cases []struct {
		URL       string `json:"url"`
		Canonical string `json:"canonical"`
	}
	raw, e := os.ReadFile("video-policy-cases.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &cases); e != nil {
		t.Fatal(e)
	}
	for _, c := range cases {
		t.Run(c.URL, func(t *testing.T) {
			value, e := canonicalVideo(c.URL)
			if c.Canonical == "" {
				if e == nil {
					t.Fatal("unsupported link accepted", value)
				}
			} else if e != nil || value != c.Canonical {
				t.Fatal("canonical mismatch", value, c.Canonical, e)
			}
		})
	}
	plain := "SteamID: 76561190000000001。时间 02:15，联系人 test@example.com。"
	if value, e := normalizeVideoText(plain); e != nil || value != plain {
		t.Fatal("ordinary text or contact altered", value, e)
	}
	text := "视频：https://youtu.be/dQw4w9WgXcQ?si=tracking 。补充 https://www.bilibili.com/video/BV1La411N7Nd/。"
	if value, e := normalizeVideoText(text); e != nil || strings.Contains(value, "tracking") || !strings.Contains(value, "youtube.com/watch") {
		t.Fatal("multiple links not normalized", value, e)
	}
	for _, bad := range []string{"看这里 https://evil.com/virus.exe", "视频 https://youtu.be/dQw4w9WgXcQ 然后 https://evil.com", "www.evil.xyz/file", "javascript:alert(1)", "链接 ftp://evil.com/file"} {
		if _, e := normalizeVideoText(bad); e == nil {
			t.Fatal("unsupported link in text accepted", bad)
		}
	}
}

func TestLinkRestrictionsCannotBypassUI(t *testing.T) {
	store := &fileStore{path: filepath.Join(t.TempDir(), "data.json")}
	seed := State{Users: []User{{Name: "member", SteamID: "76561190000000001", Role: "member"}, {Name: "admin", Role: "admin"}}, Communities: []Community{{Name: "Test", Approved: true}}, Records: []Record{{ID: "case", Player: "Test player", Account: "76561190000000001", Community: "Test", Status: "pending", Evidence: "https://www.bilibili.com/video/BV1La411N7Nd/", Public: true}}}
	store.Save(context.Background(), seed, 0)
	app := newApp(store)
	call := func(path, method, body, user string) *httptest.ResponseRecorder {
		cookie, csrf := fixtureSession(app, user)
		r := httptest.NewRequest(method, "http://localhost/api/v1/"+path, strings.NewReader(body))
		r.Header.Set("Origin", "http://localhost")
		r.Header.Set("X-CSRF-Token", csrf)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		return w
	}
	tests := []struct{ path, method, body, user string }{
		{"reports", "POST", `{"player":"Test","account":"76561190000000001","community":"Test","tags":["cheating"],"reason":"review","evidence":"https://evil.com/file"}`, "member"},
		{"comments", "POST", `{"recordId":"case","content":"https://evil.com/file"}`, "member"},
		{"appeals", "POST", `{"recordId":"case","account":"76561190000000001","content":"https://evil.com/file"}`, "member"},
		{"communities", "POST", `{"name":"Test application","website":"https://evil.com","description":"Long description for testing","contact":"contact"}`, "member"},
		{"admin/records/case", "PATCH", `{"player":"Test","account":"76561190000000001","community":"Test","tags":["cheating"],"reason":"review","evidence":"https://evil.com/file","status":"active","public":true}`, "admin"},
		{"admin/votes", "POST", `{"recordId":"case","verdict":"active","reason":"https://evil.com/file"}`, "admin"},
		{"admin/announcements", "POST", `{"title":"Test announcement","content":"https://evil.com/file","public":true}`, "admin"},
	}
	for _, c := range tests {
		w := call(c.path, c.method, c.body, c.user)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "Bilibili") {
			t.Fatalf("%s bypassed restriction: %d %s", c.path, w.Code, w.Body.String())
		}
	}
	w := call("reports", "POST", `{"player":"Test","account":"76561190000000001","community":"Test","tags":["cheating"],"reason":"review","evidence":"https://youtu.be/dQw4w9WgXcQ?si=tracking"}`, "member")
	if w.Code != 201 || strings.Contains(w.Body.String(), "si=tracking") || !strings.Contains(w.Body.String(), "www.youtube.com/watch") {
		t.Fatal("valid video failed", w.Code, w.Body.String())
	}
	state, _, _ := store.Load(context.Background())
	if len(state.Records) != 2 || len(state.Comments) != 0 || len(state.Appeals) != 0 || len(state.Announcements) != 0 {
		t.Fatal("rejected submission persisted")
	}
}
