# URL Shortener — System Design

This document covers two subsystems built for a distributed URL shortener: a **region-aware, coordination-free ID generator** and a **distributed rate limiter**. Both are designed to run across multiple stateless application instances without relying on a shared in-memory lock or a single point of coordination.

---

## 1. Requirements & Constraints

### Functional requirements
- Given a long URL, generate a unique short URL.
- Given a short URL, redirect to the original long URL.
- Support custom aliases and link expiration (future work).
- Basic click analytics (future work).

### Non-functional constraints
- **Scale target:** 100M new URLs/month, 10:1 read/write ratio.
  - Average writes: ~39/s, peak (4x): ~154/s.
  - Average reads: ~386/s, peak (4x): ~1,544/s.
- **Latency:** redirect path < 100ms p99 (hot path); create path < 500ms.
- **Availability over strong consistency** (CAP): a short delay in ID uniqueness propagation is acceptable; downtime is not.
- **Short code format:** 7 characters, base62 alphabet (`0-9a-zA-Z`), fixed length via zero-padding.
- **Storage:** ~4TB over 5 years (including index overhead) — sharding is driven by **read throughput**, not by storage volume.
- **Cache:** a single-node Redis comfortably covers the estimated 100MB hot working set (top 20% of links, 80/20 access pattern).

These numbers are estimates, not hard requirements — but documenting the reasoning behind them (rather than picking constants arbitrarily) is itself part of the design.

---

## 2. Short ID Generation

### 2.1 Why a standard Snowflake ID doesn't fit

A 7-character base62 code has a maximum representable value of `62^7 - 1 ≈ 3.52 × 10^12`, which requires **at most 41 bits** (`2^41 ≈ 2.2 × 10^12 < 62^7 < 2^42 ≈ 4.4 × 10^12`). A standard Twitter Snowflake ID uses 63–64 bits (41-bit timestamp + 10-bit machine ID + 12-bit sequence), which does not fit.

The solution is a **compact, custom bit layout** that follows the same principle as Snowflake (timestamp + identity + sequence, combined without coordination between nodes) but sized to fit the available budget.

### 2.2 Bit layout

Final layout (41 bits total), evolved in two stages:

**Stage 1 — no geography:**
```
[ timestamp: 30 bits ][ node: 6 bits ][ sequence: 5 bits ]
```

**Stage 2 — region added** (to support geographically sharded Postgres + Redis, see §2.5):
```
[ timestamp: 30 bits ][ region: 2 bits ][ node: 4 bits ][ sequence: 5 bits ]
```

| Field | Bits | Range | Purpose |
|---|---|---|---|
| timestamp | 30 | ~34 years at 1s resolution, relative to a custom epoch | orders IDs roughly by creation time |
| region | 2 | 4 regions | encodes which region "owns" this ID, for stateless routing |
| node | 4 | 16 nodes per region | disambiguates concurrent generators within the same region |
| sequence | 5 | 32 IDs/node/second | disambiguates multiple IDs from the same node in the same second |

Capacity: `4 regions × 16 nodes × 32 IDs/s = 2,048 IDs/s` system-wide — comfortably above the ~154 writes/s peak computed in §1.

