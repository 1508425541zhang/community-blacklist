package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

//go:embed web/*
var assets embed.FS

type Event struct {
	Date   string `json:"date"`
	Actor  string `json:"actor"`
	Action string `json:"action"`
}
type Record struct {
	Game      string   `json:"game"`
	Reporter  string   `json:"reporter"`
	UpdatedAt string   `json:"updatedAt"`
	ExpiresAt string   `json:"expiresAt"`
	ID        string   `json:"id"`
	Player    string   `json:"player"`
	Account   string   `json:"account"`
	Community string   `json:"community"`
	Reason    string   `json:"reason"`
	Tags      []string `json:"tags"`
	Status    string   `json:"status"`
	Date      string   `json:"date"`
	Evidence  string   `json:"evidence"`
	Public    bool     `json:"public"`
	History   []Event  `json:"history"`
}
type User struct {
	SteamID   string `json:"steamId,omitempty"`
	Name      string `json:"name"`
	Hash      string `json:"hash,omitempty"`
	Role      string `json:"role"`
	Community string `json:"community"`
	Disabled  bool   `json:"disabled"`
}
type Appeal struct {
	ID       string `json:"id"`
	RecordID string `json:"recordId"`
	Account  string `json:"account"`
	Content  string `json:"content"`
	Status   string `json:"status"`
	Response string `json:"response"`
	Date     string `json:"date"`
}
type State struct {
	Comments      []Comment      `json:"comments"`
	Votes         []Vote         `json:"votes"`
	Communities   []Community    `json:"communities"`
	Announcements []Announcement `json:"announcements"`
	Users         []User         `json:"users"`
	Records       []Record       `json:"records"`
	Appeals       []Appeal       `json:"appeals"`
	Audit         []Event        `json:"audit"`
}
type Store interface {
	Load(context.Context) (State, int64, error)
	Save(context.Context, State, int64) error
}

var conflict = errors.New("concurrent update")

type document struct {
	Revision int64 `json:"revision"`
	State    State `json:"state"`
}
type fileStore struct {
	path string
	mu   sync.Mutex
}

func (f *fileStore) load() (document, error) {
	var d document
	b, e := os.ReadFile(f.path)
	if os.IsNotExist(e) {
		return d, nil
	}
	if e != nil {
		return d, e
	}
	e = json.Unmarshal(b, &d)
	return d, e
}
func (f *fileStore) Load(context.Context) (State, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, e := f.load()
	return d.State, d.Revision, e
}
func (f *fileStore) Save(_ context.Context, s State, rev int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, e := f.load()
	if e != nil {
		return e
	}
	if d.Revision != rev {
		return conflict
	}
	b, e := json.Marshal(document{rev + 1, s})
	if e != nil {
		return e
	}
	if e = os.WriteFile(f.path+".tmp", b, 0600); e != nil {
		return e
	}
	return os.Rename(f.path+".tmp", f.path)
}
func token() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}

// PBKDF2-HMAC-SHA256, 600000 iterations, a random 32-byte salt.
func derive(p, s string) string {
	m := hmac.New(sha256.New, []byte(p))
	m.Write([]byte(s))
	m.Write([]byte{0, 0, 0, 1})
	u := m.Sum(nil)
	out := append([]byte(nil), u...)
	for i := 1; i < 600000; i++ {
		m.Reset()
		m.Write(u)
		u = m.Sum(nil)
		for j := range out {
			out[j] ^= u[j]
		}
	}
	return hex.EncodeToString(out)
}
func hash(p string) string { s := token(); return s + ":" + derive(p, s) }
func verify(p, h string) bool {
	s := strings.Split(h, ":")
	return len(s) == 2 && subtle.ConstantTimeCompare([]byte(derive(p, s[0])), []byte(s[1])) == 1
}

type session struct {
	User, CSRF string
	Expires    time.Time
}
type rate struct {
	Count   int
	Expires time.Time
}
type App struct {
	store       Store
	mu          sync.Mutex
	sessions    map[string]session
	rates       map[string]rate
	steamFlows  map[string]time.Time
	steamNonces map[string]time.Time
	steamVerify steamVerifier
	secure      bool
}

