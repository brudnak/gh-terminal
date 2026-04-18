package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	handler "github.com/brudnak/gh-terminal/api"
)

func main() {
	req := httptest.NewRequest(http.MethodGet, "http://localhost/api/ticker", nil)
	recorder := httptest.NewRecorder()

	handler.Handler(recorder, req)

	resp := recorder.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "unexpected status: %s\n", resp.Status)
		os.Exit(1)
	}

	samplePath := filepath.Join("examples", "sample.svg")
	if err := os.MkdirAll(filepath.Dir(samplePath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create examples dir: %v\n", err)
		os.Exit(1)
	}

	body := recorder.Body.Bytes()
	if err := os.WriteFile(samplePath, body, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write sample svg: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("wrote %s\n", samplePath)
}