Shift constants (each field's shift = sum of the bits of all fields **below** it):
```go
const (
    timestampBits = 30
    regionBits    = 2
    nodeBits      = 4
    sequenceBits  = 5

    maxRegionID = -1 ^ (-1 << regionBits)
    maxNodeID   = -1 ^ (-1 << nodeBits)
    maxSequence = -1 ^ (-1 << sequenceBits)

    nodeShift      = sequenceBits                          // 5
    regionShift    = nodeBits + sequenceBits                // 9
    timestampShift = nodeBits + sequenceBits + regionBits   // 11
)
```

Assembly:
```go
id := (timestamp << timestampShift) | (regionID << regionShift) | (nodeID << nodeShift) | sequence
```

### 2.3 Custom epoch

Using the Unix epoch (1970) directly would already consume more than the 30-bit timestamp budget today. Instead, the epoch is relative to a project start date (e.g. 2020-01-01 UTC), computed once at startup:

```go
var epoch = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
```

This must be a `var`, not a `const`: `time.Date(...).Unix()` is a function call, and Go constants require compile-time-computable values.

### 2.4 Generation algorithm (per node)

Each `Generator` holds `nodeID`, `regionID`, `lastTimestamp`, and `sequence`, protected by a `sync.Mutex` (safe **within** one process; cross-node uniqueness comes from the `nodeID`/`regionID` fields, not from any shared lock):

1. Read the current timestamp (seconds since the custom epoch).
2. If it moved **backwards** relative to `lastTimestamp` (NTP adjustment), refuse to generate and return an error — generating anyway risks a duplicate ID.
3. If it's the **same** second as the last call, increment `sequence` with wraparound (`(sequence + 1) & maxSequence`). If the increment wraps back to `0`, the 32-per-second budget for this node was exhausted: busy-wait until the next second before continuing.
4. If it's a **new** second, reset `sequence` to `0`.
5. Assemble the final ID via bit-shifting and OR, as shown in §2.2.

This design has zero network calls and zero shared state between nodes — uniqueness is guaranteed structurally, not by coordination.

### 2.5 Region-aware routing without a lookup service

**The problem:** if Postgres (and its co-located Redis rate limiter) is sharded by region for latency, a link created in region A can be clicked from region B. Region B needs to know where the link's canonical data lives — without a cross-region lookup on every request.

**The solution:** embed the region directly in the ID (§2.2). Any region, upon receiving a `GET /:short_id`, decodes the region bits directly from the short code — no external metadata service required:

```go
func DecodeRegion(shortID string) (int64, error) {
    decodedID, err := DecodeBase62(shortID)
    if err != nil {
        return 0, err
    }
    return (decodedID >> regionShift) & maxRegionID, nil
}
```

This makes the rate limiter for a given link consistently route to the link's **home region**, giving exact (not approximate) per-link counting without a dedicated centralized rate-limit service — the "centralization" is already implicit in how the ID decodes. The trade-off: a link that goes viral in a region other than its origin pays one cross-region hop per check; this is accepted as rare and acceptable, versus paying that cost on 100% of traffic with a single global rate-limit store.

### 2.6 Base62 encoding/decoding

**Encoding** (`int64 → 7-char string`): repeated division by 62, using the remainder as an index into the alphabet, reversing the result, and left-padding with `"0"` to a fixed 7 characters.

**Decoding** (`string → int64`): Horner's method, left to right — `result = result*62 + digit(char)` — avoiding the need to track positional powers of 62 separately. A pre-built `map[byte]int64` reverse index (built once at package init, via a package-level `var` calling a builder function) makes character lookup O(1). Invalid characters are rejected explicitly, since Go maps silently return the zero value for missing keys — without an explicit existence check, an invalid character and the valid character `'0'` are indistinguishable.

Round-trip correctness (`Decode(Encode(x)) == x`) is verified across edge cases: `0`, the last alphabet character, exact base rollovers, and full 41-bit values.

---

## 3. Rate Limiting

### 3.1 Algorithm: token bucket

Each rate-limited key (a client IP, or a link's short ID) has:
- `currentTokens` (float64) — available capacity right now.
- `lastTimestamp` — when the bucket was last touched.
- `maxTokens`, `refillRate` (tokens/second) — fixed configuration, shared across all keys served by one limiter instance.

**Lazy refill:** there is no background process replenishing tokens. On every check, the elapsed time since `lastTimestamp` is multiplied by `refillRate` and **added** to `currentTokens` (capped at `maxTokens`), before deciding whether to allow the request:

```go
elapsed := now.Sub(b.lastTimestamp)
tokensToAdd := elapsed.Seconds() * refillRate
b.currentTokens = min(b.currentTokens + tokensToAdd, maxTokens)
```

Using `float64` (not an integer type) for `currentTokens` matters: with a fractional refill rate (e.g. `10/60 ≈ 0.1667` tokens/s), an integer type would silently truncate the fractional part on every check, and a key polled frequently in small time slices would never accumulate tokens correctly.

### 3.2 Interface: supporting two backends

```go
type Limiter interface {
    Allow(ctx context.Context, key string) (bool, error)
}
```

`Allow` is a **single, high-level** operation rather than exposing `Get`/`Set` primitives. This matters because of what happened when we tried the simpler design (see §3.3): the atomicity of "read state, compute, decide, write state" needs to be guaranteed **inside** each implementation, using whatever mechanism fits its execution context — a `sync.Mutex` for in-memory, a Redis Lua script for a shared store. A low-level `Get`/`Set` interface pushes that responsibility to the caller, which cannot provide it correctly across processes.

### 3.3 Why a client-side mutex doesn't work for a shared store

A deliberate experiment: two separate OS processes, each with its own `sync.Mutex`, both incrementing the same counter in Redis via `GET` → sleep → `SET`.

Result: with 150 increments per process (300 expected), the final value was **154** — 146 updates silently lost. Each process's mutex worked perfectly *within* that process; neither mutex has any knowledge of the other process's existence, because a `sync.Mutex` lives in one process's memory space. This is the classic **lost update** problem: process A reads 42, process B reads 42 before A writes back, both compute 43, both write 43 — one increment vanishes.

**Conclusion:** for state shared across processes/machines, atomicity has to be enforced by the shared store itself, not by any client-side lock.

### 3.4 In-memory implementation

```go
type InMemoryLimiter struct {
    mu         sync.Mutex
    buckets    map[string]*bucket
    maxTokens  float64
    refillRate float64
}
```

- `buckets` maps to `*bucket` (pointer), not `bucket` (value) — Go map values are not addressable, so `buckets[key].field = x` does not compile on a `map[string]bucket`.
- `maxTokens`/`refillRate` live once on the limiter, not duplicated per bucket, since they are fixed configuration shared by every key served by that instance.
- New keys start "full" (`currentTokens = maxTokens - requestCost`, charging for the current request immediately) rather than empty — this matches typical rate limiter UX (an unused key can burst up to capacity right away) and avoids granting the first request "for free" without decrementing.

Verified behavior:
- A burst of 25 requests against `maxTokens=20` yields exactly 20 allowed, 5 denied.
- 20 goroutines × 10 requests each (200 total) against `maxTokens=50`, with `refillRate=0` for a deterministic expectation, yields exactly 50 allowed under `go test -race`.
- Tokens are correctly capped at `maxTokens` even after long idle periods (no unbounded accumulation).
- Different keys have fully independent bucket state.

### 3.5 Redis implementation

**Design decisions:**
- **State representation:** one Redis **Hash** per bucket (fields `tokens`, `timestamp`), not two separate keys — a single `EXPIRE` on the hash keeps both fields' TTL in sync, avoiding the case where one field expires and the other doesn't.
- **Determinism:** the current time (`now`) is computed by the **Go client** and passed as a Lua `ARGV`, rather than the script calling `redis.call('TIME')` internally — keeping the script's behavior independent of server-side non-determinism, the same principle behind using a fixed, externally-supplied epoch in the ID generator (§2.3).
- **Atomicity:** Redis executes Lua scripts single-threaded on the server; the entire read-refill-decide-write sequence for a bucket happens without any other client able to interleave a command in the middle.

**Lua script** (`token_bucket.lua`):
```lua
-- KEYS[1] = bucket hash key
-- ARGV[1] = maxTokens, ARGV[2] = refillRate, ARGV[3] = requestCost, ARGV[4] = now

local maxTokens = tonumber(ARGV[1])
local refillRate = tonumber(ARGV[2])
local requestCost = tonumber(ARGV[3])
local now = tonumber(ARGV[4])

local vals = redis.call('HMGET', KEYS[1], 'tokens', 'timestamp')

local currentTokens
local lastTimestamp

if vals[1] == false then
    -- Redis-Lua represents a missing HMGET field as `false`, not `nil`
    currentTokens = maxTokens
    lastTimestamp = now
else
    -- everything from Redis arrives as a string, even numbers
    currentTokens = tonumber(vals[1])
    lastTimestamp = tonumber(vals[2])
end

local elapsed = now - lastTimestamp
if elapsed < 0 then elapsed = 0 end  -- defensive: clock moved backwards

local tokensToAdd = elapsed * refillRate
currentTokens = math.min(currentTokens + tokensToAdd, maxTokens)

local allowed = 0
if currentTokens >= requestCost then
    currentTokens = currentTokens - requestCost
    allowed = 1
end

redis.call('HMSET', KEYS[1], 'tokens', currentTokens, 'timestamp', now)

local ttl = 3600
if refillRate > 0 then
    ttl = math.ceil(maxTokens / refillRate) + 60
end
redis.call('EXPIRE', KEYS[1], ttl)

return allowed
```

**Go wrapper:**
```go
//go:embed token_bucket.lua
var tokenBucketScript string

type RedisLimiter struct {
    client     *redis.Client
    namespace  string
    maxTokens  float64
    refillRate float64
    script     *redis.Script
}

func NewRedisLimiter(client *redis.Client, namespace string, maxTokens, refillRate float64) *RedisLimiter {
    return &RedisLimiter{
        client: client, namespace: namespace,
        maxTokens: maxTokens, refillRate: refillRate,
        script: redis.NewScript(tokenBucketScript),
    }
}

func (l *RedisLimiter) Allow(ctx context.Context, key string) (bool, error) {
    now := float64(time.Now().Unix())
    combinedKey := "bucket:" + l.namespace + ":" + key

    result, err := l.script.Run(ctx, l.client, []string{combinedKey},
        l.maxTokens, l.refillRate, requestCost, now).Int()
    if err != nil {
        return false, err
    }
    return result == 1, nil
}
```

**Why `//go:embed` instead of `os.ReadFile`:** the Lua source is compiled directly into the binary. A missing script file becomes a **build-time** error (the binary simply won't compile), not a runtime failure discovered in production — and it removes any dependency on the script file's location on disk at deploy time.

**Why the `namespace` field:** `Allow`'s key derivation (`"bucket:" + namespace + ":" + key`) has no inherent way to know *which logical dimension* a key belongs to. Using the same limiter instance for two different purposes (e.g. IP-based and link-based limiting) without namespacing risks a Redis key collision if the two key spaces ever overlap (e.g. a client IP string happening to equal a short ID string). Embedding a fixed namespace **in the instance**, rather than relying on callers to remember to prefix manually, makes the separation structural rather than a convention that can be forgotten. Verified directly against Redis: two limiter instances (`get:ip`, `get:link`) given the identical logical key produce two independent keys (`bucket:get:ip:192.168.1.1`, `bucket:get:link:192.168.1.1`) with independent bucket state.

**Concurrency verification:** 20 goroutines × 10 requests (200 total) against a real Redis instance, `maxTokens=50`, yields exactly 50 allowed — confirming the Lua script provides the atomicity that a client-side mutex could not (§3.3).

### 3.6 HTTP integration

Three separate `RedisLimiter` instances, one per rate-limiting purpose, each with its own namespace and configuration:

```go
rlPost := NewRedisLimiter(client, "post", 10, 1.0)      // creation: stricter
rlIP    := NewRedisLimiter(client, "get:ip", 100, 5.0)   // read, by client
rlLink  := NewRedisLimiter(client, "get:link", 50, 2.0)  // read, by link
```

- **`POST /url`** is limited on a single dimension: client IP (no auth is implemented yet, so IP is the only available identity; this should become "creator ID / API key" once auth exists).
- **`GET /url/{id}`** is limited on **two** dimensions simultaneously: client IP (defends against distributed scraping across many links) and short ID (defends against a single link being flooded). The IP check runs first; if it already denies, the link check is skipped to avoid a redundant Redis round-trip.
- **Failure mode:** if Redis itself is unreachable, middleware fails **open** (allows the request rather than blocking all traffic) — a deliberate choice prioritizing availability over strict enforcement during an infrastructure outage; the alternative (fail-closed) is valid too, depending on the threat model, and should be revisited when auth and abuse patterns are better understood.

```go
r.With(rateLimitByIP(rlPost)).Post("/url", CreateShortURL)
r.With(rateLimitByIPAndLink(rlIP, rlLink)).Get("/url/{id}", GetShortURL)
```

Verified end-to-end against a real running server: a burst of 12 `POST /url` requests against `maxTokens=10` returns 10× `200` followed by 2× `429`; two different `GET /url/{id}` short IDs from the same client correctly maintain independent link-level buckets while sharing the same IP-level bucket.

---

## 4. Testing Strategy

- **Unit tests** for pure logic (bit math, base62 round-trips, token bucket arithmetic) with table-driven cases covering boundaries (`0`, max values, rollovers).
- **Concurrency tests** run with `go test -race`, exercising multiple goroutines against shared state (ID generation across simulated nodes, rate limiter bucket contention) to catch data races that would not surface in single-threaded testing.
- **Integration tests** against a real Redis instance (not mocked) for the distributed rate limiter — the correctness being verified (atomicity across processes) is specifically a property that in-process mocks cannot exercise.
- **End-to-end tests** against a running HTTP server with real requests, verifying observable behavior (status codes) rather than internal state.

---

## 5. Open Questions / Future Work

- Replace IP-based rate limiting on `POST /url` with authenticated identity (API key / user ID) once auth is implemented.
- Revisit fail-open vs. fail-closed behavior once real abuse patterns and SLAs are better understood.
- `clientIP` currently reads `r.RemoteAddr` directly; behind a load balancer or reverse proxy, this needs to read `X-Forwarded-For` (with appropriate trust boundaries to avoid header spoofing).
- Postgres regional sharding and its interaction with the region-encoded ID (§2.5) is designed but not yet implemented.
- Redis client version pinned to v9.5.1 in this environment due to a sandboxed Go 1.22 toolchain; production should track the latest `github.com/redis/go-redis/v9` release.