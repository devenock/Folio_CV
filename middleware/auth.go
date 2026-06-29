package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"foliocv/models"
)

type contextKey string

const UserContextKey contextKey = "user"

func RequireAuth(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session_id")
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			ctx := r.Context()

			session, err := models.FindSession(ctx, pool, cookie.Value)
			if err != nil || session == nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			if session.ExpiresAt.Before(time.Now()) {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			user, err := models.FindUserByID(ctx, pool, session.UserID)
			if err != nil || user == nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			ctx = context.WithValue(ctx, UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserFromContext(ctx context.Context) *models.User {
	user, ok := ctx.Value(UserContextKey).(*models.User)
	if !ok {
		return nil
	}
	return user
}
