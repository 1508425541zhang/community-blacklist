package main

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type profileItem struct {
	Record
	Group string `json:"group"`
}

func profileGroup(rec Record, at time.Time) string {
	if rec.Status == "active" {
		if rec.ExpiresAt == "" {
			return "active"
		}
		expires, err := time.Parse(time.RFC3339, rec.ExpiresAt)
		if err != nil {
			return "resolved"
		}
		if !at.Before(expires) {
			return "expired"
		}
		return "active"
	}
	if reportOpen(rec.Status) {
		return "pending"
	}
	return "resolved"
}

func profileUpdated(rec Record) (time.Time, string) {
	for _, raw := range []string{rec.UpdatedAt, rec.Date} {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			return parsed, raw
		}
	}
	return time.Time{}, ""
}

func (a *App) publicProfile(w http.ResponseWriter, r *http.Request, s State) {
	if r.Method != "GET" {
		fail(w, 405, "账号公开档案仅支持GET")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/profiles/")
	if !steamPattern.MatchString(id) {
		fail(w, 400, "请使用17位SteamID64查询")
		return
	}
	query := r.URL.Query()
	for _, key := range []string{"group", "page", "size"} {
		if len(query[key]) > 1 {
			fail(w, 400, "查询参数不能重复")
			return
		}
	}
	group := query.Get("group")
	if group == "" {
		group = "all"
	}
	if group != "all" && group != "active" && group != "expired" && group != "pending" && group != "resolved" {
		fail(w, 400, "记录分组无效")
		return
	}
	page, size := 1, 20
	for _, parameter := range []struct {
		key     string
		value   *int
		maximum int
	}{{"page", &page, 1000000}, {"size", &size, 50}} {
		if raw := query.Get(parameter.key); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > parameter.maximum {
				fail(w, 400, "页码或每页数量无效，每页最多50条")
				return
			}
			*parameter.value = value
		}
	}
	at := time.Now().UTC()
	items := []profileItem{}
	counts := map[string]int{"active": 0, "expired": 0, "pending": 0, "resolved": 0}
	communities := map[string]bool{}
	// The public account summary never includes hidden cases, even for an administrator.
	for _, rec := range s.Records {
		if rec.Public && rec.Account == id {
			kind := profileGroup(rec, at)
			counts[kind]++
			items = append(items, profileItem{rec, kind})
			if name := strings.TrimSpace(rec.Community); name != "" {
				communities[name] = true
			}
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		left, _ := profileUpdated(items[i].Record)
		right, _ := profileUpdated(items[j].Record)
		if left.Equal(right) {
			return items[i].ID > items[j].ID
		}
		return left.After(right)
	})
	names := []string{}
	for name := range communities {
		names = append(names, name)
	}
	sort.Strings(names)
	displayName, lastUpdated := "", ""
	if len(items) > 0 {
		_, lastUpdated = profileUpdated(items[0].Record)
		for _, item := range items {
			if name := strings.TrimSpace(item.Player); name != "" {
				displayName = name
				break
			}
		}
	}
	publicTotal := len(items)
	selected := []profileItem{}
	for _, item := range items {
		if group == "all" || item.Group == group {
			selected = append(selected, item)
		}
	}
	start := min((page-1)*size, len(selected))
	end := min(start+size, len(selected))
	reply(w, 200, map[string]any{
		"account": id, "displayName": displayName, "lastUpdated": lastUpdated,
		"asOf": at.Format(time.RFC3339), "publicOnly": true, "publicTotal": publicTotal,
		"counts": counts, "communities": names, "group": group,
		"items": selected[start:end], "total": len(selected), "page": page, "size": size,
	})
}