func newApp(s Store) *App {
	return &App{store: s, steamFlows: map[string]time.Time{}, steamNonces: map[string]time.Time{}, steamVerify: verifySteam, sessions: map[string]session{}, rates: map[string]rate{}, secure: os.Getenv("COOKIE_SECURE") == "true"}
}
func (a *App) limited(key string, max int) bool {
	return a.limitedWindow(key, max, 15*time.Minute)
}
func reply(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, code int, msg string) {
	reply(w, code, map[string]string{"error": msg})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		fail(w, 400, "请求格式错误或内容过长")
		return false
	}
	return true
}
func now() string       { return time.Now().UTC().Format(time.RFC3339) }
func clean(u User) User { u.Hash = ""; return u }
func allowed(u User, r Record) bool {
	return u.Role == "admin" || (u.Role == "reviewer" && u.Community == r.Community)
}
func recordValid(v Record) bool {
	st := map[string]bool{"pending": true, "discussion": true, "active": true, "appealing": true, "revoked": true, "needvotes": true, "selfproof": true, "moss": true, "invalid": true, "boosting": true}
	return expiryOK(v.ExpiresAt) && len(v.Game) <= 60 && strings.TrimSpace(v.Player) != "" && len(v.Player) <= 200 && strings.TrimSpace(v.Account) != "" && len(v.Account) <= 200 && strings.TrimSpace(v.Community) != "" && len(v.Community) <= 200 && strings.TrimSpace(v.Reason) != "" && len(v.Reason) <= 2000 && strings.TrimSpace(v.Evidence) != "" && len(v.Evidence) <= 10000 && st[v.Status]
}
func (a *App) identity(r *http.Request, s State) (User, session, bool) {
	c, e := r.Cookie("alliance_session")
	if e != nil {
		return User{}, session{}, false
	}
	a.mu.Lock()
	v, ok := a.sessions[c.Value]
	if ok && time.Now().After(v.Expires) {
		delete(a.sessions, c.Value)
		ok = false
	}
	a.mu.Unlock()
	if ok {
		for _, u := range s.Users {
			if u.Name == v.User && !u.Disabled && (u.Role == "admin" || steamPattern.MatchString(u.SteamID)) {
				return u, v, true
			}
		}
	}
	return User{}, session{}, false
}
func (a *App) save(w http.ResponseWriter, r *http.Request, s State, rev int64) bool {
	if e := a.store.Save(r.Context(), s, rev); e != nil {
		if errors.Is(e, conflict) {
			fail(w, 409, "其他管理员已更新记录，请刷新重试")
		} else {
			fail(w, 503, "数据库保存失败")
		}
		return false
	}
	return true
}
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; object-src 'none'; frame-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	origin, hostOK := requestOrigin(r, a.secure)
	if !hostOK {
		fail(w, 403, "网站地址不匹配，请从配置的本地地址访问")
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		a.static(w, r)
		return
	}
	if r.Method != "GET" && r.Method != "POST" && r.Method != "PATCH" {
		fail(w, 405, "不支持的方法")
		return
	}
	if r.Method != "GET" {
		if r.Header.Get("Origin") != origin || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			fail(w, 403, "请求来源不匹配")
			return
		}
	}
	if a.throttleRequest(w, r) {
		return
	}
	s, rev, e := a.store.Load(r.Context())
	if e != nil {
		fail(w, 503, "数据库暂不可用")
		return
	}
	if a.steamRoutes(w, r) {
		return
	}
	u, ss, logged := a.identity(r, s)
	if a.accountRoutes(w, r, s, u, ss, logged) {
		return
	}
	path := r.URL.Path
	if a.extended(w, r, s, rev, u, ss, logged) {
		return
	}
	if path == "/api/v1/login" && r.Method == "POST" {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if a.limited("login:"+ip, 10) {
			fail(w, 429, "尝试过多，请十五分钟后重试")
			return
		}
		var b struct {
			Name     string `json:"name"`
			Password string `json:"password"`
		}
		if !decode(w, r, &b) {
			return
		}
		if len(b.Name) < 3 || len(b.Name) > 60 || len(b.Password) < 1 || len(b.Password) > 128 {
			fail(w, 401, "账号或密码错误")
			return
		}
		for _, v := range s.Users {
			if v.Name == b.Name && v.Role == "admin" && !v.Disabled && verify(b.Password, v.Hash) {
				ss = a.beginSession(w, r, v)
				reply(w, 200, map[string]any{"user": clean(v), "csrf": ss.CSRF})
				return
			}
		}
		fail(w, 401, "账号或密码错误")
		return
	}
	if path == "/api/v1/me" && r.Method == "GET" {
		if !logged {
			fail(w, 401, "请登录")
			return
		}
		reply(w, 200, map[string]any{"user": clean(u), "csrf": ss.CSRF})
		return
	}
	if path == "/api/v1/records" && r.Method == "GET" {
		out := []Record{}
		for _, v := range s.Records {
			q := strings.ToLower(r.URL.Query().Get("q"))
			st := r.URL.Query().Get("status")
			if v.Public && (st == "" || v.Status == st) && strings.Contains(strings.ToLower(v.Player+v.Account+v.ID+v.Community), q) {
				out = append(out, v)
			}
		}
		reply(w, 200, map[string]any{"items": out, "total": len(out), "demo": false})
		return
	}
	if path == "/api/v1/appeals" && r.Method == "POST" {
		if !secured(w, r, ss, logged) {
			return
		}
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if a.limited("appeal:"+ip, 5) {
			fail(w, 429, "提交过于频繁")
			return
		}
		var b struct {
			RecordID string `json:"recordId"`
			Account  string `json:"account"`
			Content  string `json:"content"`
		}
		if !decode(w, r, &b) {
			return
		}
		if u.Role != "admin" && b.Account != u.SteamID {
			fail(w, 403, "只能为已验证的本人Steam账号申诉")
			return
		}
		if e := normalizeVideoFields(&b.Content); e != nil {
			fail(w, 400, e.Error())
			return
		}
		if len(b.Content) < 10 || len(b.Content) > 10000 {
			fail(w, 400, "申诉说明需为 10 至 10000 字节")
			return
		}
		found := false
		for _, v := range s.Records {
			if v.Public && v.ID == b.RecordID && v.Account == b.Account {
				found = true
			}
		}
		if !found {
			fail(w, 400, "记录和账号标识不匹配")
			return
		}
		for _, existing := range s.Appeals {
			if existing.RecordID == b.RecordID && existing.Account == b.Account && existing.Status == "open" {
				fail(w, 409, "该案件已有待处理申诉，请在个人中心查看处理进度")
				return
			}
		}
		v := Appeal{ID: "AP-" + token()[:12], RecordID: b.RecordID, Account: b.Account, Content: b.Content, Status: "open", Date: now()}
		s.Appeals = append(s.Appeals, v)
		s.Audit = append(s.Audit, Event{now(), u.Name, "收到申诉 " + v.ID})
		if !a.save(w, r, s, rev) {
			return
		}
		reply(w, 201, map[string]string{"id": v.ID})
		return
	}
	if !logged {
		fail(w, 401, "请登录")
		return
	}
	if r.Method != "GET" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(ss.CSRF)) != 1 {
		fail(w, 403, "会话校验失败")
		return
	}
	if path == "/api/v1/logout" && r.Method == "POST" {
		c, _ := r.Cookie("alliance_session")
		a.mu.Lock()
		delete(a.sessions, c.Value)
		a.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "alliance_session", Path: siteBase() + "/", MaxAge: -1, HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteStrictMode})
		reply(w, 200, map[string]bool{"ok": true})
		return
	}
	if path == "/api/v1/admin/records" && r.Method == "GET" {
		out := []Record{}
		for _, v := range s.Records {
			if allowed(u, v) {
				out = append(out, v)
			}
		}
		reply(w, 200, map[string]any{"items": out})
		return
	}
	if (path == "/api/v1/admin/records" && r.Method == "POST") || (strings.HasPrefix(path, "/api/v1/admin/records/") && r.Method == "PATCH") {
		var v Record
		if !decode(w, r, &v) {
			return
		}
		originalTags := v.Tags
		var tagErr error
		v.Tags, tagErr = normalizeReportTags(v.Tags)
		if tagErr != nil {
			fail(w, 400, tagErr.Error())
			return
		}
		if e := normalizeVideoFields(&v.Player, &v.Account, &v.Community, &v.Reason, &v.Evidence); e != nil {
			fail(w, 400, e.Error())
			return
		}
		if !recordValid(v) {
			fail(w, 400, "请完整填写玩家、账号、社区、原因、证据和有效状态")
			return
		}
		if !allowed(u, v) {
			fail(w, 403, "只能管理所属社区")
			return
		}
		if r.Method == "POST" {
			v.UpdatedAt = now()
			v.ID = "CB-" + token()[:12]
			v.Date = now()
			v.History = []Event{{now(), u.Name, "创建案件：" + v.Status}}
			s.Records = append(s.Records, v)
		} else {
			id := strings.TrimPrefix(path, "/api/v1/admin/records/")
			found := false
			for i, old := range s.Records {
				if old.ID == id {
					if !allowed(u, old) {
						fail(w, 403, "无权修改")
						return
					}
					if originalTags == nil {
						v.Tags = old.Tags
					}
					v.UpdatedAt = now()
					v.ID = id
					v.Date = old.Date
					v.Reporter = old.Reporter
					v.History = append(old.History, Event{now(), u.Name, fmt.Sprintf("状态 %s → %s，公开 %t", old.Status, v.Status, v.Public)})
					s.Records[i] = v
					found = true
					break
				}
			}
			if !found {
				fail(w, 404, "记录不存在")
				return
			}
		}
		s.Audit = append(s.Audit, v.History[len(v.History)-1])
		if !a.save(w, r, s, rev) {
			return
		}
		reply(w, 200, v)
		return
	}
	if path == "/api/v1/admin/users" && u.Role == "admin" {
		if r.Method == "GET" {
			out := []User{}
			for _, v := range s.Users {
				out = append(out, clean(v))
			}
			reply(w, 200, map[string]any{"items": out})
			return
		}
		if r.Method == "POST" {
			var b struct {
				Name      string `json:"name"`
				Password  string `json:"password"`
				Role      string `json:"role"`
				Community string `json:"community"`
			}
			if !decode(w, r, &b) {
				return
			}
			if len(b.Name) < 3 || len(b.Name) > 60 || len(b.Password) < 12 || len(b.Password) > 128 || b.Role != "admin" || (b.Role == "reviewer" && strings.TrimSpace(b.Community) == "") {
				fail(w, 400, "只能创建管理员密码账号；审核员须先Steam登录再分配角色")
				return
			}
			for _, v := range s.Users {
				if v.Name == b.Name {
					fail(w, 409, "账号已存在")
					return
				}
			}
			v := User{Name: b.Name, Hash: hash(b.Password), Role: b.Role, Community: b.Community}
			s.Users = append(s.Users, v)
			s.Audit = append(s.Audit, Event{now(), u.Name, "创建账号 " + v.Name})
			if !a.save(w, r, s, rev) {
				return
			}
			reply(w, 201, clean(v))
			return
		}
	}
	if path == "/api/v1/admin/appeals" && r.Method == "GET" {
		out := []Appeal{}
		for _, v := range s.Appeals {
			for _, rec := range s.Records {
				if v.RecordID == rec.ID && allowed(u, rec) {
					out = append(out, v)
				}
			}
		}
		reply(w, 200, map[string]any{"items": out})
		return
	}
	if strings.HasPrefix(path, "/api/v1/admin/appeals/") && r.Method == "PATCH" {
		id := strings.TrimPrefix(path, "/api/v1/admin/appeals/")
		var b struct {
			Response string `json:"response"`
		}
		if !decode(w, r, &b) {
			return
		}
		if e := normalizeVideoFields(&b.Response); e != nil {
			fail(w, 400, e.Error())
			return
		}
		if strings.TrimSpace(b.Response) == "" || len(b.Response) > 10000 {
			fail(w, 400, "请填写处理结论")
			return
		}
		for i, v := range s.Appeals {
			if v.ID == id {
				for _, rec := range s.Records {
					if rec.ID == v.RecordID && allowed(u, rec) {
						s.Appeals[i].Status = "closed"
						s.Appeals[i].Response = b.Response
						s.Audit = append(s.Audit, Event{now(), u.Name, "处理申诉 " + id})
						if !a.save(w, r, s, rev) {
							return
						}
						reply(w, 200, s.Appeals[i])
						return
					}
				}
			}
		}
		fail(w, 404, "申诉不存在或无权处理")
		return
	}
	if path == "/api/v1/admin/audit" && u.Role == "admin" && r.Method == "GET" {
		reply(w, 200, map[string]any{"items": s.Audit})
		return
	}
	fail(w, 404, "接口不存在或无权访问")
}
func (a *App) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		fail(w, 405, "仅支持GET")
		return
	}
	p := r.URL.Path
	if p == "/" || p == "/players" || p == "/report" || p == "/statistics" || p == "/announcements" || p == "/communities" || p == "/api-docs" || p == "/account" || p == "/privacy" || p == "/terms" || p == "/reporting-rules" || p == "/community-guidelines" || p == "/api-terms" || p == "/cookies" || p == "/design" || strings.HasPrefix(p, "/player/") || strings.HasPrefix(p, "/search/") {
		p = "/index.html"
	}
	if p == "/admin" {
		p = "/admin.html"
	}
	types := map[string]string{"/index.html": "text/html", "/admin.html": "text/html", "/style.css": "text/css", "/home-motion.css": "text/css", "/player-profile.css": "text/css", "/player-profile.js": "text/javascript", "/site-docs.css": "text/css", "/site-docs.js": "text/javascript", "/home-motion.js": "text/javascript", "/app.js": "text/javascript", "/admin.js": "text/javascript", "/video-policy.js": "text/javascript", "/report-tags.js": "text/javascript", "/account.js": "text/javascript", "/brand-mark.svg": "image/svg+xml", "/brand-wordmark.svg": "image/svg+xml"}
	mime, ok := types[p]
	if !ok {
		http.NotFound(w, r)
		return
	}
	b, e := assets.ReadFile("web" + p)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime+"; charset=utf-8")
	if mime == "text/html" {
		html := string(b)
		html = strings.Replace(html, "<head>", "<head><meta name=\"site-base\" content=\""+siteBase()+"\">", 1)
		html = strings.ReplaceAll(html, `href="/`, `href="`+siteBase()+`/`)
		html = strings.ReplaceAll(html, `src="/`, `src="`+siteBase()+`/`)
		b = []byte(html)
	}
	w.Write(b)
}
func main() {
	var store Store
	var e error
	mode := os.Getenv("STORAGE_MODE")
	if mode == "" || mode == "sqlite" {
		path := os.Getenv("DATABASE_FILE")
		if path == "" {
			path = "alliance.db"
		}
		store, e = newSQLiteStore(path)
		if e != nil {
			log.Fatal(e)
		}
	} else if mode == "d1" {
		if os.Getenv("COOKIE_SECURE") != "true" {
			log.Fatal("D1模式需要 COOKIE_SECURE=true 和 HTTPS反向代理")
		}
		store, e = newD1Store()
		if e != nil {
			log.Fatal(e)
		}
	} else if mode == "local" {
		p := os.Getenv("DATA_FILE")
		if p == "" {
			p = "data.json"
		}
		store = &fileStore{path: p}
	} else {
		log.Fatal("STORAGE_MODE只能为sqlite、local或d1")
	}
	s, rev, e := store.Load(context.Background())
	if e != nil {
		log.Fatal("数据库读取失败：", e)
	}
	if len(s.Users) == 0 && os.Getenv("IMPORT_JSON") != "" {
		legacy := &fileStore{path: os.Getenv("IMPORT_JSON")}
		imported, _, err := legacy.Load(context.Background())
		if err != nil || len(imported.Users) == 0 {
			log.Fatal("旧数据导入失败或没有账号，停止初始化")
		}
		if err = store.Save(context.Background(), imported, rev); err != nil {
			log.Fatal(err)
		}
		s, rev, e = store.Load(context.Background())
		if e != nil {
			log.Fatal(e)
		}
	}
	if len(s.Users) == 0 {
		s.Communities = append(s.Communities, Community{ID: "CO-founder", Name: "战狗 WARDOGS", Description: "联合黑名单发起社区，按证据审核，保留申诉与复核记录。", Approved: true, Date: now()})
		p := os.Getenv("ADMIN_PASSWORD")
		if p == "" {
			p = token()
			secretPath := ".sqlite-initial-password"
			if mode == "local" {
				secretPath = ".initial-password"
			}
			secretFile, err := os.OpenFile(secretPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				log.Fatal("首次密码文件已存在或不可写，请显式设置 ADMIN_PASSWORD；不会覆盖原凭证")
			}
			_, e = secretFile.Write([]byte(p))
			secretFile.Close()
			if e != nil {
				log.Fatal(e)
			}
			log.Print("首次密码已保存到独立初始化密码文件，请登录后修改")
		}
		if len(p) < 12 || len(p) > 128 {
			log.Fatal("ADMIN_PASSWORD需要12至128字符")
		}
		name := os.Getenv("ADMIN_USERNAME")
		if name == "" {
			name = "admin"
		}
		s.Users = append(s.Users, User{Name: name, Hash: hash(p), Role: "admin"})
		if e = store.Save(context.Background(), s, rev); e != nil {
			log.Fatal(e)
		}
	}
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8088"
	}
	log.Printf("社区联合黑名单 http://%s/admin", addr)
	server := &http.Server{Addr: addr, Handler: proxyHandler(newApp(store)), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	log.Fatal(server.ListenAndServe())
}
