package redirects

import (
	"strings"
	"time"
)

// Redirect represents a configured URL redirect or shortcut mapping.
type Redirect struct {
	ID          string    `json:"id"`
	Shortcut    string    `json:"shortcut"`    // e.g. "gitrush.test" or "gitrush"
	TargetURL   string    `json:"target_url"`  // e.g. "https://github.com/RushikeshIche"
	StatusCode  int       `json:"status_code"` // 301, 302, 307, 308
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CleanShortcut normalizes the shortcut by trimming whitespace, lowercasing, and stripping leading/trailing slashes.
func CleanShortcut(s string) string {
	sc := strings.TrimSpace(strings.ToLower(s))
	sc = strings.TrimPrefix(sc, "http://")
	sc = strings.TrimPrefix(sc, "https://")
	sc = strings.Trim(sc, "/")
	return sc
}
