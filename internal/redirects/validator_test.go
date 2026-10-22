package redirects_test

import (
	"testing"

	"github.com/RushikeshIche/ResolverX/internal/redirects"
)

func TestValidateRedirect(t *testing.T) {
	tests := []struct {
		name        string
		redirect    *redirects.Redirect
		expectErr   bool
		expectedURL string
		expectedSC  string
	}{
		{
			name: "Valid shortcut with full URL",
			redirect: &redirects.Redirect{
				Shortcut:  "gitrush.test",
				TargetURL: "https://github.com/RushikeshIche",
			},
			expectErr:   false,
			expectedURL: "https://github.com/RushikeshIche",
			expectedSC:  "gitrush.test",
		},
		{
			name: "Auto-prefix https scheme",
			redirect: &redirects.Redirect{
				Shortcut:  "gitrush.com",
				TargetURL: "github.com/RushikeshIche",
			},
			expectErr:   false,
			expectedURL: "https://github.com/RushikeshIche",
			expectedSC:  "gitrush.com",
		},
		{
			name: "Clean shortcut with slashes",
			redirect: &redirects.Redirect{
				Shortcut:  "/myrepo/",
				TargetURL: "https://github.com/RushikeshIche/ResolverX",
			},
			expectErr:   false,
			expectedURL: "https://github.com/RushikeshIche/ResolverX",
			expectedSC:  "myrepo",
		},
		{
			name: "Empty shortcut error",
			redirect: &redirects.Redirect{
				Shortcut:  "   ",
				TargetURL: "https://google.com",
			},
			expectErr: true,
		},
		{
			name: "Empty target URL error",
			redirect: &redirects.Redirect{
				Shortcut:  "test",
				TargetURL: "   ",
			},
			expectErr: true,
		},
		{
			name: "Invalid status code error",
			redirect: &redirects.Redirect{
				Shortcut:   "test",
				TargetURL:  "https://example.com",
				StatusCode: 500,
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := redirects.ValidateRedirect(tt.redirect)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error for %+v, got nil", tt.redirect)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if tt.redirect.TargetURL != tt.expectedURL {
					t.Errorf("expected TargetURL '%s', got '%s'", tt.expectedURL, tt.redirect.TargetURL)
				}
				if tt.redirect.Shortcut != tt.expectedSC {
					t.Errorf("expected Shortcut '%s', got '%s'", tt.expectedSC, tt.redirect.Shortcut)
				}
				if tt.redirect.StatusCode != 302 {
					t.Errorf("expected default status code 302, got %d", tt.redirect.StatusCode)
				}
			}
		})
	}
}
