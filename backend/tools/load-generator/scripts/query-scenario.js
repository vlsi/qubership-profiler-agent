// T6 background query load (load-testing-plan.md §7.6, runbook
// doc/soak-runs.md): read-side traffic against the query service's /api/v1,
// on the standard k6/http module of the same custom binary.
//
// Three profiles, each its own scenario, sized by env:
//   ui       — UI_VUS users looping the UI journey: list the last hour, open
//              a random call (trace + tree; there is no bare /calls/{pk}).
//   incident — INCIDENT_VUS users hammering wide ranges for INCIDENT_DURATION
//              out of every INCIDENT_PERIOD, idle in between.
//   cold     — COLD_VUS users issuing ranges just under the wide-range guard
//              and paging through every next_cursor; every page re-lists S3
//              by design.
//
// Windows are computed here and sent as integer Unix milliseconds — the API
// accepts nothing else. Guard rejections (HTTP 400) are the point of probing
// wide ranges: they land in the query_guard_rejected counter, not in
// http_req_failed.
import http from 'k6/http';
import { sleep } from 'k6';
import { Counter } from 'k6/metrics';

function num(name, dflt) {
    const v = __ENV[name];
    return v === undefined || v === '' ? dflt : Number(v);
}

function str(name, dflt) {
    const v = __ENV[name];
    return v === undefined || v === '' ? dflt : v;
}

const BASE = str('QUERY_URL', 'http://profiler-backend-query.profiler-load.svc:8080');
const LIST_LIMIT = num('LIST_LIMIT', 50);
const THINK_SECONDS = num('THINK_SECONDS', 5);
// Wide-range probes sit just under PROFILER_WIDE_RANGE_LIMIT (default 6h)
// unless pushed over it on purpose.
const WIDE_RANGE_MINUTES = num('WIDE_RANGE_MINUTES', 350);
const COLD_MAX_PAGES = num('COLD_MAX_PAGES', 200);

const guardRejected = new Counter('query_guard_rejected');
const partialResponses = new Counter('query_partial_responses');
const coldPages = new Counter('query_cold_pages');

// A profile with 0 VUs is omitted entirely (constant-vus rejects vus: 0).
// Defaults match the soak companion: the UI journey plus incident bursts on,
// the cold-heavy profile off — it is a dedicated probe run
// (doc/soak-runs.md).
function buildScenarios() {
    const out = {};
    const add = (name, exec, vus) => {
        if (vus > 0) {
            out[name] = {
                executor: 'constant-vus',
                exec,
                vus,
                duration: str('DURATION', '2h'),
            };
        }
    };
    add('ui', 'ui', num('UI_VUS', 3));
    add('incident', 'incident', num('INCIDENT_VUS', 20));
    add('cold', 'cold', num('COLD_VUS', 0));
    return out;
}

export const options = {
    scenarios: buildScenarios(),
    tags: { testid: str('TESTID', 'query-dev') },
};

function getCalls(fromMs, toMs, extra, tags) {
    let url = `${BASE}/api/v1/calls?from=${fromMs}&to=${toMs}&limit=${LIST_LIMIT}`;
    if (extra) {
        url += extra;
    }
    const res = http.get(url, { tags });
    classify(res);
    return res;
}

// classify folds the expected non-200s into the custom counters: a guard
// rejection is a probe result, not a failure.
function classify(res) {
    if (res.status === 400) {
        guardRejected.add(1);
        return null;
    }
    if (res.status !== 200) {
        return null;
    }
    const body = res.json();
    if (body && body.partial) {
        partialResponses.add(1);
    }
    return body;
}

// ui: list the trailing UI_RANGE_MINUTES (default: the last hour), open a
// random row (trace, then tree), think, loop. Accelerated-timer stands pack
// an hour's guard budget into minutes — shrink the range with the timers.
export function ui() {
    const now = Date.now();
    const res = getCalls(now - num('UI_RANGE_MINUTES', 60) * 60 * 1000, now, '', { profile: 'ui' });
    const body = res.status === 200 ? res.json() : null;
    const calls = body && body.calls ? body.calls : [];
    if (calls.length > 0) {
        const call = calls[Math.floor(Math.random() * calls.length)];
        openCall(call, { profile: 'ui' });
    }
    sleep(THINK_SECONDS);
}

function pkPath(pk) {
    const parts = [pk.pod_namespace, pk.pod_service, pk.pod_name,
        pk.restart_time_ms, pk.trace_file_index, pk.buffer_offset, pk.record_index];
    return encodeURIComponent(parts.join(':'));
}

function openCall(call, tags) {
    const hint = `ts_ms=${call.ts_ms}&retention_class=${call.retention_class}`;
    const trace = http.get(`${BASE}/api/v1/calls/${pkPath(call.pk)}/trace?${hint}`, { tags });
    classify(trace);
    const tree = http.get(`${BASE}/api/v1/calls/${pkPath(call.pk)}/tree?${hint}`, { tags });
    classify(tree);
}

// incident: for INCIDENT_DURATION out of every INCIDENT_PERIOD, all VUs
// hammer wide ranges back to back; outside the burst they idle. Phase is
// wall-clock-based so every VU bursts together.
export function incident() {
    const periodMs = num('INCIDENT_PERIOD_MINUTES', 30) * 60 * 1000;
    const burstMs = num('INCIDENT_DURATION_MINUTES', 5) * 60 * 1000;
    const now = Date.now();
    if (now % periodMs >= burstMs) {
        sleep(5);
        return;
    }
    const wide = WIDE_RANGE_MINUTES * 60 * 1000;
    getCalls(now - wide, now, '', { profile: 'incident' });
    sleep(1);
}

// cold: a wide range paged to the end — every page re-resolves the fan-out
// and re-lists S3 (02 §2.3.1), which is exactly the cost this profile
// measures. A guard 400 on page one is a valid probe outcome.
export function cold() {
    const now = Date.now();
    const wide = WIDE_RANGE_MINUTES * 60 * 1000;
    let cursor = '';
    for (let page = 0; page < COLD_MAX_PAGES; page++) {
        const extra = cursor ? `&cursor=${encodeURIComponent(cursor)}` : '';
        const res = getCalls(now - wide, now, extra, { profile: 'cold' });
        if (res.status !== 200) {
            break;
        }
        coldPages.add(1);
        const body = res.json();
        if (!body || !body.next_cursor) {
            break;
        }
        cursor = body.next_cursor;
    }
    sleep(THINK_SECONDS);
}
