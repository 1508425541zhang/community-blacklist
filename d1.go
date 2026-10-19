package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

type d1Store struct {
	endpoint, token string
	client          *http.Client
}

func newD1Store() (*d1Store, error) {
	a, d, t := os.Getenv("CLOUDFLARE_ACCOUNT_ID"), os.Getenv("D1_DATABASE_ID"), os.Getenv("CLOUDFLARE_API_TOKEN")
	if a == "" || d == "" || t == "" {
		return nil, errors.New("D1配置不完整")
	}
	return &d1Store{"https://api.cloudflare.com/client/v4/accounts/" + url.PathEscape(a) + "/d1/database/" + url.PathEscape(d) + "/query", t, &http.Client{Timeout: 10 * time.Second}}, nil
}

type queryResult struct {
	Success bool              `json:"success"`
	Results []json.RawMessage `json:"results"`
	Meta    struct {
		Changes int `json:"changes"`
	} `json:"meta"`
}

func (d *d1Store) query(ctx context.Context, sql string, params []any) (queryResult, error) {
	b, _ := json.Marshal(map[string]any{"sql": sql, "params": params})
	r, e := http.NewRequestWithContext(ctx, "POST", d.endpoint, bytes.NewReader(b))
	if e != nil {
		return queryResult{}, e
	}
	r.Header.Set("Authorization", "Bearer "+d.token)
	r.Header.Set("Content-Type", "application/json")
	res, e := d.client.Do(r)
	if e != nil {
		return queryResult{}, errors.New("D1请求失败")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return queryResult{}, fmt.Errorf("D1状态 %d", res.StatusCode)
	}
	var body struct {
		Success bool          `json:"success"`
		Result  []queryResult `json:"result"`
	}
	if e = json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(&body); e != nil {
		return queryResult{}, errors.New("无效D1响应")
	}
	if !body.Success || len(body.Result) != 1 || !body.Result[0].Success {
		return queryResult{}, errors.New("D1查询失败")
	}
	return body.Result[0], nil
}
func (d *d1Store) Load(ctx context.Context) (State, int64, error) {
	res, e := d.query(ctx, "SELECT payload, revision FROM alliance_state WHERE id = 1", []any{})
	if e != nil {
		return State{}, 0, e
	}
	if len(res.Results) == 0 {
		return State{}, 0, nil
	}
	var row struct {
		Payload  string `json:"payload"`
		Revision int64  `json:"revision"`
	}
	if e = json.Unmarshal(res.Results[0], &row); e != nil {
		return State{}, 0, e
	}
	var s State
	e = json.Unmarshal([]byte(row.Payload), &s)
	return s, row.Revision, e
}
func (d *d1Store) Save(ctx context.Context, s State, rev int64) error {
	b, e := json.Marshal(s)
	if e != nil {
		return e
	}
	if len(b) > 1<<20 {
		return errors.New("数据量超过当前存储上限")
	}
	var res queryResult
	if rev == 0 {
		res, e = d.query(ctx, "INSERT INTO alliance_state(id,payload,revision) VALUES(1,?,1) ON CONFLICT(id) DO NOTHING", []any{string(b)})
	} else {
		res, e = d.query(ctx, "UPDATE alliance_state SET payload = ?, revision = revision + 1 WHERE id = 1 AND revision = ?", []any{string(b), rev})
	}
	if e != nil {
		return e
	}
	if res.Meta.Changes != 1 {
		return conflict
	}
	return nil
}
