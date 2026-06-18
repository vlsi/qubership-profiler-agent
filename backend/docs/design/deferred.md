# Deferred design ideas

Design-level ideas that surfaced during Stage 0 (contracts) but are intentionally out of scope for the initial implementation. Each entry should explain WHAT, WHY deferred, and the trigger that would bring it back into scope.

## `cutoff=strict` query parameter

**What.** A `?cutoff=strict` flag on `/api/v1/calls` that disables the hot/cold overlap window: hot would be queried for `(now - flush_interval, now]` and cold for `[from, now - flush_interval]`, with no overlap and no deduplication. Reduces query CPU at the cost of a brief (~seconds) window where a Call flushed exactly at the boundary may be temporarily missing.

**Why deferred.** No identified MVP consumer that runs queries frequently enough for the dedup CPU to matter, while also being tolerant of momentary gaps. The two profiles that would benefit — dashboards refreshing every 10 s, alerting rules — do not yet exist as integrations.

**Trigger to revisit.** First concrete consumer that profiles `query` and identifies the dedup pass as a bottleneck under sustained refresh load.

**Implementation note.** The hot/cold model in `02-read-contract.md` §4 already separates hot retention from flush interval, so adding the strict variant is a localized change in `query`'s range planner. No data-model impact.

## Tree endpoint in alternate formats (Protobuf / JSON)

**What.** Alongside the MVP MessagePack encoding (`02-read-contract.md` §2.5), expose the same data under additional `?format=…` variants — typically `?format=proto` for partners who want a formal schema and `?format=json` for human debugging via curl.

**Why deferred.** MVP picks MessagePack + int-keyed maps (single in-team API surface). Multiple encodings carry a multiplicative testing burden, and the int-keyed map shape already maps 1:1 to protobuf field tags — a future migration is mechanical.

**Trigger to revisit.** External / third-party integration that requires a formal `.proto` schema (e.g. partner SDK distribution), or operational need to debug wire format via curl in production.
