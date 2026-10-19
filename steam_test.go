package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Isolated authorization fixtures, never available to the production router.
func fixtureSession(app *App, user string) (*http.Cookie, string) {
	key := token()
	csrf := token()
	app.sessions[key] = session{user, csrf, time.Now().Add(time.Hour)}
	return &http.Cookie{Name: "alliance_session", Value: key}, csrf
}

type steamTestTransport func(*http.Request) (*http.Response, error)

func (f steamTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSteamProviderVerificationWire(t *testing.T) {
	values := url.Values{"openid.mode": {"id_res"}, "openid.sig": {"signature"}, "openid.identity": {"identity"}, "openid.op_endpoint": {"https://attacker.invalid"}, "state": {"not-sent-to-provider"}}
	response := "ns:" + openidNS + "\nis_valid:true\n"
	client := &http.Client{Transport: steamTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != steamEndpoint || r.Method != "POST" {
			t.Fatal("provider destination was not fixed HTTPS", r.URL)
		}
		r.ParseForm()
		if r.Form.Get("openid.mode") != "check_authentication" || r.Form.Get("openid.sig") != "signature" || r.Form.Get("state") != "" {
			t.Fatal("wrong verification parameters", r.Form)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: make(http.Header)}, nil
	})}
	valid, err := verifySteamUsing(context.Background(), values, client)
	if err != nil || !valid {
		t.Fatal("valid provider response rejected", err)
	}
	for _, invalid := range []string{"is_valid:false\n", "is_valid:true\nis_valid:true\n", "ns:ignored\n", "is_valid:trueish\n"} {
		response = invalid
		valid, err = verifySteamUsing(context.Background(), values, client)
		if valid {
			t.Fatal("invalid provider response accepted", invalid, err)
		}
	}
}

func steamResponse(t *testing.T, app *App, id string) (url.Values, *http.Cookie) {
	t.Helper()
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/api/v1/auth/steam", nil))
	if w.Code != 302 {
		t.Fatal("Steam start", w.Code, w.Body.String())
	}
	target, e := url.Parse(w.Header().Get("Location"))
	if e != nil || target.Scheme != "https" || target.Host != "steamcommunity.com" {
		t.Fatal("wrong provider", target)
	}
	ret := target.Query().Get("openid.return_to")
	callback, _ := url.Parse(ret)
	q := url.Values{"state": {callback.Query().Get("state")}, "openid.ns": {openidNS}, "openid.mode": {"id_res"}, "openid.op_endpoint": {steamEndpoint}, "openid.claimed_id": {"https://steamcommunity.com/openid/id/" + id}, "openid.identity": {"https://steamcommunity.com/openid/id/" + id}, "openid.return_to": {ret}, "openid.response_nonce": {time.Now().UTC().Format("2006-01-02T15:04:05Z") + token()}, "openid.assoc_handle": {"test-handle"}, "openid.signed": {"op_endpoint,claimed_id,identity,return_to,response_nonce,assoc_handle"}, "openid.sig": {"synthetic-signature"}}
	return q, w.Result().Cookies()[0]
}

