# ResolverX

**A local-first DNS resolver and desktop DNS manager built with Go and Fyne.**

ResolverX is a cross-platform desktop application that lets users manage custom DNS records through a graphical interface while running a local DNS resolver for domain lookups.

Create, edit, and delete DNS records, resolve custom local domains, cache DNS responses, and forward queries for public domains to an upstream DNS server—all through a single desktop application.

The goal is to make DNS management accessible through a clean GUI while providing a practical introduction to DNS protocols, networking, caching, concurrency, and systems programming in Go.

> **Project status:** Core features implemented and verified! The desktop GUI, local DNS engine, SQLite storage, upstream forwarder, and cache are fully functional.

## Features

### DNS Record Management

- Create, edit, view, and delete custom DNS records.
- Support IPv4 (`A`) and IPv6 (`AAAA`) records.
- Validate domain names, record types, and IP addresses (RFC 1123).
- Persist custom records in an embedded SQLite database across application restarts.
- Apply record changes to the running resolver instantly with automatic cache invalidation without restarting the application.
- Display configured records in a searchable, organized table with inline Add, Edit, and Delete modals.

### Local DNS Resolution

- Run a local DNS server on loopback (`127.0.0.1:5354`).
- Resolve configured custom domains using locally stored records.
- Forward queries for public domains to configurable upstream DNS servers (e.g. `1.1.1.1:53`, `8.8.8.8:53`).
- Support DNS queries over both UDP and TCP.
- Return appropriate DNS response codes (`NOERROR`, `NXDOMAIN`, `SERVFAIL`).
- Support configurable DNS listener addresses and ports via GUI Settings or `config.json`.

### DNS Caching

- Cache eligible DNS responses in memory to reduce repeated upstream requests.
- Respect DNS time-to-live (TTL) values with dynamic countdown decrements.
- Automatic LRU (Least Recently Used) cache eviction with configurable capacity limits.
- Invalidate affected cached entries instantly when relevant custom records are created, updated, or deleted.

### Desktop Interface

- Cross-platform GUI powered by **Fyne v2**.
- Dashboard displaying resolver status, Start/Stop toggle button, and live DNS statistics (Total Queries, Local Hits, Cache Hits, Upstream Lookups, Hit Ratio %).
- Real-time scrolling Query Log stream showing incoming queries, resolution source, response code, and latency.
- Dedicated Record Management interface with search filters and modal forms.
- Settings panel to configure listener addresses, upstream servers, and cache size.

### Reliability and Security

- Concurrent DNS request handling verified with Go's race detector (`go test -race ./...`).
- Synchronous socket pre-binding so port conflicts or permission errors surface immediately.
- Graceful start, stop, and shutdown of listeners.
- Bounded cache and resource usage.
- Local-only management interface on loopback by default.

## Architecture

ResolverX separates its desktop interface, record management, DNS protocol handling, and storage into independent components.

```text
                         ResolverX
                             |
                 +-----------+-----------+
                 |                       |
             Fyne GUI                DNS Engine
                 |                       |
          Record Manager          DNS Query Handler
                 |                       |
          Validation Layer        +------+------+
                 |                |             |
       Pure-Go SQLite Storage Local Records   Cache
                                  |             |
                                  +------+------+
                                         |
                                  Upstream DNS (1.1.1.1, 8.8.8.8)
```

### Components

| Component | Responsibility |
|---|---|
| Desktop UI | Provides dashboard, record management, settings, and resolver controls. |
| Record Manager | Validates and manages custom DNS records with event hooks. |
| DNS Engine | Receives and processes UDP/TCP DNS queries. |
| Record Store | Persists custom DNS records in SQLite using `modernc.org/sqlite`. |
| DNS Cache | In-memory thread-safe LRU cache with dynamic TTL decrements. |
| Upstream Forwarder | Forwards queries with sequential failover and timeout management. |
| Configuration Manager | Manages listener settings, upstream servers, and application preferences. |
| Metrics Collector | Thread-safe aggregation of query counts, hit ratio, and live query history. |

For deep-dive documentation on design choices and data flow sequence diagrams, see:
- [decision.md](decision.md) — Architectural Decision Records (ADRs).
- [flow.md](flow.md) — System startup, query resolution, and cache invalidation sequence diagrams.

## How It Works

### 1. Custom DNS Records

Users can create mappings such as:

| Domain | Type | Value |
|---|---|---|
| `myapp.test` | A | `127.0.0.1` |
| `dev.test` | A | `192.168.1.10` |
| `ipv6.test` | AAAA | `::1` |

When a matching query arrives, ResolverX immediately returns the configured record with authoritative headers instead of forwarding upstream.

### 2. Public Domain Resolution

For domains without a matching custom record, ResolverX forwards the DNS query to a configured upstream server (e.g. `1.1.1.1:53` or `8.8.8.8:53`). Responses are cached in memory so subsequent requests resolve in sub-millisecond latencies.

### 3. DNS Caching & Invalidation

- **TTL-Aware:** Answers are cached for their remaining TTL.
- **LRU Eviction:** Bounded memory usage prevents memory leaks.
- **Live Invalidation:** When a user updates or deletes a domain in the GUI, the domain's cached entry is purged instantly.

## Technology Stack

| Technology | Purpose |
|---|---|
| Go (1.22+) | Core application, networking, and concurrency |
| Fyne v2 | Cross-platform desktop GUI toolkit |
| `github.com/miekg/dns` | Low-level wire format DNS message handling |
| `modernc.org/sqlite` | Embedded relational database (Pure Go, CGO-free) |
| Go standard library | Concurrency, synchronization, and configuration |

## Project Structure

