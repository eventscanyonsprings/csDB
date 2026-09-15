package main

import (
	"context"
	"log"
	"net/http"

	"canyon-springs-residents/store"
)

type ctxKey string

const userKey ctxKey = "user"
const adminKey ctxKey = "admin"

const sessionCookie = "cs_session"

func getUser(r *http.Request) string {
	if v, ok := r.Context().Value(userKey).(string); ok {
		return v
	}
	return ""
}

func isAdmin(r *http.Request) bool {
	if v, ok := r.Context().Value(adminKey).(bool); ok {
		return v
	}
	return false
}

func userCount(app *App) int {
	c, _ := store.UserCount(context.Background(), app.db)
	return c
}

func loadUser(app *App) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("loadUser START: path=%s", r.URL.Path)
			cookie, err := r.Cookie(sessionCookie)
			if err == nil && cookie.Value != "" {
				log.Printf("loadUser: session cookie present path=%s", r.URL.Path)
				username, ok, _ := store.FindSession(r.Context(), app.db, cookie.Value)
				log.Printf("loadUser: username=%q ok=%v", username, ok)
				if ok && username != "" {
					exists, _ := store.UserExists(r.Context(), app.db, username)
					if exists {
						admin, _ := store.IsAdmin(r.Context(), app.db, username)
						ctx := context.WithValue(r.Context(), userKey, username)
						ctx = context.WithValue(ctx, adminKey, admin)
						r = r.WithContext(ctx)
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requireAuth(app *App) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := userCount(app)
			user := getUser(r)
			log.Printf("requireAuth: path=%s user=%q count=%d", r.URL.Path, user, count)
			if count == 0 {
				next.ServeHTTP(w, r)
				return
			}
			if user == "" {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requireAdmin(app *App) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := userCount(app)
			if count == 0 {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			if getUser(r) == "" {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			if !isAdmin(r) {
				http.Error(w, "admin access required", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
