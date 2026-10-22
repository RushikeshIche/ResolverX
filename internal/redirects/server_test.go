package redirects_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/RushikeshIche/ResolverX/internal/redirects"
	"github.com/RushikeshIche/ResolverX/internal/storage"
)

func TestHTTPRedirectServer(t *testing.T) {
	store, err := storage.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}
	defer store.Close()

	svc := redirects.NewService(store, nil)

	// Pick a free random loopback port
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate free port: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()

	server := redirects.NewServer(addr, svc)
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start redirect server: %v", err)
	}
	defer server.Stop()

	time.Sleep(50 * time.Millisecond)

	ctx := context.Background()

	// 1. Create a redirect for gitrush.test -> https://github.com/RushikeshIche
	redir := &redirects.Redirect{
		Shortcut:    "gitrush.test",
		TargetURL:   "https://github.com/RushikeshIche",
		StatusCode:  302,
		Description: "GitHub Profile",
	}
	if err := svc.Create(ctx, redir); err != nil {
		t.Fatalf("failed to create redirect: %v", err)
	}

	httpClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirect, capture 302
		},
		Timeout: 2 * time.Second,
	}

	// 2. Test matching by Host header (http://gitrush.test -> target)
	reqByHost, err := http.NewRequest("GET", "http://"+addr+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	reqByHost.Host = "gitrush.test:8080" // Simulate browser Host header

	respHost, err := httpClient.Do(reqByHost)
	if err != nil {
		t.Fatalf("request by host failed: %v", err)
	}
	defer respHost.Body.Close()

	if respHost.StatusCode != 302 {
		t.Errorf("expected status 302, got %d", respHost.StatusCode)
	}
	loc := respHost.Header.Get("Location")
	if loc != "https://github.com/RushikeshIche" {
		t.Errorf("expected location 'https://github.com/RushikeshIche', got '%s'", loc)
	}

	// 3. Test matching by Path (http://127.0.0.1:8080/gitrush.test)
	reqByPath, err := http.NewRequest("GET", "http://"+addr+"/gitrush.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	respPath, err := httpClient.Do(reqByPath)
	if err != nil {
		t.Fatalf("request by path failed: %v", err)
	}
	defer respPath.Body.Close()

	if respPath.StatusCode != 302 {
		t.Errorf("expected status 302, got %d", respPath.StatusCode)
	}
	if respPath.Header.Get("Location") != "https://github.com/RushikeshIche" {
		t.Errorf("expected location 'https://github.com/RushikeshIche', got '%s'", respPath.Header.Get("Location"))
	}

	// 4. Test Modifying Shortcut and TargetURL
	redir.Shortcut = "rush.dev"
	redir.TargetURL = "https://github.com/RushikeshIche/ResolverX"
	if err := svc.Update(ctx, redir); err != nil {
		t.Fatalf("failed to update redirect: %v", err)
	}

	// Old shortcut should now not match
	respOld, err := httpClient.Do(reqByPath)
	if err != nil {
		t.Fatal(err)
	}
	defer respOld.Body.Close()
	if respOld.StatusCode == 302 {
		t.Error("expected old shortcut to no longer redirect after modification")
	}

	// New shortcut should match
	reqNew, _ := http.NewRequest("GET", "http://"+addr+"/rush.dev", nil)
	respNew, err := httpClient.Do(reqNew)
	if err != nil {
		t.Fatal(err)
	}
	defer respNew.Body.Close()
	if respNew.StatusCode != 302 {
		t.Errorf("expected status 302, got %d", respNew.StatusCode)
	}
	if respNew.Header.Get("Location") != "https://github.com/RushikeshIche/ResolverX" {
		t.Errorf("expected location 'https://github.com/RushikeshIche/ResolverX', got '%s'", respNew.Header.Get("Location"))
	}
}