```text
ResolverX/
├── cmd/
│   ├── resolverx/
│   │   └── main.go              # Main application entrypoint (Desktop GUI & -headless mode)
│   └── lookup/
│       └── main.go              # Dedicated CLI DNS client tool for rapid testing
├── internal/
│   ├── app/
│   │   └── app.go               # Top-level lifecycle coordinator & auto-seeding
│   ├── config/
│   │   ├── config.go            # JSON configuration management, validation & defaults
│   │   └── config_test.go       # Config unit tests
│   ├── records/
│   │   ├── model.go             # DNS Record domain model (A, AAAA, FQDN normalization)
│   │   ├── validator.go         # RFC 1123 domain syntax, IPv4/IPv6, and TTL validation
│   │   ├── validator_test.go    # Validation test suite covering edge cases
│   │   ├── service.go           # Validated record CRUD & cache invalidation event bus
│   │   └── service_test.go      # Service & event notification tests
│   ├── storage/
│   │   ├── sqlite.go            # Pure Go SQLite persistent store (modernc.org/sqlite)
│   │   └── sqlite_test.go       # Storage CRUD & constraint tests (in-memory)
│   ├── dns/
│   │   ├── server.go            # Synchronous UDP & TCP DNS server listeners & lifecycle
│   │   ├── handler.go           # DNS query router (Local Store -> Cache -> Upstream)
│   │   ├── forwarder.go         # Upstream forwarder pool with failover & timeouts
│   │   ├── cache.go             # Thread-safe LRU cache with dynamic TTL decrements
│   │   ├── cache_test.go        # Cache LRU eviction, expiration, and invalidation tests
│   │   └── dns_test.go          # Mock upstream integration & 25-goroutine concurrency tests
│   ├── metrics/
│   │   └── metrics.go           # Thread-safe query counters & ring buffer query logger
│   └── ui/
│       ├── window.go            # Top-level Fyne window with navigation tabs
│       ├── dashboard.go         # Resolver toggle button, live metrics cards & query log
│       ├── records.go           # Records table, search filter, Add/Edit/Delete modals
│       └── settings.go          # Config form for sockets, upstreams, and cache
├── tests/
│   └── integration/
│       └── integration_test.go  # End-to-end client integration tests
├── config.json                  # Runtime configuration file
├── resolverx.db                 # Persistent SQLite database file
├── decision.md                  # Architectural Decision Records (ADR)
├── flow.md                      # Data flow and sequence diagrams
├── .gitignore                   # Git ignore rules
├── go.mod                       # Go module dependencies
├── go.sum                       # Go checksums
└── README.md                    # Project documentation
```

## Getting Started

### Prerequisites

- **Go:** Version 1.22 or higher.
- **Git.**
- **C Compiler (for Fyne GUI builds):** MinGW-w64 on Windows (via MSYS2), Clang on macOS, or GCC on Linux.
  - *Note:* The database layer uses `modernc.org/sqlite` which is pure Go and does not require CGO.

### Clone and Run

```bash
git clone https://github.com/RushikeshIche/ResolverX.git
cd ResolverX
```

Install dependencies:
```bash
go mod tidy
```

### Launch ResolverX

**1. Desktop Graphical Interface:**
```bash
go run ./cmd/resolverx
# OR run the compiled binary:
.\resolverx.exe
```

**2. Headless Daemon Mode (no GUI):**
```bash
go run ./cmd/resolverx -headless
```

**3. Build Standalone Executable:**
```bash
go build -o resolverx.exe ./cmd/resolverx
```

## DNS Configuration

ResolverX is configured via `config.json` in the root directory or directly through the **Settings** tab in the desktop application:

```json
{
  "listen_address": "127.0.0.1:5354",
  "upstream_servers": [
    "1.1.1.1:53",
    "8.8.8.8:53"
  ],
  "cache_enabled": true,
  "cache_max_entries": 10000,
  "log_queries": true,
  "database_path": "resolverx.db"
}
```

> **Why port 5354?**
> On Windows, port `5353` is strictly reserved by the operating system for Multicast DNS (`svchost.exe`). ResolverX binds to port `5354` by default so it runs without administrator privileges and without port conflicts. Port `53` can be configured when running with elevated permissions.

## Testing and Verification

### 1. Dedicated CLI Query Tool (`lookup.exe`)

Because Windows does not include `dig` and Windows `nslookup` only targets port 53 by default, ResolverX includes a lightweight CLI query tool:

Build the tool (or run directly):
```bash
go build -o lookup.exe ./cmd/lookup
```

Test a local custom record:
```bash
.\lookup.exe myapp.test
```
*Output:*
```text
; <<>> ResolverX Query Tool <<>> myapp.test. A
;; Server: 127.0.0.1:5354, Query time: 1.09ms, Response status: NOERROR
;; ANSWER SECTION:
  myapp.test.  300  IN  A  127.0.0.1
```

Test public upstream forwarding:
```bash
.\lookup.exe google.com
```

Test IPv6 resolution:
```bash
.\lookup.exe -type AAAA ipv6.test
```

### 2. Testing with `dig` (macOS / Linux / MSYS2)

```bash
dig @127.0.0.1 -p 5354 myapp.test A
dig @127.0.0.1 -p 5354 google.com A
```

### 3. Automated Automated Test Suite

Run all unit and integration tests:
```bash
go test -v ./...
```

Run with the **Go Race Detector**:
```bash
go test -v -race ./...
```

## License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.

## Acknowledgements

- [Go](https://go.dev/) — The Go Programming Language.
- [Fyne](https://fyne.io/) — Cross-platform GUI toolkit for Go.
- [miekg/dns](https://github.com/miekg/dns) — DNS library for Go.
- [modernc.org/sqlite](https://modernc.org/sqlite) — Pure Go SQLite implementation.

---

**ResolverX — Your DNS. Your rules. Locally managed.**
