package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const steamEndpoint = "https://steamcommunity.com/openid/login"
const openidNS = "http://specs.openid.net/auth/2.0"

type steamVerifier func(context.Context, url.Values) (bool, error)

func steamOrigin() (string, error) {
	origin := strings.TrimRight(os.Getenv("PUBLIC_ORIGIN"), "/")
	u, e := url.Parse(origin)
	if e != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("请配置 PUBLIC_ORIGIN 为网站原点地址")
	}
	ip := net.ParseIP(u.Hostname())
	local := u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return "", errors.New("Steam回调需要HTTPS或本机HTTP地址")
	}
	return origin, nil
}

// Always verify against Valve's fixed HTTPS provider; callback input cannot change it.
func verifySteam(ctx context.Context, values url.Values) (bool, error) {
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return verifySteamUsing(ctx, values, client)
}

func verifySteamUsing(ctx context.Context, values url.Values, client *http.Client) (bool, error) {
	form := url.Values{}
	for key, v := range values {
		if strings.HasPrefix(key, "openid.") {
			form[key] = v
		}
	}
	form.Set("openid.mode", "check_authentication")
	req, e := http.NewRequestWithContext(ctx, "POST", steamEndpoint, strings.NewReader(form.Encode()))
	if e != nil {
		return false, e
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, e := client.Do(req)
	if e != nil {
		return false, e
	}
	defer res.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(res.Body, 65537))
	if e != nil || len(raw) > 65536 || res.StatusCode != 200 {
		return false, errors.New("Steam验证服务响应异常")
	}
	found := false
	for _, line := range strings.Split(string(raw), "\n") {
		pair := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(pair) == 2 && pair[0] == "is_valid" {
			if found || pair[1] != "true" {
				return false, nil
			}
			found = true
		}
	}
	return found, nil
}

func (a *App) beginSession(w http.ResponseWriter, r *http.Request, u User) session {
	id := token()
	ss := session{u.Name, token(), time.Now().Add(8 * time.Hour)}
	a.mu.Lock()
	for k, v := range a.sessions {
		if time.Now().After(v.Expires) {
			delete(a.sessions, k)
		}
	}
	if old, e := r.Cookie("alliance_session"); e == nil {
		delete(a.sessions, old.Value)
	}
	a.sessions[id] = ss
	a.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "alliance_session", Value: id, Path: siteBase() + "/", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode, MaxAge: 28800})
	return ss
}

func steamClaim(q url.Values, returnTo string) (string, error) {
	for key, values := range q {
		if (strings.HasPrefix(key, "openid.") || key == "state") && len(values) != 1 {
			return "", errors.New("重复认证参数")
		}
	}
	if q.Get("openid.ns") != openidNS || q.Get("openid.mode") != "id_res" || q.Get("openid.op_endpoint") != steamEndpoint || q.Get("openid.return_to") != returnTo {
		return "", errors.New("Steam认证来源或回调地址不匹配")
	}
	claimed := q.Get("openid.claimed_id")
	id := ""
	for _, prefix := range []string{"https://steamcommunity.com/openid/id/", "http://steamcommunity.com/openid/id/"} {
		if strings.HasPrefix(claimed, prefix) {
			id = strings.TrimPrefix(claimed, prefix)
		}
	}
	if !steamPattern.MatchString(id) || q.Get("openid.identity") != claimed {
		return "", errors.New("无效的Steam身份")
	}
	signed := map[string]bool{}
	for _, key := range strings.Split(q.Get("openid.signed"), ",") {
		signed[key] = true
	}
	for _, key := range []string{"op_endpoint", "claimed_id", "identity", "return_to", "response_nonce", "assoc_handle"} {
		if !signed[key] || q.Get("openid."+key) == "" {
			return "", errors.New("Steam认证缺少签名字段")
		}
	}
	if q.Get("openid.sig") == "" {
		return "", errors.New("Steam签名缺失")
	}
	nonce := q.Get("openid.response_nonce")
	if len(nonce) < 21 || len(nonce) > 255 {
		return "", errors.New("认证随机数无效")
	}
	stamp, e := time.Parse(time.RFC3339, nonce[:20])
	if e != nil || time.Since(stamp) > 10*time.Minute || time.Until(stamp) > 2*time.Minute {
		return "", errors.New("Steam认证已过期")
	}
	return id, nil
}

