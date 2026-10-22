package records_test

import (
	"strings"
	"testing"

	"github.com/rushi/ResolverX/internal/records"
)

func TestValidateAndNormalizeDomain(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		expectErr bool
	}{
		{"Standard domain", "myapp.test", "myapp.test", false},
		{"Uppercase normalized", "MyApp.TEST", "myapp.test", false},
		{"Trailing dot stripped", "dev.local.", "dev.local", false},
		{"Single label allowed", "localhost", "localhost", false},
		{"Subdomains with hyphens", "my-service.sub-domain.internal", "my-service.sub-domain.internal", false},
		{"Whitespace trimmed", "   test.com   ", "test.com", false},
		{"Empty domain error", "", "", true},
		{"Spaces only error", "   ", "", true},
		{"Label starting with hyphen", "-invalid.com", "", true},
		{"Label ending with hyphen", "invalid-.com", "", true},
		{"Consecutive dots error", "my..domain.com", "", true},
		{"Special characters error", "my@domain.com", "", true},
		{"Label exceeds 63 chars", strings.Repeat("a", 64) + ".com", "", true},
		{"Total domain exceeds 253 chars", strings.Repeat("a.", 130), "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := records.ValidateAndNormalizeDomain(tt.input)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error for input '%s', got nil (result: '%s')", tt.input, res)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for input '%s': %v", tt.input, err)
				}
				if res != tt.expected {
					t.Errorf("expected '%s', got '%s'", tt.expected, res)
				}
			}
		})
	}
}

func TestValidateRecordValue(t *testing.T) {
	tests := []struct {
		name      string
		recType   records.RecordType
		value     string
		expectErr bool
	}{
		{"Valid IPv4 for A", records.TypeA, "127.0.0.1", false},
		{"Valid IPv4 private", records.TypeA, "192.168.1.100", false},
		{"Invalid IPv4 out of range", records.TypeA, "256.0.0.1", true},
		{"Invalid IPv4 text", records.TypeA, "not-an-ip", true},
		{"IPv6 passed to A record", records.TypeA, "::1", true},

		{"Valid IPv6 loopback", records.TypeAAAA, "::1", false},
		{"Valid IPv6 full", records.TypeAAAA, "2001:0db8:85a3:0000:0000:8a2e:0370:7334", false},
		{"IPv4 passed to AAAA record", records.TypeAAAA, "127.0.0.1", true},
		{"Empty value", records.TypeA, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := records.ValidateRecordValue(tt.recType, tt.value)
			if tt.expectErr && err == nil {
				t.Fatalf("expected error for type %s with value '%s', got nil", tt.recType, tt.value)
			}
			if !tt.expectErr && err != nil {
				t.Fatalf("unexpected error for type %s with value '%s': %v", tt.recType, tt.value, err)
			}
		})
	}
}

func TestValidateRecord(t *testing.T) {
	rec := &records.Record{
		Domain: "MYAPP.TEST.",
		Type:   records.TypeA,
		Value:  "127.0.0.1",
		TTL:    0, // Should default to DefaultTTL (300)
	}

	if err := records.ValidateRecord(rec); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	if rec.Domain != "myapp.test" {
		t.Errorf("expected normalized domain 'myapp.test', got '%s'", rec.Domain)
	}
	if rec.TTL != records.DefaultTTL {
		t.Errorf("expected default TTL %d, got %d", records.DefaultTTL, rec.TTL)
	}

	// Invalid TTL out of bounds
	rec.TTL = 9999999
	if err := records.ValidateRecord(rec); err == nil {
		t.Error("expected error for oversized TTL, got nil")
	}
}
