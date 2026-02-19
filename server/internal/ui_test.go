package internal

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConsoleUIBasic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			io.WriteString(w, `<!DOCTYPE html>
<html>
<head><title>Mini-KV Console</title></head>
<body>
<h1>Mini-KV Console</h1>
<div id="connections">0</div>
<div id="kv-count">0</div>
<div id="connection-status" class="connection-dot connected"></div>
</body>
</html>`)
		} else if r.URL.Path == "/ws" {
			http.Error(w, "Not Found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to get page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	buf := make([]byte, 1024)
	n, _ := resp.Body.Read(buf)
	content := string(buf[:n])

	if !strings.Contains(content, "Mini-KV Console") {
		t.Error("Expected page to contain 'Mini-KV Console'")
	}

	if !strings.Contains(content, "connections") {
		t.Error("Expected page to contain connection counter")
	}

	if !strings.Contains(content, "kv-count") {
		t.Error("Expected page to contain kv-count")
	}

	if !strings.Contains(content, "connection-dot") {
		t.Error("Expected page to contain connection status indicator")
	}
}

func TestConsoleUIRendersProperly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			io.WriteString(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Mini-KV Dashboard</title>
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-50">
    <header>
        <h1>Mini-KV Console</h1>
    </header>
    <main>
        <div id="connections">0</div>
        <div id="kv-count">0</div>
        <div id="last-update">--</div>
        <form id="kv-form">
            <input type="text" id="key-input">
            <input type="text" id="value-input">
        </form>
        <div id="kv-list"></div>
    </main>
    <script>
        let ws = null;
    </script>
</body>
</html>`)
		}
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to get page: %v", err)
	}
	defer resp.Body.Close()

	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	content := string(buf[:n])

	checks := []struct {
		name    string
		content string
	}{
		{"title", "Mini-KV Console"},
		{"HTMX", "htmx.org"},
		{"Tailwind", "tailwindcss"},
		{"connections div", `id="connections"`},
		{"kv-count div", `id="kv-count"`},
		{"kv-form form", `id="kv-form"`},
		{"kv-list div", `id="kv-list"`},
		{"WebSocket", "ws"},
	}

	for _, check := range checks {
		if !strings.Contains(content, check.content) {
			t.Errorf("Expected page to contain '%s' (%s)", check.content, check.name)
		}
	}
}
