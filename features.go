package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Comment struct {
	ID       string `json:"id"`
	RecordID string `json:"recordId"`
	Actor    string `json:"actor"`
	Content  string `json:"content"`
	Date     string `json:"date"`
	Hidden   bool   `json:"hidden"`
}
type Vote struct {
	RecordID string `json:"recordId"`
	Actor    string `json:"actor"`
	Verdict  string `json:"verdict"`
	Reason   string `json:"reason"`
	Date     string `json:"date"`
}
type Community struct {
	Applicant   string `json:"applicant,omitempty"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Website     string `json:"website"`
	Description string `json:"description"`
	Contact     string `json:"contact,omitempty"`
	Approved    bool   `json:"approved"`
	Date        string `json:"date"`
}
type Announcement struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Public  bool   `json:"public"`
}

var steamPattern = regexp.MustCompile(`^7656119[0-9]{10}$`)
var usernamePattern = regexp.MustCompile(`^[\p{L}\p{N}_-]{3,40}$`)

func bounded(s string, min, max int) bool { return len(strings.TrimSpace(s)) >= min && len(s) <= max }
func validURL(s string) bool {
	if s == "" {
		return true
	}
	u, e := url.Parse(s)
	return e == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.User == nil
}
func expiryOK(s string) bool {
	if s == "" {
		return true
	}
	_, e := time.Parse(time.RFC3339, s)
	return e == nil
}
func expiresLive(s string) bool {
	if s == "" {
		return true
	}
	t, e := time.Parse(time.RFC3339, s)
	return e == nil && time.Now().Before(t)
}
func ipOf(r *http.Request) string { ip, _, _ := net.SplitHostPort(r.RemoteAddr); return ip }
func secured(w http.ResponseWriter, r *http.Request, ss session, logged bool) bool {
	if !logged {
		fail(w, 401, "请先登录")
		return false
	}
	if r.Method != "GET" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(ss.CSRF)) != 1 {
		fail(w, 403, "会话校验失败")
		return false
	}
	return true
}
func (a *App) extended(w http.ResponseWriter, r *http.Request, s State, rev int64, u User, ss session, logged bool) bool {
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/v1/profiles/") {
		a.publicProfile(w, r, s)
		return true
	}
	if path == "/api/v1/report-categories" && r.Method == "GET" {
		reply(w, 200, map[string]any{"items": reportCategories})
		return true
	}
	if path == "/api/v1/health" && r.Method == "GET" {
		reply(w, 200, map[string]any{"ok": true, "database": "ready"})
		return true
	}
	if path == "/api/v1/register" && r.Method == "POST" {
		fail(w, 403, "普通用户只能使用Steam登录，不支持密码注册")
		return true
	}

	if path == "/api/v1/statistics" && r.Method == "GET" {
		counts := map[string]int{}
		games := map[string]int{}
		total := 0
		events := []Event{}
		for _, rec := range s.Records {
			if rec.Public {
				total++
				counts[rec.Status]++
				game := rec.Game
				if game == "" {
					game = "WARDOGS"
				}
				games[game]++
				for _, v := range rec.History {
					events = append(events, Event{v.Date, v.Actor, rec.Player + " · " + v.Action})
				}
			}
		}
		sort.SliceStable(events, func(i, j int) bool { return events[i].Date > events[j].Date })
		if len(events) > 12 {
			events = events[:12]
		}
		communities := 0
		for _, c := range s.Communities {
			if c.Approved {
				communities++
			}
		}
		reply(w, 200, map[string]any{"total": total, "statuses": counts, "games": games, "users": len(s.Users), "communities": communities, "events": events})
		return true
	}
	if path == "/api/v1/players" && r.Method == "GET" {
		q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
		status := r.URL.Query().Get("status")
		game := r.URL.Query().Get("game")
		community := r.URL.Query().Get("community")
		tag := r.URL.Query().Get("tag")
		if tag != "" {
			if _, err := normalizeReportTags([]string{tag}); err != nil {
				fail(w, 400, err.Error())
				return true
			}
		}
		from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
		out := []Record{}
		for _, v := range s.Records {
			if v.Public && (status == "" || v.Status == status) && (game == "" || v.Game == game) && (community == "" || v.Community == community) && (from == "" || v.Date >= from) && (to == "" || v.Date[:min(10, len(v.Date))] <= to) && strings.Contains(strings.ToLower(v.Player+" "+v.Account+" "+v.ID+" "+v.Community), q) {
				if tag != "" {
					match := false
					for _, value := range v.Tags {
						if value == tag {
							match = true
						}
					}
					if !match {
						continue
					}
				}
				out = append(out, v)
			}
		}
		order := r.URL.Query().Get("sort")
		sort.SliceStable(out, func(i, j int) bool {
			if order == "oldest" {
				return out[i].Date < out[j].Date
			}
			if order == "created" {
				return out[i].Date > out[j].Date
			}
			return out[i].UpdatedAt+out[i].Date > out[j].UpdatedAt+out[j].Date
		})
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 || page > 1000000 {
			page = 1
		}
		size, _ := strconv.Atoi(r.URL.Query().Get("size"))
		if size < 1 || size > 50 {
			size = 20
		}
		start := (page - 1) * size
		if start < 0 || start > len(out) {
			start = len(out)
		}
		end := min(start+size, len(out))
		reply(w, 200, map[string]any{"items": out[start:end], "total": len(out), "page": page, "size": size})
		return true
	}
	if path == "/api/v1/bans" && r.Method == "GET" {
		if a.limited("feed:"+ipOf(r), 100) {
			fail(w, 429, "下载频率过高")
			return true
		}
		if configured := osExportToken(); configured != "" && !sameToken(r.Header.Get("Authorization"), "Bearer "+configured) {
			fail(w, 401, "需要名单 API 密钥")
			return true
		}
		bans := []map[string]any{}
		selected := map[string]Record{}
		for _, v := range s.Records {
			if v.Public && v.Status == "active" && steamPattern.MatchString(v.Account) && expiresLive(v.ExpiresAt) {
				old, exists := selected[v.Account]
				if !exists || (old.ExpiresAt != "" && (v.ExpiresAt == "" || v.ExpiresAt > old.ExpiresAt)) {
					selected[v.Account] = v
				}
			}
		}
		accounts := []string{}
		for account := range selected {
			accounts = append(accounts, account)
		}
		sort.Strings(accounts)
		for _, account := range accounts {
			v := selected[account]
			var expiry any
			if v.ExpiresAt != "" {
				expiry = v.ExpiresAt
			}
			bans = append(bans, map[string]any{"steamId": v.Account, "reason": v.Reason, "expiresAt": expiry, "caseId": v.ID})
		}
		reply(w, 200, map[string]any{"version": 1, "generatedAt": now(), "cacheHours": 24, "bans": bans})
		return true
	}
	if strings.HasPrefix(path, "/api/v1/player/") {
		id := strings.TrimPrefix(path, "/api/v1/player/")
		var rec *Record
		for i := range s.Records {
			if s.Records[i].ID == id && (s.Records[i].Public || (logged && (s.Records[i].Reporter == u.Name || allowed(u, s.Records[i])))) {
				rec = &s.Records[i]
				break
			}
		}
		if rec == nil {
			fail(w, 404, "公开案件不存在")
			return true
		}
		if r.Method == "GET" {
			comments := []Comment{}
			votes := []Vote{}
			for _, v := range s.Comments {
				if v.RecordID == id && !v.Hidden {
					comments = append(comments, v)
				}
			}
			for _, v := range s.Votes {
				if v.RecordID == id {
					votes = append(votes, v)
				}
			}
			reply(w, 200, map[string]any{"record": rec, "comments": comments, "votes": votes})
			return true
		}
	}
	if path == "/api/v1/reports" && r.Method == "POST" {
		if !secured(w, r, ss, logged) {
			return true
		}
		if a.limited("report:"+u.Name, 10) {
			fail(w, 429, "举报提交过于频繁")
			return true
		}
		var v Record
		if !decode(w, r, &v) {
			return true
		}
		v.Status = "pending"
		v.Public = true
		v.Reporter = u.Name
		v.ExpiresAt = ""
		var tagErr error
		v.Tags, tagErr = normalizeReportTags(v.Tags)
		if tagErr != nil {
			fail(w, 400, tagErr.Error())
			return true
		}
		if len(v.Tags) == 0 {
			fail(w, 400, "请至少选择一个违规标签，再补充详细原因")
			return true
		}
		if v.Game == "" {
			v.Game = "WARDOGS"
		}
		if e := normalizeVideoFields(&v.Player, &v.Account, &v.Community, &v.Reason, &v.Evidence); e != nil {
			fail(w, 400, e.Error())
			return true
		}
		if !recordValid(v) || !steamPattern.MatchString(v.Account) || len(v.Game) > 60 {
			fail(w, 400, "请填写真实SteamID64、昵称、社区、原因和证据")
			return true
		}
		known := false
		for _, c := range s.Communities {
			if c.Approved && c.Name == v.Community {
				known = true
			}
		}
		if !known {
			fail(w, 400, "请选择已加入联合的社区")
			return true
		}
		for _, old := range s.Records {
			if old.Reporter == u.Name && old.Account == v.Account && old.Community == v.Community && old.Public && reportOpen(old.Status) {
				fail(w, 409, "你已提交该玩家的待处理举报（"+old.ID+"），请在原案件讨论中补充证据")
				return true
			}
		}
		v.ID = "CB-" + token()[:12]
		v.Date = now()
		v.UpdatedAt = v.Date
		v.History = []Event{{v.Date, u.Name, "提交举报，等待审核"}}
		s.Records = append(s.Records, v)
		s.Audit = append(s.Audit, v.History[0])
		if a.save(w, r, s, rev) {
			reply(w, 201, v)
		}
		return true
	}
	if path == "/api/v1/comments" && r.Method == "POST" {
		if !secured(w, r, ss, logged) {
			return true
		}
		if a.limited("comment:"+u.Name, 20) {
			fail(w, 429, "讨论过于频繁")
			return true
		}
		var b struct {
			RecordID string `json:"recordId"`
			Content  string `json:"content"`
		}
		if !decode(w, r, &b) {
			return true
		}
		found := false
		for _, rec := range s.Records {
			if rec.ID == b.RecordID && rec.Public {
				found = true
			}
		}
		if e := normalizeVideoFields(&b.Content); e != nil {
			fail(w, 400, e.Error())
			return true
		}
		if !found || !bounded(b.Content, 2, 5000) {
			fail(w, 400, "案件或讨论内容无效")
			return true
		}
		c := Comment{ID: "CM-" + token()[:12], RecordID: b.RecordID, Actor: u.Name, Content: b.Content, Date: now()}
		s.Comments = append(s.Comments, c)
		if a.save(w, r, s, rev) {
			reply(w, 201, c)
		}
		return true
	}
	if path == "/api/v1/communities" {
		if r.Method == "GET" {
			type PublicCommunity struct {
				Community
				CaseCount int `json:"caseCount"`
			}
			out := []PublicCommunity{}
			counts := map[string]int{}
			for _, record := range s.Records {
				if record.Public {
					counts[record.Community]++
				}
			}
			for _, v := range s.Communities {
				if v.Approved {
					v.Contact = ""
					v.Applicant = ""
					out = append(out, PublicCommunity{v, counts[v.Name]})
				}
			}
			reply(w, 200, map[string]any{"items": out})
			return true
		}
		if r.Method == "POST" {
			if !secured(w, r, ss, logged) {
				return true
			}
			if a.limited("community:"+u.Name, 3) {
				fail(w, 429, "社区申请过于频繁")
				return true
			}
			var v Community
			if !decode(w, r, &v) {
				return true
			}
			if e := normalizeVideoFields(&v.Name, &v.Website, &v.Description, &v.Contact); e != nil {
				fail(w, 400, e.Error())
				return true
			}
			if !bounded(v.Name, 2, 100) || !bounded(v.Description, 10, 2000) || !bounded(v.Contact, 3, 200) || !validURL(v.Website) {
				fail(w, 400, "请完整填写社区资料和联系方式")
				return true
			}
			for _, c := range s.Communities {
				if c.Name == v.Name {
					fail(w, 409, "该社区已有申请或已加入")
					return true
				}
			}
			v.ID = "CO-" + token()[:12]
			v.Applicant = u.Name
			v.Approved = false
			v.Date = now()
			s.Communities = append(s.Communities, v)
			s.Audit = append(s.Audit, Event{now(), u.Name, "申请加入社区 " + v.Name})
			if a.save(w, r, s, rev) {
				reply(w, 201, map[string]any{"id": v.ID})
			}
			return true
		}
	}
	if path == "/api/v1/announcements" && r.Method == "GET" {
		out := []Announcement{}
		for _, v := range s.Announcements {
			if v.Public {
				out = append(out, v)
			}
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].Date > out[j].Date })
		reply(w, 200, map[string]any{"items": out})
		return true
	}
	if path == "/api/v1/my-reports" && r.Method == "GET" {
		if !secured(w, r, ss, logged) {
			return true
		}
		out := []Record{}
		for _, v := range s.Records {
			if v.Reporter == u.Name {
				out = append(out, v)
			}
		}
		reply(w, 200, map[string]any{"items": out})
		return true
	}
	if path == "/api/v1/password" && r.Method == "POST" {
		if u.Role != "admin" {
			fail(w, 403, "只有管理员可使用本站密码；其他账号使用Steam登录")
			return true
		}
		if !secured(w, r, ss, logged) {
			return true
		}
		if a.limited("password:"+u.Name, 5) {
			fail(w, 429, "尝试过于频繁")
			return true
		}
		var b struct {
			Old      string `json:"old"`
			Password string `json:"password"`
		}
		if !decode(w, r, &b) {
			return true
		}
		if !verify(b.Old, u.Hash) || len(b.Password) < 12 || len(b.Password) > 128 {
			fail(w, 400, "当前密码不正确或新密码长度无效")
			return true
		}
		for i := range s.Users {
			if s.Users[i].Name == u.Name {
				s.Users[i].Hash = hash(b.Password)
			}
		}
		s.Audit = append(s.Audit, Event{now(), u.Name, "修改本人密码"})
		if a.save(w, r, s, rev) {
			a.mu.Lock()
			for id, v := range a.sessions {
				if v.User == u.Name {
					delete(a.sessions, id)
				}
			}
			a.mu.Unlock()
			reply(w, 200, map[string]bool{"ok": true})
		}
		return true
	}
	if !strings.HasPrefix(path, "/api/v1/admin/") {
		return false
	}
	// Existing routes share this role boundary; ordinary registered players cannot administer.
	if u.Role != "admin" && u.Role != "reviewer" {
		if !logged {
			fail(w, 401, "请登录")
		} else {
			fail(w, 403, "需要社区审核权限")
		}
		return true
	}
	if path == "/api/v1/admin/votes" && r.Method == "POST" {
		if !secured(w, r, ss, logged) {
			return true
		}
		var v Vote
		if !decode(w, r, &v) {
			return true
		}
		if e := normalizeVideoFields(&v.Reason); e != nil {
			fail(w, 400, e.Error())
			return true
		}
		if (v.Verdict != "active" && v.Verdict != "invalid" && v.Verdict != "selfproof") || !bounded(v.Reason, 5, 2000) {
			fail(w, 400, "请填写有效审核建议和依据")
			return true
		}
		found := false
		for i := range s.Records {
			if s.Records[i].ID == v.RecordID && allowed(u, s.Records[i]) {
				s.Records[i].UpdatedAt = now()
				s.Records[i].History = append(s.Records[i].History, Event{now(), u.Name, "审核建议：" + v.Verdict + "；" + v.Reason})
				found = true
			}
		}
		if !found {
			fail(w, 404, "案件不存在或无权审核")
			return true
		}
		v.Actor = u.Name
		v.Date = now()
		replaced := false
		for i, old := range s.Votes {
			if old.RecordID == v.RecordID && old.Actor == u.Name {
				s.Votes[i] = v
				replaced = true
			}
		}
		if !replaced {
			s.Votes = append(s.Votes, v)
		}
		s.Audit = append(s.Audit, Event{now(), u.Name, "提交案件审核建议 " + v.RecordID})
		if a.save(w, r, s, rev) {
			reply(w, 200, v)
		}
		return true
	}
	if strings.HasPrefix(path, "/api/v1/admin/comments/") && r.Method == "PATCH" {
		if !secured(w, r, ss, logged) {
			return true
		}
		id := strings.TrimPrefix(path, "/api/v1/admin/comments/")
		var b struct {
			Hidden bool `json:"hidden"`
		}
		if !decode(w, r, &b) {
			return true
		}
		for i, v := range s.Comments {
			if v.ID == id {
				for _, rec := range s.Records {
					if rec.ID == v.RecordID && allowed(u, rec) {
						s.Comments[i].Hidden = b.Hidden
						s.Audit = append(s.Audit, Event{now(), u.Name, "调整讨论可见性 " + id})
						if a.save(w, r, s, rev) {
							reply(w, 200, s.Comments[i])
						}
						return true
					}
				}
			}
		}
		fail(w, 404, "讨论不存在或无权处理")
		return true
	}
	if u.Role != "admin" {
		return false
	}
	if path == "/api/v1/admin/communities" && r.Method == "GET" {
		reply(w, 200, map[string]any{"items": s.Communities})
		return true
	}
	if strings.HasPrefix(path, "/api/v1/admin/communities/") && r.Method == "PATCH" {
		if !secured(w, r, ss, logged) {
			return true
		}
		id := strings.TrimPrefix(path, "/api/v1/admin/communities/")
		var b struct {
			Approved bool `json:"approved"`
		}
		if !decode(w, r, &b) {
			return true
		}
		for i, v := range s.Communities {
			if v.ID == id {
				s.Communities[i].Approved = b.Approved
				s.Audit = append(s.Audit, Event{now(), u.Name, "审核社区加入申请 " + v.Name})
				if a.save(w, r, s, rev) {
					reply(w, 200, s.Communities[i])
				}
				return true
			}
		}
		fail(w, 404, "社区不存在")
		return true
	}
	if path == "/api/v1/admin/announcements" {
		if r.Method == "GET" {
			reply(w, 200, map[string]any{"items": s.Announcements})
			return true
		}
		if r.Method == "POST" {
			if !secured(w, r, ss, logged) {
				return true
			}
			var v Announcement
			if !decode(w, r, &v) {
				return true
			}
			if e := normalizeVideoFields(&v.Title, &v.Content); e != nil {
				fail(w, 400, e.Error())
				return true
			}
			if !bounded(v.Title, 2, 150) || !bounded(v.Content, 10, 20000) {
				fail(w, 400, "公告标题或正文长度无效")
				return true
			}
			v.ID = "AN-" + token()[:12]
			v.Date = now()
			v.Author = u.Name
			s.Announcements = append(s.Announcements, v)
			s.Audit = append(s.Audit, Event{now(), u.Name, "创建公告 " + v.ID})
			if a.save(w, r, s, rev) {
				reply(w, 201, v)
			}
			return true
		}
	}
	if strings.HasPrefix(path, "/api/v1/admin/announcements/") && r.Method == "PATCH" {
		if !secured(w, r, ss, logged) {
			return true
		}
		id := strings.TrimPrefix(path, "/api/v1/admin/announcements/")
		var b struct {
			Public bool `json:"public"`
		}
		if !decode(w, r, &b) {
			return true
		}
		for i, v := range s.Announcements {
			if v.ID == id {
				s.Announcements[i].Public = b.Public
				s.Audit = append(s.Audit, Event{now(), u.Name, "调整公告可见性 " + id})
				if a.save(w, r, s, rev) {
					reply(w, 200, s.Announcements[i])
				}
				return true
			}
		}
		fail(w, 404, "公告不存在")
		return true
	}
	if strings.HasPrefix(path, "/api/v1/admin/users/") && r.Method == "PATCH" {
		if !secured(w, r, ss, logged) {
			return true
		}
		name := strings.TrimPrefix(path, "/api/v1/admin/users/")
		var b struct {
			Role      string `json:"role"`
			Community string `json:"community"`
			Disabled  bool   `json:"disabled"`
		}
		if !decode(w, r, &b) {
			return true
		}
		if (b.Role != "admin" && b.Role != "reviewer" && b.Role != "member") || (b.Role == "reviewer" && !bounded(b.Community, 1, 200)) {
			fail(w, 400, "角色或所属社区无效")
			return true
		}
		activeAdmins := 0
		for _, v := range s.Users {
			if v.Role == "admin" && !v.Disabled {
				activeAdmins++
			}
		}
		for i, v := range s.Users {
			if v.Name == name {
				if b.Role != "admin" && !steamPattern.MatchString(v.SteamID) {
					fail(w, 400, "非管理员角色必须有经过验证的Steam身份")
					return true
				}
				if name == u.Name && (b.Disabled || b.Role != "admin") {
					fail(w, 400, "不能禁用或降级当前管理员")
					return true
				}
				if v.Role == "admin" && !v.Disabled && activeAdmins <= 1 && (b.Disabled || b.Role != "admin") {
					fail(w, 400, "必须保留至少一位联合管理员")
					return true
				}
				s.Users[i].Role = b.Role
				s.Users[i].Community = b.Community
				s.Users[i].Disabled = b.Disabled
				s.Audit = append(s.Audit, Event{now(), u.Name, "修改账号权限 " + name})
				if a.save(w, r, s, rev) {
					reply(w, 200, clean(s.Users[i]))
				}
				return true
			}
		}
		fail(w, 404, "账号不存在")
		return true
	}
	return false
}
func osExportToken() string { return os.Getenv("EXPORT_TOKEN") }
func sameToken(a, b string) bool {
	x, y := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(x[:], y[:]) == 1
}
func fingerprint(raw string) string { v := sha256.Sum256([]byte(raw)); return hex.EncodeToString(v[:]) }

func siteBase() string {
	base := strings.TrimRight(os.Getenv("BASE_PATH"), "/")
	if base != "" && !regexp.MustCompile(`^(/[a-zA-Z0-9_-]+)+$`).MatchString(base) {
		return ""
	}
	return base
}

// Only the explicitly configured loopback reverse proxy may supply the client IP.
func proxyHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, _, _ := net.SplitHostPort(r.RemoteAddr)
		if os.Getenv("TRUST_PROXY_LOOPBACK") == "true" && net.ParseIP(peer) != nil && net.ParseIP(peer).IsLoopback() {
			if ip := net.ParseIP(r.Header.Get("X-Real-IP")); ip != nil {
				r = r.Clone(r.Context())
				r.RemoteAddr = net.JoinHostPort(ip.String(), "0")
			}
		}
		next.ServeHTTP(w, r)
	})
}
