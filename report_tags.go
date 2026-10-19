package main

import (
	"encoding/json"
	"errors"
)

type ReportTag struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}
type ReportCategory struct {
	ID    string      `json:"id"`
	Label string      `json:"label"`
	Items []ReportTag `json:"items"`
}

var reportCategories = func() []ReportCategory {
	raw, err := assets.ReadFile("web/report-categories.json")
	if err != nil {
		panic(err)
	}
	var groups []ReportCategory
	if err = json.Unmarshal(raw, &groups); err != nil {
		panic(err)
	}
	return groups
}()

// Tags describe the reporter's allegation; they never select a verdict or penalty.
func normalizeReportTags(tags []string) ([]string, error) {
	valid := map[string]bool{}
	for _, group := range reportCategories {
		for _, tag := range group.Items {
			valid[tag.ID] = true
		}
	}
	if len(tags) > 1 {
		return nil, errors.New("每份举报只能选择一个违规分类")
	}
	seen := map[string]bool{}
	out := []string{}
	for _, tag := range tags {
		if !valid[tag] {
			return nil, errors.New("请选择有效的违规标签")
		}
		if !seen[tag] {
			seen[tag] = true
			out = append(out, tag)
		}
	}
	return out, nil
}
