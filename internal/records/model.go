package records

import (
	"strings"
	"time"
)

// RecordType defines supported DNS record types.
type RecordType string

const (
	TypeA    RecordType = "A"
	TypeAAAA RecordType = "AAAA"
)

// Record represents a custom DNS record stored in ResolverX.
type Record struct {
	ID        string     `json:"id"`
	Domain    string     `json:"domain"`
	Type      RecordType `json:"type"`
	Value     string     `json:"value"`
	TTL       uint32     `json:"ttl"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// CanonicalDomain returns the domain in lowercase with a trailing dot for DNS wire matching.
func (r *Record) CanonicalDomain() string {
	d := strings.ToLower(strings.TrimSpace(r.Domain))
	if !strings.HasSuffix(d, ".") {
		return d + "."
	}
	return d
}

// DisplayDomain returns the domain without a trailing dot for UI display.
func (r *Record) DisplayDomain() string {
	d := strings.ToLower(strings.TrimSpace(r.Domain))
	return strings.TrimSuffix(d, ".")
}
