package main

import (
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Fixed authority prevents a foreign hostname resolving to this loopback service.
func requestOrigin(r *http.Request, secure bool) (string, bool) {
	if os.Getenv("PUBLIC_ORIGIN") != "" {
		origin, err := steamOrigin()
		if err != nil {
			return "", false
		}
		u, _ := url.Parse(origin)
		return origin, strings.EqualFold(u.Host, r.Host)
	}
	u, err := url.Parse("http://" + r.Host)
	if err != nil || u.User != nil || u.Path != "" || u.Host == "" {
		return "", false
	}
	ip := net.ParseIP(u.Hostname())
	if u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return "", false
	}
	scheme := "http"
	if secure {
		scheme = "https"
	}
	return scheme + "://" + r.Host, true
}

func (a *App) limitedWindow(key string, max int, window time.Duration) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	if len(a.rates) > 10000 {
		for k, v := range a.rates {
			if now.After(v.Expires) {
				delete(a.rates, k)
			}
		}
		if len(a.rates) > 10000 {
			return true
		}
	}
	v := a.rates[key]
	if now.After(v.Expires) {
		v = rate{Expires: now.Add(window)}
	}
	v.Count++
	a.rates[key] = v
	return v.Count > max
}

func (a *App) throttleRequest(w http.ResponseWriter, r *http.Request) bool {
	key := "api:" + ipOf(r)
	if !a.limitedWindow(key, 240, time.Minute) {
		return false
	}
	a.mu.Lock()
	until := a.rates[key].Expires
	a.mu.Unlock()
	seconds := int(math.Ceil(time.Until(until).Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	fail(w, 429, "请求过于频繁，请稍后重试")
	return true
}
