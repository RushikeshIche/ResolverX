# ResolverX — Architectural Decision Records (ADR)

This document records the architectural, technical, and engineering decisions made during the design and implementation of **ResolverX**.

---

## ADR-001: Language & GUI Framework Selection

### Context
ResolverX requires a cross-platform desktop interface paired with high-concurrency low-level networking capabilities for DNS query parsing, caching, and upstream forwarding.

### Decision
- **Language:** **Go (1.22+)**
  - Native concurrency primitives (`goroutines`, `sync.RWMutex`, atomic counters).
  - Excellent networking standard library (`net`, `net/url`).
  - Single-binary distribution with zero external runtime requirements.
- **GUI Toolkit:** **`fyne.io/fyne/v2`**
  - Native cross-platform rendering (Windows, macOS, Linux).
  - Built-in material-inspired modern theming (light and dark mode support).
  - Rich widget ecosystem (tabs, forms, dialogs, lists, cards).

### Consequences
- Requires a C compiler (MinGW-w64 on Windows) during developer builds to link with OpenGL and GLFW graphics drivers.
- Once compiled, the resulting executable is standalone and does not require users to install runtimes.

---

## ADR-002: Pure-Go SQLite Driver (`modernc.org/sqlite`)

### Context
Custom DNS records must persist across application restarts. While `github.com/mattn/go-sqlite3` is a common SQLite driver, it requires CGO compilation for SQLite's C source code on every build, leading to fragile builds and toolchain conflicts on Windows.

### Decision
Adopt **`modernc.org/sqlite`**, a pure-Go transpiled implementation of SQLite.

### Consequences
- Eliminates CGO requirements specifically for the database layer.
- Tests (`go test ./...`) for models, validator, and storage can run lightning-fast in in-memory mode (`:memory:`) without C compiler overhead.
- Supports full standard SQL transactions, unique constraints, and WAL (Write-Ahead Logging) mode.

---

## ADR-003: DNS Library Selection (`github.com/miekg/dns`)

### Context
A DNS resolver requires raw wire-format packet serialization and deserialization for RFC 1035 UDP and TCP DNS messages.

### Decision
Use **`github.com/miekg/dns`**, the industry standard for DNS services in Go (used by CoreDNS, Kubernetes, and Cloudflare tools).

### Consequences
- High-performance, RFC-compliant parsing of questions, answer resource records (`A`, `AAAA`), and error codes (`NOERROR`, `NXDOMAIN`, `SERVFAIL`).
- Simplifies handling of UDP packet size negotiation and message compression.

---

## ADR-004: Windows Port Selection (`5354` vs `5353` / `53`)

### Context
The standard DNS port is `53`, which requires elevated administrator privileges on Windows and Linux. The initial roadmap proposed port `5353`. However, on Windows, port `5353` UDP is strictly reserved by the operating system for **mDNS (Multicast DNS)** via `svchost.exe`, causing standard socket bindings to fail with `"An attempt was made to access a socket in a way forbidden by its access permissions"`.

### Decision
- Configure the default development listen address to **`127.0.0.1:5354`**.
- Keep port fully configurable in `config.json` and via the GUI Settings tab, allowing elevated users to switch to standard port `53` when desired.

### Consequences
- ResolverX runs out-of-the-box on Windows without administrator rights and without colliding with Windows mDNS.

---

## ADR-005: Synchronous Socket Pre-Binding in Server Lifecycle

### Context
In early implementations, `s.udpServer.ListenAndServe()` was executed inside a background goroutine. If the socket failed to bind (e.g. port already occupied or permission denied), `Server.Start()` would return `nil` before the background error occurred, giving false-positive startup notifications.

### Decision
In `internal/dns/server.go`, explicitly pre-bind sockets synchronously using:
1. `net.ListenPacket("udp", s.addr)`
2. `net.Listen("tcp", s.addr)`

Only after both sockets bind successfully does the server transition to the running state and invoke `ActivateAndServe()` in background worker goroutines.

### Consequences
- Binding failures (port conflicts, permission errors) are caught immediately and surfaced to the UI modal dialog or terminal.

---

## ADR-006: Event-Driven Cache Invalidation

### Context
When a user updates or deletes a custom DNS record via the GUI, cached upstream or stale local entries must not linger.

### Decision
The `RecordService` maintains an event bus / listener list (`OnChange(func(domain string))`).
Whenever a record is created, modified, or deleted:
1. The change is validated and committed to SQLite.
2. The `OnChange` hooks fire asynchronously.
3. The in-memory cache immediately removes all entries matching the canonical domain name.

### Consequences
- DNS lookups reflect record mutations instantaneously without clearing unrelated cached public domains.

---

## ADR-007: Thread-Safe GUI Updates via `fyne.Do`

### Context
Fyne v2.5+ enforces strict thread boundaries: any mutation of UI widgets (`label.SetText()`, `list.Refresh()`, `button.SetImportance()`) must occur on the main Fyne event loop. Updating widgets from background goroutines (such as our 1-second metric timer or real-time query log callbacks) triggers fatal concurrent access warnings.

### Decision
Wrap all UI refreshes within `internal/ui/dashboard.go` inside `fyne.Do(func() { ... })`.

### Consequences
- Smooth 60fps UI performance with zero race conditions between background networking goroutines and the Fyne rendering loop.

---

## ADR-008: Standalone CLI DNS Query Tool (`lookup.exe`)

### Context
Windows does not include the `dig` tool natively, and native Windows `nslookup` defaults strictly to port 53 and makes unsolicited reverse-lookup PTR queries that confuse port-specific development testing.

### Decision
Build a lightweight standalone CLI utility in `cmd/lookup/main.go` (`lookup.exe`) that sends RFC DNS requests directly to `127.0.0.1:5354` and displays query latency, status codes, and formatted answer sections.

### Consequences
- Developers and users can test local records and upstream resolution with a single command (`.\lookup.exe myapp.test`).
