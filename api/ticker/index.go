package handler

import (
	"net/http"

	"github.com/brudnak/gh-terminal/pkg/ticker"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	ticker.ServeHTTP(w, r, ticker.DarkTheme)
}
