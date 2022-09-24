package main

import "net/http"

// session middleware
func SessionLoad(next http.Handler) http.Handler {
	return session.LoadAndSave(next)
}
