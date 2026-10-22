package records

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
)

var (
	// labelRegex validates standard RFC 1123 hostname labels:
	// 1-63 chars, alphanumeric start/end, internal hyphens allowed.
	labelRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)
)

const (
	DefaultTTL uint32 = 300
	MinTTL     uint32 = 1
	MaxTTL     uint32 = 604800 // 7 days in seconds
)

// ValidateRecord validates all fields of a Record and normalizes the domain.
func ValidateRecord(r *Record) error {
	if r == nil {
		return errors.New("record cannot be nil")
	}

	normDomain, err := ValidateAndNormalizeDomain(r.Domain)
	if err != nil {
		return err
	}
	r.Domain = normDomain

	if err := ValidateRecordType(r.Type); err != nil {
		return err
	}

	if err := ValidateRecordValue(r.Type, r.Value); err != nil {
		return err
	}

	if r.TTL == 0 {
		r.TTL = DefaultTTL
	} else if r.TTL < MinTTL || r.TTL > MaxTTL {
		return fmt.Errorf("TTL must be between %d and %d seconds", MinTTL, MaxTTL)
	}

	return nil
}

// ValidateAndNormalizeDomain checks RFC domain rules and returns the normalized lowercase domain without trailing dot.
func ValidateAndNormalizeDomain(domain string) (string, error) {
	d := strings.TrimSpace(domain)
	if d == "" {
		return "", errors.New("domain name cannot be empty")
	}

	// Remove trailing dot for uniform validation and UI display
	d = strings.TrimSuffix(d, ".")
	d = strings.ToLower(d)

	if len(d) > 253 {
		return "", errors.New("domain name exceeds maximum length of 253 characters")
	}

	labels := strings.Split(d, ".")
	if len(labels) == 0 {
		return "", errors.New("domain name must contain at least one label")
	}

	for _, label := range labels {
		if len(label) == 0 {
			return "", errors.New("domain name cannot contain empty labels")
		}
		if len(label) > 63 {
			return "", fmt.Errorf("domain label '%s' exceeds 63 characters", label)
		}
		if !labelRegex.MatchString(label) {
			return "", fmt.Errorf("invalid label '%s': must contain only alphanumeric characters and hyphens, and cannot start or end with a hyphen", label)
		}
	}

	return d, nil
}

// ValidateRecordType ensures the record type is supported (A or AAAA).
func ValidateRecordType(t RecordType) error {
	switch t {
	case TypeA, TypeAAAA:
		return nil
	default:
		return fmt.Errorf("unsupported record type '%s'; only 'A' and 'AAAA' are supported", t)
	}
}

// ValidateRecordValue checks IP validity for the corresponding record type.
func ValidateRecordValue(t RecordType, value string) error {
	val := strings.TrimSpace(value)
	if val == "" {
		return errors.New("record value cannot be empty")
	}

	ip := net.ParseIP(val)
	if ip == nil {
		return fmt.Errorf("invalid IP address format: '%s'", val)
	}

	switch t {
	case TypeA:
		if ip.To4() == nil {
			return fmt.Errorf("value '%s' is not a valid IPv4 address for type A", val)
		}
	case TypeAAAA:
		if ip.To4() != nil {
			return fmt.Errorf("value '%s' is an IPv4 address, but type AAAA requires IPv6", val)
		}
	}

	return nil
}
