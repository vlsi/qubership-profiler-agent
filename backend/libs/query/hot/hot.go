// Package hot is the query service's client side of the collector fan-out
// (02-read-contract.md §3, §7): replica discovery through the headless
// Service and the per-replica /internal/v1 reads whose rows feed the tier
// merge. It holds no per-replica state — the fan-out is re-issued whole on
// every page (§2.3.1).
package hot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/Netcracker/qubership-profiler-backend/libs/query/model"
	"github.com/pkg/errors"
)

type (
	// Discovery lists the base URLs of the Ready collector replicas. The DNS
	// implementation re-resolves on every request (02 §7.1); tests supply a
	// static or scripted list.
	Discovery interface {
		Replicas(ctx context.Context) ([]string, error)
	}

	// DNSDiscovery resolves the headless Service to one A record per Ready
	// replica (COLLECTOR_HEADLESS_SVC, 02 §7.1; only Ready pods are published,
	// 04-storage-layout.md §3.3).
	DNSDiscovery struct {
		Service string
		Port    int
	}

	// Window is one replica's /internal/v1/health/hot-window report (02 §3),
	// the input to the dynamic cold cutoff (§4.3).
	Window struct {
		OldestMs int64 `json:"hot_window_oldest_ms"`
		NowMs    int64 `json:"hot_window_now_ms"`
	}

	// Client issues the per-replica reads with the §7.2 per-request timeout.
	Client struct {
		http *http.Client
	}

	callsBody struct {
		Calls []model.CallJSON `json:"calls"`
	}

	podsBody struct {
		Pods []model.PodEntry `json:"pods"`
	}
)

// Replicas resolves one base URL per Ready collector pod, in a stable order.
func (d DNSDiscovery) Replicas(ctx context.Context) ([]string, error) {
	addrs, err := net.DefaultResolver.LookupHost(ctx, d.Service)
	if err != nil {
		return nil, errors.Wrapf(err, "resolve %s", d.Service)
	}
	sort.Strings(addrs)
	urls := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		urls = append(urls, "http://"+net.JoinHostPort(addr, strconv.Itoa(d.Port)))
	}
	return urls, nil
}

// NewClient builds the fan-out HTTP client (PROFILER_FANOUT_TIMEOUT, §7.2).
func NewClient(timeout time.Duration) *Client {
	return &Client{http: &http.Client{Timeout: timeout}}
}

// HotWindow fetches one replica's hot-window report.
func (c *Client) HotWindow(ctx context.Context, baseURL string) (Window, error) {
	var w Window
	err := c.getJSON(ctx, baseURL+"/internal/v1/health/hot-window", &w)
	return w, err
}

// Calls fetches one replica's rows past the cursor position: the same
// parameters as /api/v1/calls plus the keyset (02 §3, §2.3.1). The replica
// returns them already in the shared (ts_ms DESC, pk ASC) order, so the
// result plugs into the k-way merge as one run.
func (c *Client) Calls(ctx context.Context, baseURL string, q model.CallsQuery, after *model.Position, limit int) ([]model.CallRow, error) {
	v := q.Values()
	if after != nil {
		v.Set("after_ts_ms", strconv.FormatInt(after.TsMs, 10))
		v.Set("after_pk", after.PK.PathString())
	}
	v.Set("limit", strconv.Itoa(limit))
	var body callsBody
	if err := c.getJSON(ctx, baseURL+"/internal/v1/calls?"+v.Encode(), &body); err != nil {
		return nil, err
	}
	rows := make([]model.CallRow, 0, len(body.Calls))
	for _, call := range body.Calls {
		rows = append(rows, call.Row(model.TierHot))
	}
	return rows, nil
}

// Pods fetches the pod-restarts one replica holds data for in [fromMs, toMs)
// — the hot half of the §2.7 union.
func (c *Client) Pods(ctx context.Context, baseURL string, fromMs, toMs int64) ([]model.PodEntry, error) {
	v := url.Values{}
	v.Set("from", strconv.FormatInt(fromMs, 10))
	v.Set("to", strconv.FormatInt(toMs, 10))
	var body podsBody
	if err := c.getJSON(ctx, baseURL+"/internal/v1/pods?"+v.Encode(), &body); err != nil {
		return nil, err
	}
	return body.Pods, nil
}

func (c *Client) getJSON(ctx context.Context, url string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return errors.Wrap(err, "build replica request")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("GET %s: %s: %s", url, resp.Status, snippet)
	}
	return errors.Wrapf(json.NewDecoder(resp.Body).Decode(into), "decode %s", url)
}
