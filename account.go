package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
	"time"
)

func reportOpen(status string) bool {
	return status == "pending" || status == "discussion" || status == "needvotes" || status == "selfproof" || status == "moss" || status == "appealing"
}

func sessionLabel(key string) string {
	digest := sha256.Sum256([]byte("session-management:" + key))
	return hex.EncodeToString(digest[:])[:24]
}

func (a *App) accountRoutes(w http.ResponseWriter, r *http.Request, s State, u User, ss session, logged bool) bool {
	path := r.URL.Path
	if path != "/api/v1/account-summary" && !strings.HasPrefix(path, "/api/v1/sessions") {
		return false
	}
	if !secured(w, r, ss, logged) {
		return true
	}
	if path == "/api/v1/account-summary" && r.Method == "GET" {
		reports := []Record{}
		appeals := []Appeal{}
		applications := []Community{}
		pending, completed := 0, 0
		for _, rec := range s.Records {
			if rec.Reporter == u.Name {
				reports = append(reports, rec)
				if reportOpen(rec.Status) {
					pending++
				} else {
					completed++
				}
			}
		}
		for _, appeal := range s.Appeals {
			if u.SteamID != "" && appeal.Account == u.SteamID {
				appeals = append(appeals, appeal)
			}
		}
		for _, application := range s.Communities {
			if application.Applicant == u.Name {
				applications = append(applications, application)
			}
		}
		sort.SliceStable(reports, func(i, j int) bool { return reports[i].UpdatedAt > reports[j].UpdatedAt })
		sort.SliceStable(appeals, func(i, j int) bool { return appeals[i].Date > appeals[j].Date })
		reply(w, 200, map[string]any{"reports": reports, "appeals": appeals, "applications": applications, "pending": pending, "completed": completed})
		return true
	}
	cookie, _ := r.Cookie("alliance_session")
	if path == "/api/v1/sessions" && r.Method == "GET" {
		items := []map[string]any{}
		a.mu.Lock()
		for key, current := range a.sessions {
			if time.Now().After(current.Expires) {
				delete(a.sessions, key)
				continue
			}
			if current.User == u.Name {
				items = append(items, map[string]any{"id": sessionLabel(key), "current": key == cookie.Value, "expiresAt": current.Expires.UTC().Format(time.RFC3339)})
			}
		}
		a.mu.Unlock()
		sort.Slice(items, func(i, j int) bool { return items[i]["id"].(string) < items[j]["id"].(string) })
		reply(w, 200, map[string]any{"items": items})
		return true
	}
	if path == "/api/v1/sessions/revoke-others" && r.Method == "POST" {
		a.mu.Lock()
		count := 0
		for key, current := range a.sessions {
			if current.User == u.Name && key != cookie.Value {
				delete(a.sessions, key)
				count++
			}
		}
		a.mu.Unlock()
		reply(w, 200, map[string]any{"revoked": count})
		return true
	}
	if path == "/api/v1/sessions/revoke" && r.Method == "POST" {
		var body struct {
			ID string `json:"id"`
		}
		if !decode(w, r, &body) {
			return true
		}
		if len(body.ID) != 24 {
			fail(w, 400, "会话标识无效")
			return true
		}
		a.mu.Lock()
		found := false
		for key, current := range a.sessions {
			if current.User == u.Name && sessionLabel(key) == body.ID {
				delete(a.sessions, key)
				found = true
				break
			}
		}
		a.mu.Unlock()
		if !found {
			fail(w, 404, "会话不存在或不属于本人")
			return true
		}
		reply(w, 200, map[string]bool{"ok": true})
		return true
	}
	fail(w, 405, "不支持的个人中心操作")
	return true
}
