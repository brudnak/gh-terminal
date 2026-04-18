package handler

import (
	"net/http"

	"github.com/brudnak/gh-terminal/internal/ticker"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	ticker.ServeHTTP(w, r, ticker.LightTheme)
}
