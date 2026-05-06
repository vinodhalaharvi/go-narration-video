package web

import (
	"context"
	"fmt"
	"net/http"
)

// ============================================================
// THE STDLIB PATTERN — closure implements interface
// ============================================================

// In net/http, this is the canonical functional seam:
//
//   type Handler interface { ServeHTTP(ResponseWriter, *Request) }
//   type HandlerFunc func(ResponseWriter, *Request)
//   func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) { f(w, r) }
//
// A function value satisfies the Handler interface by calling itself.
// We follow this lead. Every "thing" in our framework is a function type.

// Handler is just an alias to drop the .HandlerFunc noise.
type Handler = http.HandlerFunc

// ============================================================
// HANDLERS — plain functions
// ============================================================

var Hello Handler = func(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "hello")
}

type ctxKey string

const userKey ctxKey = "user"

var WhoAmI Handler = func(w http.ResponseWriter, r *http.Request) {
	user, _ := r.Context().Value(userKey).(string)
	fmt.Fprintln(w, user)
}

// ============================================================
// MIDDLEWARE — a function from Handler to Handler
// ============================================================

type Middleware = func(Handler) Handler

// RequireAuth is one middleware. It checks a Bearer token, sets the user
// on context, and rejects unauthorized requests. No interface, no struct.
func RequireAuth(next Handler) Handler {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token != "Bearer secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userKey, "alice")
		next(w, r.WithContext(ctx))
	}
}

// ============================================================
// CHAIN — combinator, composes middleware right-to-left
// ============================================================

func Chain(mws ...Middleware) Middleware {
	return func(h Handler) Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			h = mws[i](h)
		}
		return h
	}
}

// ============================================================
// DEMO — wire it up
// ============================================================

func DemoServer() {
	auth := Chain(RequireAuth)

	http.Handle("/hello", Hello)
	http.Handle("/me", auth(WhoAmI))

	http.ListenAndServe(":8080", nil)
}
