package main

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

const videoLinkMessage = "链接仅支持 HTTPS 的 Bilibili 或 YouTube 视频播放页；不接受其他网站、短链跳转或下载链接。B站短链请先打开，再复制完整视频地址。"

var biliVideoID = regexp.MustCompile(`^(BV[A-Za-z0-9]{10}|av[1-9][0-9]{0,19})$`)
var youtubeVideoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
var videoTime = regexp.MustCompile(`^(?:[0-9]{1,7}|(?:[0-9]{1,3}h)?(?:[0-9]{1,3}m)?(?:[0-9]{1,3}s)?)$`)
var biliNumber = regexp.MustCompile(`^[1-9][0-9]{0,5}$`)
var linkCandidate = regexp.MustCompile("(?i)(?:[a-z][a-z0-9+.-]{0,24}://|(?:https?|javascript|data|file|vbscript|blob|mailto):|//|(?:[a-z0-9-]+\\.)+[a-z]{2,63}\\b)[^\\s<>\"'`，。；、（）【】]*")
var nestedLink = regexp.MustCompile(`(?i)(?:https?|ftp|file|javascript|data):|//`)

func canonicalVideo(raw string) (string, error) {
	u, e := url.Parse(raw)
	if e != nil || !strings.EqualFold(u.Scheme, "https") || u.Host == "" || u.User != nil || u.Port() != "" || u.Opaque != "" || u.RawPath != "" || strings.ContainsAny(raw, "\\\x00\r\n") {
		return "", errors.New(videoLinkMessage)
	}
	q, e := url.ParseQuery(u.RawQuery)
	if e != nil {
		return "", errors.New(videoLinkMessage)
	}
	for key, values := range q {
		switch strings.ToLower(key) {
		case "url", "redirect", "redirect_url", "redirect_uri", "target", "next", "continue", "link":
			return "", errors.New(videoLinkMessage)
		}
		for _, value := range values {
			decoded, _ := url.QueryUnescape(value)
			if nestedLink.MatchString(value) || nestedLink.MatchString(decoded) {
				return "", errors.New(videoLinkMessage)
			}
		}
	}
	host := strings.ToLower(u.Hostname())
	path := strings.TrimSuffix(u.Path, "/")
	params := url.Values{}
	target := ""
	switch host {
	case "bilibili.com", "www.bilibili.com", "m.bilibili.com":
		id := strings.TrimPrefix(path, "/video/")
		if !strings.HasPrefix(path, "/video/") || !biliVideoID.MatchString(id) {
			return "", errors.New(videoLinkMessage)
		}
		target = "https://www.bilibili.com/video/" + id + "/"
		if values := q["p"]; len(values) == 1 && biliNumber.MatchString(values[0]) {
			params.Set("p", values[0])
		}
		if values := q["t"]; len(values) == 1 && regexp.MustCompile(`^[0-9]{1,7}$`).MatchString(values[0]) {
			params.Set("t", values[0])
		}
	case "youtube.com", "www.youtube.com", "m.youtube.com", "youtu.be":
		id := ""
		if host == "youtu.be" {
			id = strings.TrimPrefix(path, "/")
		} else if path == "/watch" && len(q["v"]) == 1 {
			id = q.Get("v")
		} else {
			for _, prefix := range []string{"/shorts/", "/live/"} {
				if strings.HasPrefix(path, prefix) {
					id = strings.TrimPrefix(path, prefix)
				}
			}
		}
		if !youtubeVideoID.MatchString(id) {
			return "", errors.New(videoLinkMessage)
		}
		target = "https://www.youtube.com/watch"
		params.Set("v", id)
		if values := q["t"]; len(values) == 1 && values[0] != "" && videoTime.MatchString(values[0]) {
			params.Set("t", values[0])
		}
	default:
		return "", errors.New(videoLinkMessage)
	}
	if len(params) > 0 {
		target += "?" + params.Encode()
	}
	return target, nil
}

func normalizeVideoText(value string) (string, error) {
	positions := linkCandidate.FindAllStringIndex(value, -1)
	var result strings.Builder
	last := 0
	for _, position := range positions {
		// Contact email addresses stay plain text and are never turned into hyperlinks.
		if position[0] > 0 && value[position[0]-1] == '@' {
			continue
		}
		candidate := strings.TrimRight(value[position[0]:position[1]], ".,;!?:)]}")
		canonical, e := canonicalVideo(candidate)
		if e != nil {
			return "", e
		}
		result.WriteString(value[last:position[0]])
		result.WriteString(canonical)
		last = position[0] + len(candidate)
	}
	result.WriteString(value[last:])
	return result.String(), nil
}

func normalizeVideoFields(fields ...*string) error {
	for _, field := range fields {
		value, e := normalizeVideoText(*field)
		if e != nil {
			return e
		}
		*field = value
	}
	return nil
}