func TestSteamAuthenticationBoundary(t *testing.T) {
	t.Setenv("PUBLIC_ORIGIN", "http://localhost")
	store := &fileStore{path: filepath.Join(t.TempDir(), "data.json")}
	app := newApp(store)
	verifyCalls := 0
	app.steamVerify = func(ctx context.Context, q url.Values) (bool, error) { verifyCalls++; return true, nil }
	callback := func(q url.Values, c *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "http://localhost/api/v1/auth/steam/callback?"+q.Encode(), nil)
		if c != nil {
			r.AddCookie(c)
		}
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		return w
	}
	q, c := steamResponse(t, app, "76561190000000001")
	if w := callback(q, nil); w.Code != 403 || verifyCalls != 0 {
		t.Fatal("missing state cookie accepted")
	}
	q.Set("openid.op_endpoint", "http://127.0.0.1/private")
	if w := callback(q, c); w.Code != 403 || verifyCalls != 0 {
		t.Fatal("provider injection accepted")
	}
	q, c = steamResponse(t, app, "76561190000000001")
	q.Set("openid.signed", "identity")
	if w := callback(q, c); w.Code != 403 || verifyCalls != 0 {
		t.Fatal("unsigned identity accepted")
	}
	q, c = steamResponse(t, app, "76561190000000001")
	q.Add("openid.claimed_id", "https://steamcommunity.com/openid/id/76561190000000002")
	if w := callback(q, c); w.Code != 403 {
		t.Fatal("duplicate claimed id accepted")
	}
	q, c = steamResponse(t, app, "76561190000000001")
	app.steamVerify = func(context.Context, url.Values) (bool, error) { return false, nil }
	if w := callback(q, c); w.Code != 403 {
		t.Fatal("invalid Valve verification accepted")
	}
	app.steamVerify = func(ctx context.Context, q url.Values) (bool, error) { verifyCalls++; return true, nil }
	q, c = steamResponse(t, app, "76561190000000001")
	w := callback(q, c)
	if w.Code != 303 || w.Header().Get("Location") != "/account" {
		t.Fatal("verified callback failed", w.Code, w.Body.String())
	}
	state, _, e := store.Load(context.Background())
	if e != nil || len(state.Users) != 1 || state.Users[0].Role != "member" || state.Users[0].SteamID != "76561190000000001" || state.Users[0].Hash != "" {
		t.Fatal("Steam account wrongly created", state, e)
	}
	if replay := callback(q, c); replay.Code != 403 {
		t.Fatal("state replay accepted")
	}
	oldNonce := q.Get("openid.response_nonce")
	q, c = steamResponse(t, app, "76561190000000001")
	q.Set("openid.response_nonce", oldNonce)
	if replay := callback(q, c); replay.Code != 403 {
		t.Fatal("nonce replay accepted")
	}
	q, c = steamResponse(t, app, "76561190000000001")
	if w = callback(q, c); w.Code != 303 {
		t.Fatal("repeat login failed")
	}
	state, rev, _ := store.Load(context.Background())
	if len(state.Users) != 1 {
		t.Fatal("duplicate account on repeat login")
	}
	state.Users[0].Disabled = true
	store.Save(context.Background(), state, rev)
	q, c = steamResponse(t, app, "76561190000000001")
	if w = callback(q, c); w.Code != 403 {
		t.Fatal("disabled account signed in")
	}
}

func TestSteamSignedReturnAndExpiry(t *testing.T) {
	t.Setenv("PUBLIC_ORIGIN", "http://localhost")
	app := newApp(&fileStore{path: filepath.Join(t.TempDir(), "data.json")})
	q, _ := steamResponse(t, app, "76561190000000001")
	ret := q.Get("openid.return_to")
	q.Set("openid.return_to", "https://other.example/callback")
	if _, e := steamClaim(q, ret); e == nil {
		t.Fatal("return_to mismatch")
	}
	q.Set("openid.return_to", ret)
	q.Set("openid.response_nonce", time.Now().Add(-time.Hour).UTC().Format("2006-01-02T15:04:05Z")+"old")
	if _, e := steamClaim(q, ret); e == nil {
		t.Fatal("expired nonce")
	}
	t.Setenv("PUBLIC_ORIGIN", "http://public.example")
	if _, e := steamOrigin(); e == nil {
		t.Fatal("insecure external callback")
	}
	t.Setenv("PUBLIC_ORIGIN", "https://site.example/path")
	if _, e := steamOrigin(); e == nil {
		t.Fatal("origin with path")
	}
	t.Setenv("PUBLIC_ORIGIN", "http://127.0.0.1:8089")
	if origin, e := steamOrigin(); e != nil || !strings.HasPrefix(origin, "http://127.") {
		t.Fatal("local origin rejected")
	}
}