func (a *App) steamRoutes(w http.ResponseWriter, r *http.Request) bool {
	start := r.URL.Path == "/api/v1/auth/steam"
	callback := r.URL.Path == "/api/v1/auth/steam/callback"
	if !start && !callback {
		return false
	}
	if r.Method != "GET" {
		fail(w, 405, "Steam登录只支持GET")
		return true
	}
	origin, e := steamOrigin()
	if e != nil {
		fail(w, 503, e.Error())
		return true
	}
	if a.limited("steam:"+ipOf(r), 20) {
		fail(w, 429, "Steam登录过于频繁")
		return true
	}
	if start {
		state := token()
		now := time.Now()
		a.mu.Lock()
		for k, v := range a.steamFlows {
			if now.After(v) {
				delete(a.steamFlows, k)
			}
		}
		if len(a.steamFlows) >= 10000 {
			a.mu.Unlock()
			fail(w, 503, "登录繁忙，请稍后再试")
			return true
		}
		a.steamFlows[state] = now.Add(10 * time.Minute)
		a.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "steam_login_state", Value: state, Path: siteBase() + "/api/v1/auth/steam", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode, MaxAge: 600})
		q := url.Values{"openid.ns": {openidNS}, "openid.mode": {"checkid_setup"}, "openid.claimed_id": {openidNS + "/identifier_select"}, "openid.identity": {openidNS + "/identifier_select"}, "openid.realm": {origin + siteBase() + "/"}, "openid.return_to": {origin + siteBase() + "/api/v1/auth/steam/callback?state=" + state}}
		http.Redirect(w, r, steamEndpoint+"?"+q.Encode(), http.StatusFound)
		return true
	}
	if len(r.URL.RawQuery) > 16384 {
		fail(w, 400, "认证参数过长")
		return true
	}
	q := r.URL.Query()
	state := q.Get("state")
	cookie, e := r.Cookie("steam_login_state")
	if e != nil || len(state) != 64 || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(state)) != 1 {
		fail(w, 403, "登录会话不匹配，请重新使用Steam登录")
		return true
	}
	a.mu.Lock()
	expiry, exists := a.steamFlows[state]
	delete(a.steamFlows, state)
	a.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "steam_login_state", Path: siteBase() + "/api/v1/auth/steam", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	if !exists || time.Now().After(expiry) {
		fail(w, 403, "登录会话过期或已使用")
		return true
	}
	id, e := steamClaim(q, origin+siteBase()+"/api/v1/auth/steam/callback?state="+state)
	if e != nil {
		fail(w, 403, e.Error())
		return true
	}
	valid, e := a.steamVerify(r.Context(), q)
	if e != nil {
		fail(w, 502, "无法完成Steam官方验证，请稍后重试")
		return true
	}
	if !valid {
		fail(w, 403, "Steam官方验证未通过")
		return true
	}
	nonce := q.Get("openid.response_nonce")
	a.mu.Lock()
	for k, v := range a.steamNonces {
		if time.Now().After(v) {
			delete(a.steamNonces, k)
		}
	}
	_, replayed := a.steamNonces[nonce]
	if !replayed {
		a.steamNonces[nonce] = time.Now().Add(12 * time.Minute)
	}
	a.mu.Unlock()
	if replayed {
		fail(w, 403, "认证响应已使用")
		return true
	}
	s, rev, e := a.store.Load(r.Context())
	if e != nil {
		fail(w, 503, "数据库暂不可用")
		return true
	}
	var user User
	found := false
	for _, v := range s.Users {
		if v.SteamID == id {
			user = v
			found = true
			break
		}
	}
	if !found {
		// A proven Steam ID never merges with a legacy password account by display name.
		name := "steam_" + id
		for _, v := range s.Users {
			if v.Name == name {
				fail(w, 409, "Steam账号标识冲突，请联系管理员")
				return true
			}
		}
		user = User{Name: name, Role: "member", SteamID: id}
		s.Users = append(s.Users, user)
		s.Audit = append(s.Audit, Event{now(), name, "Steam验证后首次创建账号"})
		if !a.save(w, r, s, rev) {
			return true
		}
	}
	if user.Disabled {
		fail(w, 403, "账号已停用")
		return true
	}
	a.beginSession(w, r, user)
	http.Redirect(w, r, siteBase()+"/account", http.StatusSeeOther)
	return true
}
