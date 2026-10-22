// Package hostsfile provides helpers to add and remove entries from the
// operating system hosts file (e.g. C:\Windows\System32\drivers\etc\hosts).
// Each managed entry is flanked by ResolverX sentinel comments so they can be
// cleanly identified and removed later.
package hostsfile

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
)

const (
	sentinel = "# ResolverX managed"
	loopback = "127.0.0.1"
)

var mu sync.Mutex

// HostsPath returns the path to the OS hosts file.
func HostsPath() string {
	if runtime.GOOS == "windows" {
		return `C:\Windows\System32\drivers\etc\hosts`
	}
	return "/etc/hosts"
}

// EnsureEntry adds a "127.0.0.1 <domain>" entry to the hosts file if it is
// not already present. It is a no-op if the domain already has any entry.
func EnsureEntry(domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return nil
	}

	mu.Lock()
	defer mu.Unlock()

	path := HostsPath()
	lines, err := readLines(path)
	if err != nil {
		return fmt.Errorf("hostsfile: cannot read %s: %w", path, err)
	}

	// Check if any existing line already maps this domain.
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue // lone IP or malformed line — skip safely
		}
		for _, f := range fields[1:] {
			if strings.EqualFold(f, domain) {
				return nil // already present
			}
		}
	}

	// Append the new managed entry.
	entry := fmt.Sprintf("%s\t%s\t\t%s\n", loopback, domain, sentinel)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("hostsfile: cannot open %s for writing (try running as Administrator): %w", path, err)
	}
	defer f.Close()

	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("hostsfile: cannot write entry: %w", err)
	}
	return nil
}

// RemoveEntry removes any ResolverX-managed hosts file entry for the given domain.
func RemoveEntry(domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return nil
	}

	mu.Lock()
	defer mu.Unlock()

	path := HostsPath()
	lines, err := readLines(path)
	if err != nil {
		return fmt.Errorf("hostsfile: cannot read %s: %w", path, err)
	}

	var kept []string
	for _, l := range lines {
		// Drop lines that are our managed entry for this domain.
		if strings.Contains(l, sentinel) {
			fields := strings.Fields(l)
			if len(fields) >= 2 && strings.EqualFold(fields[1], domain) {
				continue // skip / remove this line
			}
		}
		kept = append(kept, l)
	}

	return writeLines(path, kept)
}

// RemoveAllManaged removes every ResolverX-managed entry from the hosts file.
func RemoveAllManaged() error {
	mu.Lock()
	defer mu.Unlock()

	path := HostsPath()
	lines, err := readLines(path)
	if err != nil {
		return fmt.Errorf("hostsfile: cannot read %s: %w", path, err)
	}

	var kept []string
	for _, l := range lines {
		if strings.Contains(l, sentinel) {
			continue
		}
		kept = append(kept, l)
	}

	return writeLines(path, kept)
}

// IsDomainShortcut returns true if the shortcut looks like a domain name
// (contains a dot and no slashes), meaning it needs a hosts file entry.
func IsDomainShortcut(shortcut string) bool {
	return strings.Contains(shortcut, ".") && !strings.Contains(shortcut, "/")
}

// ---- helpers ----------------------------------------------------------------

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func writeLines(path string, lines []string) error {
	content := strings.Join(lines, "\n")
	// Preserve trailing newline if original had one.
	if len(lines) > 0 {
		content += "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("hostsfile: cannot write %s (try running as Administrator): %w", path, err)
	}
	return nil
}
