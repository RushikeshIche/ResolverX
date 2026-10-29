# ResolverX

> **Your DNS. Your rules. Locally managed.**

**ResolverX** is a local-first DNS resolver, custom shortcut manager, and desktop DNS control panel built with **Go** and **Fyne**. It provides a graphical interface for managing custom DNS records and HTTP URL redirects while running a local DNS resolver for domain lookups.

Create, edit, and delete DNS records; configure custom URL shortcuts; cache DNS responses; and forward public-domain queries to upstream DNS servers—all from one desktop application.

> **Project status:** Core features implemented and verified, including the desktop GUI, local DNS engine, URL redirect server, SQLite storage, upstream forwarder, and DNS cache.

## Contents

- [Features](#features)
- [Screenshots](#screenshots)
- [Architecture](#architecture)
- [How It Works](#how-it-works)
- [Technology Stack](#technology-stack)
- [Project Structure](#project-structure)
- [Getting Started](#getting-started)
- [Configuration](#configuration)
- [Testing and Verification](#testing-and-verification)
- [License](#license)
- [Acknowledgements](#acknowledgements)

## Features

### URL Redirects and Local Shortcuts

- Create, view, edit, and delete HTTP redirects and custom web shortcuts.
- Configure domain-level shortcuts such as `gitrush.test`, path-level shortcuts such as `dev/app`, and custom destination URLs.
- Automatically create and manage the underlying DNS `A` records (`127.0.0.1`) for shortcut domains.
- Run a built-in HTTP redirect server with domain and path pattern matching.
- Synchronize shortcut domains with the local Windows hosts file (`C:\Windows\System32\drivers\etc\hosts`) when started with elevated privileges.

**Example**

```text
http://gitrush.test  →  https://github.com/RushikeshIche
```

### DNS Record Management

- Create, view, edit, and delete custom DNS records.
- Support IPv4 (`A`) and IPv6 (`AAAA`) records.
- Validate domain names, record types, and IP addresses according to RFC 1123 domain-name syntax and applicable address validation.
- Persist DNS records and URL redirects in an embedded SQLite database across application restarts.
- Apply record changes immediately, with automatic cache invalidation—no application restart required.
- Browse searchable tables with inline Add, Edit, and Delete forms.

### Local DNS Resolution

- Run a local DNS server on `127.0.0.1:5354` by default.
- Resolve configured custom domains using locally stored records.
- Forward public-domain queries to configurable upstream DNS servers, such as `1.1.1.1:53` and `8.8.8.8:53`.
- Support DNS over both UDP and TCP.
- Return appropriate DNS response codes, including `NOERROR`, `NXDOMAIN`, and `SERVFAIL`.
- Configure listener addresses and ports through the Settings panel or `config.json`.

### DNS Caching

- Cache eligible DNS responses in memory to reduce repeated upstream requests.
- Respect DNS time-to-live (TTL) values, including dynamic TTL countdowns.
- Evict least-recently-used entries using a configurable cache capacity.
- Invalidate affected cache entries when custom records or redirects are created, updated, or deleted.

### Desktop Interface

- Cross-platform desktop GUI powered by **Fyne v2**.
- Dashboard with resolver status, Start/Stop controls, and live DNS statistics:
  - Total queries
  - Local hits
  - Cache hits
  - Upstream lookups
  - Hit ratio
- Real-time query log showing incoming queries, resolution source, response code, and latency.
- Dedicated DNS Record Management and URL Redirect Management views with search filters and modal forms.
- Settings panel for listener addresses, redirect ports, upstream servers, and cache size.

### Reliability and Security

- Single-instance enforcement using platform-native named mutexes, including `Global\ResolverX_SingleInstance_Mutex` on Windows.
- Concurrent DNS request handling tested with Go's race detector (`go test -race ./...`).
- Synchronous socket pre-binding so port conflicts and permission errors appear immediately.
- Graceful startup, shutdown, and listener lifecycle management.
- Local-only management interface on loopback by default.

## Screenshots


### Dashboard and Query Log

<img width="1065" height="694" alt="Dashboard" src="https://github.com/user-attachments/assets/4f75a668-141e-4ca2-8be8-71b57c3d74ce" />

### DNS Record Management

<img width="1072" height="424" alt="DNS Record" src="https://github.com/user-attachments/assets/97ef5f3e-80ad-4fb5-8c54-6fbeed855268" />

### URL Redirects and Shortcuts

<img width="1064" height="279" alt="URL Redirects" src="https://github.com/user-attachments/assets/e06bd97e-6a4b-44a9-ae65-9e7849f89b6f" />

### Settings Panel

<img width="1062" height="505" alt="Settings" src="https://github.com/user-attachments/assets/b6863f62-e542-425f-9bcb-16af4b68233a" />

## Architecture

ResolverX separates its desktop interface, record and redirect management, DNS protocol handling, redirect processing, and storage into independent components.

```text
                                  ResolverX
                                      |
                  +-------------------+-------------------+
                  |                                       |
               Fyne GUI                               DNS Engine
                  |                                       |
        +---------+---------+                         DNS Query Handler
        |                   |                             |
  Record Manager    Redirect Manager                +------+------+
        |                   |                       |             |
  Validator Layer    Redirect Service         Local Records     Cache
        |                   |                       |             |
        +---------+---------+                       +------+------+
                  |                                        |
      Pure-Go SQLite Storage                  Upstream DNS (1.1.1.1, 8.8.8.8)
                  |
      HTTP Redirect Server (Port 80)
```

### Component Responsibilities

| Component | Responsibility |
|---|---|
| Desktop UI | Dashboard, record management, URL redirect management, settings, and resolver controls |
| Record Manager | Validates and manages custom DNS records with event hooks |
| Redirect Manager | Manages HTTP redirects and custom shortcut URLs, and synchronizes DNS `A` records |
| DNS Engine | Receives and processes UDP/TCP DNS queries |
| Redirect Server | Handles HTTP requests and redirects based on configured rules |
| Record Store | Persists DNS records and URL redirects in SQLite using `modernc.org/sqlite` |
| DNS Cache | Thread-safe in-memory LRU cache with dynamic TTL countdowns |
| Upstream Forwarder | Forwards queries with sequential failover and timeout management |
| Configuration Manager | Manages listener settings, upstream servers, and application preferences |
| Metrics Collector | Aggregates query counts, hit ratio, and live query history in a thread-safe manner |

For deeper design documentation, see:

- [`decision.md`](decision.md) — Architectural Decision Records (ADRs)
- [`flow.md`](flow.md) — Startup, query resolution, and cache-invalidation sequence diagrams

## How It Works

### 1. Custom DNS Records

Create mappings such as:

| Domain | Type | Value |
|---|---|---|
| `myapp.test` | `A` | `127.0.0.1` |
| `dev.test` | `A` | `192.168.1.10` |
| `ipv6.test` | `AAAA` | `::1` |

When a matching query arrives, ResolverX returns the configured record locally instead of forwarding the query upstream.

### 2. URL Redirects and Custom Shortcuts

Define domain or path shortcuts that redirect to destination URLs:

| Shortcut / Source Domain | Destination URL | HTTP Status | Automatic DNS Record |
|---|---|---|---|
| `gitrush.test` | `https://github.com/RushikeshIche` | `302 Found` | `gitrush.test → 127.0.0.1` |
| `resolverx.local` | `https://github.com/RushikeshIche/ResolverX` | `301 Moved Permanently` | `resolverx.local → 127.0.0.1` |

When a domain shortcut is saved, ResolverX automatically creates a corresponding DNS `A` record pointing to `127.0.0.1`. When run with Administrator privileges, it also updates the Windows hosts file so that entering a shortcut URL directly in a browser can redirect to its destination.

### 3. Public Domain Resolution

If a domain has no matching custom record or shortcut, ResolverX forwards the DNS query to a configured upstream server, such as `1.1.1.1:53` or `8.8.8.8:53`.

Responses are cached in memory, allowing subsequent eligible lookups to be served more quickly.

### 4. DNS Caching and Invalidation

- **TTL-aware caching:** Cached answers respect their remaining TTL.
- **LRU eviction:** Configurable capacity helps bound memory use.
- **Live invalidation:** Updating or deleting a domain or redirect purges the affected cached entry.

## Technology Stack

| Technology | Purpose |
|---|---|
| Go 1.22+ | Core application, networking, and concurrency |
| Fyne v2 | Cross-platform desktop GUI toolkit |
| [`github.com/miekg/dns`](https://github.com/miekg/dns) | DNS message and protocol handling |
| [`modernc.org/sqlite`](https://modernc.org/sqlite) | Embedded SQLite database implemented in pure Go, without CGO |
| Go standard library | Concurrency, HTTP redirect server, synchronization, and configuration |

## Project Structure

```text
ResolverX/
├── cmd/
│   ├── resolverx/
│   │   └── main.go                 # App entry point (GUI and -headless mode)
│   └── lookup/
│       └── main.go                 # CLI DNS client for testing
├── internal/
│   ├── app/
│   │   └── app.go                  # Lifecycle coordinator and auto-seeding
│   ├── config/
│   │   ├── config.go               # JSON configuration, validation, defaults
│   │   └── config_test.go          # Configuration tests
│   ├── records/
│   │   ├── model.go                # DNS record model (A, AAAA, FQDN normalization)
│   │   ├── validator.go            # Domain, IP address, and TTL validation
│   │   ├── validator_test.go       # Validation edge-case tests
│   │   ├── service.go              # CRUD service and cache invalidation events
│   │   └── service_test.go         # Service and event notification tests
│   ├── redirects/
│   │   ├── model.go                # Redirect model (shortcut, target, status)
│   │   ├── service.go              # Redirect CRUD and DNS A-record synchronization
│   │   ├── server.go               # HTTP redirect listener and matcher
│   │   └── redirects_test.go       # Redirect service and handler tests
│   ├── storage/
│   │   ├── sqlite.go               # Pure-Go SQLite storage
│   │   └── sqlite_test.go          # Storage CRUD and constraint tests
│   ├── dns/
│   │   ├── server.go               # UDP/TCP DNS listeners and lifecycle
│   │   ├── handler.go              # Query routing (Local Store → Cache → Upstream)
│   │   ├── forwarder.go            # Upstream pool, failover, and timeouts
│   │   ├── cache.go                 # Thread-safe LRU cache with TTL handling
│   │   ├── cache_test.go            # LRU eviction, expiration, invalidation tests
│   │   └── dns_test.go              # Mock upstream and concurrency tests
│   ├── metrics/
│   │   └── metrics.go              # Thread-safe counters and ring-buffer query log
│   └── ui/
│       ├── window.go               # Main Fyne window and navigation
│       ├── dashboard.go            # Resolver controls, metrics, query log
│       ├── records.go               # Records table, filters, and CRUD modals
│       ├── redirects.go             # Redirect table and shortcut forms
│       └── settings.go              # Listener, upstream, and cache settings
├── tests/
│   └── integration/
│       └── integration_test.go     # End-to-end client integration tests
├── config.json                     # Runtime configuration
├── resolverx.db                    # Persistent SQLite database (runtime)
├── decision.md                     # Architectural Decision Records
├── flow.md                          # Data-flow and sequence diagrams
├── .gitignore
├── go.mod
├── go.sum
└── README.md
```

## Getting Started

### Prerequisites

- **Go:** Version 1.22 or higher
- **Git**
- **C compiler for Fyne GUI builds:**
  - Windows: MinGW-w64 via MSYS2
  - macOS: Clang
  - Linux: GCC

> **Note:** The database layer uses `modernc.org/sqlite`, a pure-Go implementation, and does not require CGO. A compiler may still be required for Fyne's GUI dependencies on your platform.

### Clone the Repository

```bash
git clone https://github.com/RushikeshIche/ResolverX.git
cd ResolverX
```

### Install Dependencies

```bash
go mod tidy
```

### Launch ResolverX

> **URL shortcuts and hosts-file sync:** Run ResolverX with Administrator privileges on Windows (or equivalent elevated privileges where required) so it can bind to HTTP port 80 and update the system hosts file.

**1. Desktop graphical interface**

```bash
go run ./cmd/resolverx
```

Or run the compiled executable:

```powershell
.\resolverx.exe
```

**2. Headless daemon mode (no GUI)**

```bash
go run ./cmd/resolverx -headless
```

**3. Build a standalone executable**

```bash
go build -o resolverx.exe ./cmd/resolverx
```

## Configuration

ResolverX reads `config.json` from the project root. Settings can also be changed through the desktop application's Settings panel.

```json
{
  "listen_address": "127.0.0.1:5354",
  "http_redirect_address": "127.0.0.1:80",
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

### Why Port 5354?

On Windows, port `5353` may be used by Multicast DNS (mDNS). ResolverX uses `127.0.0.1:5354` by default to reduce port conflicts and avoid requiring elevated permissions for that DNS listener. Port `53` can be configured when running with the necessary permissions.

## Testing and Verification

### 1. Test URL Redirects in a Browser

Run ResolverX with the required privileges, create a redirect rule such as `gitrush.test → https://github.com/RushikeshIche`, then open:

```text
http://gitrush.test
```

The browser should be redirected to the configured destination.

### 2. Test DNS with the CLI Tool

Windows does not provide `dig` by default, and `nslookup` typically targets port `53`. ResolverX includes a lightweight CLI tool that can query the configured listener on port `5354`.

Build the tool:

```bash
go build -o lookup.exe ./cmd/lookup
```

Test a local record:

```powershell
.\lookup.exe myapp.test
```

Example output:

```text
; <<>> ResolverX Query Tool <<>> myapp.test. A
;; Server: 127.0.0.1:5354, Query time: 1.09ms, Response status: NOERROR
;; ANSWER SECTION:
myapp.test.  300  IN  A  127.0.0.1
```

Test public upstream forwarding:

```powershell
.\lookup.exe google.com
```

Test IPv6 resolution:

```powershell
.\lookup.exe -type AAAA ipv6.test
```

### 3. Run the Automated Test Suite

Run unit and integration tests:

```bash
go test -v ./...
```

Run tests with Go's race detector:

```bash
go test -v -race ./...
```

## License

This project is licensed under the [MIT License](LICENSE).

## Acknowledgements

- [Go](https://go.dev/) — The Go programming language
- [Fyne](https://fyne.io/) — Cross-platform GUI toolkit for Go
- [miekg/dns](https://github.com/miekg/dns) — DNS library for Go
- [modernc.org/sqlite](https://modernc.org/sqlite) — Pure-Go SQLite implementation

---

<p align="center">
  <strong>ResolverX — Your DNS. Your rules. Locally managed.</strong>
</p>
