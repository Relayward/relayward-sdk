# Authorization Policy v1

`policy.reconcile` is a full, versioned snapshot of every authorization owned by one node. A newer generation replaces the prior snapshot atomically; replaying the same generation and content is idempotent, while conflicting content at the same generation is rejected. Authorizations and service bindings are sorted and unique so their serialized form is deterministic.

Each policy contains the immutable authorization ID and start time, administrator-enabled state, optional traffic limit and expiry, reset rule, center-calculated current period, optional soft IP limit, activity and block durations, and enabled runtime service bindings. The Agent persists the snapshot before applying it. Omitting an authorization removes its policy and causes its previously managed services and blocks to be cleared.

## Deterministic Periods

The shared `policy/v1` package is the only period algorithm used by the center and Agent. Both sides calculate an ID and UTC boundaries from the same reset rule, authorization start, and evaluation time:

- `never`: starts at authorization creation and has no end.
- `daily`: local midnight to the next local midnight.
- `weekly`: the configured ISO weekday, 1 for Monday through 7 for Sunday.
- `monthly`: the configured billing day; a day absent from a month is clamped to that month's final day.
- `interval_days`: calendar-day intervals from the supplied anchor in its configured timezone, preserving local wall-clock time across daylight-saving changes.

No period begins before the authorization. Period IDs are deterministic hashes of the effective UTC start. The Agent stores the last period ID and rolls forward locally while disconnected; the center-provided current period detects algorithm or clock divergence during reconciliation.

## Enforcement

Traffic limits aggregate upload and download across all bound runtime plugins. Runtime counters are converted to deltas locally, but Agent-to-center traffic events are absolute ledger snapshots with revisions. This makes retries and a crash between ledger persistence and event enqueue harmless: a later snapshot repairs center state without double counting.

Administrator disablement, expiry, and quota exhaustion disable every binding locally. A new period re-enables bindings only while the authorization remains administratively enabled and unexpired.

For a soft IP limit, the first N source addresses active inside the configured window occupy slots. A later address cannot evict an active slot and is added to the full plugin block set until its block expiry. This is deliberately a post-observation control and does not claim strict first-packet prevention.
