package redirects

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ValidateRedirect validates and normalizes all fields of a Redirect.
func ValidateRedirect(r *Redirect) error {
	if r == nil {
		return errors.New("redirect cannot be nil")
	}

	r.Shortcut = CleanShortcut(r.Shortcut)
	if r.Shortcut == "" {
		return errors.New("shortcut cannot be empty")
	}
	if len(r.Shortcut) > 255 {
		return errors.New("shortcut exceeds maximum length of 255 characters")
	}

	target := strings.TrimSpace(r.TargetURL)
	if target == "" {
		return errors.New("target URL cannot be empty")
	}

	// Auto-prefix https:// if user omitted scheme
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "https://" + target
	}
	r.TargetURL = target

	parsed, err := url.ParseRequestURI(r.TargetURL)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("invalid target URL format '%s'", r.TargetURL)
	}

	if r.StatusCode == 0 {
		r.StatusCode = 302
	} else {
		switch r.StatusCode {
		case 301, 302, 307, 308:
		default:
			return fmt.Errorf("invalid HTTP redirect status code %d (must be 301, 302, 307, or 308)", r.StatusCode)
		}
	}

	r.Description = strings.TrimSpace(r.Description)
	return nil
}
