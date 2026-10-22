package dns

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/RushikeshIche/ResolverX/internal/metrics"
	"github.com/RushikeshIche/ResolverX/internal/records"
)

// Handler processes DNS requests using local records, response cache, and upstream forwarding.
type Handler struct {
	records   *records.Service
	cache     *Cache
	forwarder *Forwarder
	metrics   *metrics.Collector
}

// NewHandler creates a new DNS query handler.
func NewHandler(recSvc *records.Service, cache *Cache, fwd *Forwarder, met *metrics.Collector) *Handler {
	return &Handler{
		records:   recSvc,
		cache:     cache,
		forwarder: fwd,
		metrics:   met,
	}
}

// ServeDNS implements dns.Handler.
func (h *Handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	start := time.Now()

	if len(r.Question) == 0 {
		dns.HandleFailed(w, r)
		return
	}

	q := r.Question[0]
	clientIP, _, _ := net.SplitHostPort(w.RemoteAddr().String())
	if clientIP == "" {
		clientIP = w.RemoteAddr().String()
	}

	qTypeStr := dns.TypeToString[q.Qtype]
	if qTypeStr == "" {
		qTypeStr = "UNKNOWN"
	}

	// 1. Check custom local records
	var recType records.RecordType
	switch q.Qtype {
	case dns.TypeA:
		recType = records.TypeA
	case dns.TypeAAAA:
		recType = records.TypeAAAA
	}

	if recType != "" && h.records != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		localRecs, err := h.records.Find(ctx, q.Name, recType)
		cancel()

		if err == nil && len(localRecs) > 0 {
			resp := new(dns.Msg)
			resp.SetReply(r)
			resp.Authoritative = true
			resp.RecursionAvailable = true

			for _, rec := range localRecs {
				rr := buildRR(q.Name, rec)
				if rr != nil {
					resp.Answer = append(resp.Answer, rr)
				}
			}

			w.WriteMsg(resp)
			h.recordMetric(clientIP, q.Name, qTypeStr, "local", dns.RcodeToString[resp.Rcode], time.Since(start))
			return
		}
	}

	// 2. Check local response cache
	if h.cache != nil {
		if cachedMsg, found := h.cache.Get(q.Name, q.Qtype, q.Qclass); found {
			cachedMsg.SetReply(r)
			cachedMsg.Id = r.Id
			cachedMsg.Compress = true
			w.WriteMsg(cachedMsg)
			h.recordMetric(clientIP, q.Name, qTypeStr, "cache", dns.RcodeToString[cachedMsg.Rcode], time.Since(start))
			return
		}
	}

	// 3. Forward to upstream DNS servers
	if h.forwarder != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		upstreamResp, err := h.forwarder.Forward(ctx, r)
		cancel()

		if err == nil && upstreamResp != nil {
			upstreamResp.Id = r.Id
			w.WriteMsg(upstreamResp)

			// Store in cache if enabled
			if h.cache != nil {
				h.cache.Set(upstreamResp)
			}

			h.recordMetric(clientIP, q.Name, qTypeStr, "upstream", dns.RcodeToString[upstreamResp.Rcode], time.Since(start))
			return
		}
	}

	// 4. Return SERVFAIL if resolution could not be completed
	failMsg := new(dns.Msg)
	failMsg.SetRcode(r, dns.RcodeServerFailure)
	w.WriteMsg(failMsg)
	h.recordMetric(clientIP, q.Name, qTypeStr, "error", "SERVFAIL", time.Since(start))
}

func buildRR(qName string, rec *records.Record) dns.RR {
	hdr := dns.RR_Header{
		Name:   dns.Fqdn(qName),
		Class:  dns.ClassINET,
		Ttl:    rec.TTL,
	}

	switch rec.Type {
	case records.TypeA:
		hdr.Rrtype = dns.TypeA
		ip := net.ParseIP(rec.Value).To4()
		if ip == nil {
			return nil
		}
		return &dns.A{
			Hdr: hdr,
			A:   ip,
		}
	case records.TypeAAAA:
		hdr.Rrtype = dns.TypeAAAA
		ip := net.ParseIP(rec.Value)
		if ip == nil {
			return nil
		}
		return &dns.AAAA{
			Hdr:  hdr,
			AAAA: ip,
		}
	default:
		return nil
	}
}

func (h *Handler) recordMetric(clientIP, domain, qtype, source, rcode string, dur time.Duration) {
	if h.metrics != nil {
		normDomain := strings.TrimSuffix(domain, ".")
		h.metrics.RecordQuery(metrics.QueryLogEntry{
			Timestamp: time.Now(),
			ClientIP:  clientIP,
			Domain:    normDomain,
			QType:     qtype,
			Source:    source,
			RCode:     rcode,
			Duration:  dur,
		})
	}
}
