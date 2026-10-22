# ResolverX — Execution & Data Flow Architecture

This document maps out the end-to-end data flows, lifecycle sequences, and communication patterns across ResolverX.

---

## 1. Application Initialization & Startup Flow

```mermaid
sequenceDiagram
    autonumber
    actor User
    participant Main as cmd/resolverx/main.go
    participant App as internal/app/app.go
    participant Config as internal/config
    participant Storage as internal/storage (SQLite)
    participant RecSvc as internal/records
    participant Cache as internal/dns (Cache)
    participant Server as internal/dns (Server)
    participant UI as internal/ui (Fyne GUI)

    User->>Main: Launch application (.\resolverx.exe)
    Main->>App: NewApp("config.json")
    App->>Config: LoadConfig("config.json")
    Config-->>App: Config Struct (Listen: 127.0.0.1:5354)
    App->>Storage: NewStore("resolverx.db")
    Storage->>Storage: Run migrations (create tables, indices)
    Storage-->>App: Store ready
    App->>RecSvc: NewService(store)
    App->>Cache: NewCache(maxEntries=10000)
    App->>RecSvc: OnChange(cache.Invalidate)
    App->>Server: NewServer(addr, handler)
    App-->>Main: Core App Instance ready
    Main->>App: SeedDefaultRecords()
    Note over App,Storage: If DB is empty, insert myapp.test, dev.test, ipv6.test
    Main->>Server: Start()
    Server->>Server: net.ListenPacket("udp", :5354)
    Server->>Server: net.Listen("tcp", :5354)
    Server->>Server: ActivateAndServe() in goroutines
    alt Headless Mode (-headless)
        Main->>Main: Wait for SIGINT / SIGTERM
    else GUI Mode
        Main->>UI: NewMainWindow(coreApp)
        UI->>User: Render Dashboard, Records Table, Settings Tabs
    end
```

---

## 2. DNS Query Resolution Flow

When a DNS query arrives over UDP or TCP on `127.0.0.1:5354`:

```mermaid
sequenceDiagram
    autonumber
    actor Client as DNS Client (lookup.exe / OS)
    participant Server as DNS Server (:5354)
    participant Handler as Query Handler
    participant Storage as Record Store (SQLite)
    participant Cache as Response Cache (LRU)
    participant Forwarder as Upstream Forwarder
    participant Upstream as External DNS (1.1.1.1:53)
    participant Metrics as Metrics Collector

    Client->>Server: DNS Query (e.g. myapp.test A)
    Server->>Handler: ServeDNS(w, r)
    Handler->>Storage: 1. FindRecords("myapp.test", TypeA)
    alt Local Custom Record Matched
        Storage-->>Handler: Return Record (127.0.0.1, TTL: 300)
        Handler->>Handler: Build authoritative dns.Msg (NOERROR, Answer=A: 127.0.0.1)
        Handler->>Metrics: RecordQuery(source="local", duration)
        Handler-->>Client: Respond with DNS Message
    else Local Record Not Found
        Handler->>Cache: 2. Get("google.com", TypeA, ClassINET)
        alt Response Found in Cache & Valid
            Cache-->>Handler: Cached dns.Msg (adjusted remaining TTL)
            Handler->>Metrics: RecordQuery(source="cache", duration)
            Handler-->>Client: Respond with Cached DNS Message
        else Cache Miss
            Handler->>Forwarder: 3. Forward(ctx, req)
            Forwarder->>Upstream: ExchangeContext(req, "1.1.1.1:53")
            alt Upstream Responds Successfully
                Upstream-->>Forwarder: Upstream dns.Msg (Answers, TTL)
                Forwarder-->>Handler: Return Response
                Handler->>Cache: Set(resp) -> Store in LRU
                Handler->>Metrics: RecordQuery(source="upstream", duration)
                Handler-->>Client: Respond with Upstream DNS Message
            else Upstream Timeout / Failover
                Forwarder->>Upstream: Try secondary upstream ("8.8.8.8:53")
                alt All Upstreams Fail
                    Forwarder-->>Handler: Return error
                    Handler->>Handler: Build SERVFAIL message
                    Handler->>Metrics: RecordQuery(source="error", duration)
                    Handler-->>Client: Respond with SERVFAIL
                end
            end
        end
    end
```

---

## 3. Record Mutation & Invalidation Flow

How changes made via the GUI instantly update the running DNS engine:

```mermaid
sequenceDiagram
    autonumber
    actor User
    participant RecordsUI as Records View (Fyne)
    participant RecSvc as Record Service
    participant Validator as Validator Layer
    participant Storage as SQLite Store
    participant Cache as DNS Cache
    participant Client as Future DNS Queries

    User->>RecordsUI: Enter Domain: "custom.test", IP: "10.0.0.1", TTL: 300
    User->>RecordsUI: Click "Save Record"
    RecordsUI->>RecSvc: Create(ctx, record)
    RecSvc->>Validator: ValidateRecord(record)
    Validator->>Validator: Verify RFC 1123 syntax, IP format, TTL bounds
    Validator-->>RecSvc: Validation Passed (Domain normalized)
    RecSvc->>Storage: CreateRecord(ctx, record)
    Storage->>Storage: INSERT INTO records ...
    Storage-->>RecSvc: Success
    RecSvc->>Cache: notifyChange("custom.test")
    Cache->>Cache: Invalidate("custom.test") (Purges old entries)
    RecSvc-->>RecordsUI: Success
    RecordsUI->>RecordsUI: Reload() & Refresh Table
    Note over Client,Cache: Next query for custom.test resolves directly from SQLite without restart!
```

---

## 4. Real-Time Dashboard & Query Logging Flow

```mermaid
sequenceDiagram
    autonumber
    participant Handler as DNS Handler
    participant Metrics as Metrics Collector
    participant Dashboard as Dashboard View (Fyne)
    participant UIThread as Main Fyne Event Loop

    Handler->>Metrics: RecordQuery(entry)
    Metrics->>Metrics: Increment totalQueries, localHits / cacheHits / upstreamQueries
    Metrics->>Metrics: Append to recentLogs (bounded ring buffer: 150 items)
    Metrics-->>Dashboard: OnLogEntry callback (background goroutine)
    Dashboard->>UIThread: fyne.Do(func() { ... })
    UIThread->>Dashboard: Refresh total, local, cache, and hit ratio labels
    UIThread->>Dashboard: Refresh logList widget with reverse-chronological items
    Dashboard-->>Dashboard: Visual UI updates rendered at 60fps
```
