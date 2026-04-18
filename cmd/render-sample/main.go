package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/brudnak/gh-terminal/pkg/ticker"
)

func main() {
	if err := os.MkdirAll("examples", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create examples dir: %v\n", err)
		os.Exit(1)
	}

	if err := writeSample(filepath.Join("examples", "sample.svg"), ticker.DarkTheme); err != nil {
		fmt.Fprintf(os.Stderr, "write dark sample: %v\n", err)
		os.Exit(1)
	}

	if err := writeSample(filepath.Join("examples", "sample-light.svg"), ticker.LightTheme); err != nil {
		fmt.Fprintf(os.Stderr, "write light sample: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("wrote examples/sample.svg")
	fmt.Println("wrote examples/sample-light.svg")
}

func writeSample(path string, theme ticker.Theme) error {
	req, err := http.NewRequest(http.MethodGet, "http://localhost", nil)
	if err != nil {
		return err
	}

	recorder := httptest.NewRecorder()
	ticker.ServeHTTP(recorder, req, theme)

	resp := recorder.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	return os.WriteFile(path, recorder.Body.Bytes(), 0o644)
}
