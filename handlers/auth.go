package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"foliocv/models"
)

const sessionCookieName = "session_id"
const sessionDuration = 30 * 24 * time.Hour

type AuthHandler struct {
	db *pgxpool.Pool
}

func NewAuthHandler(pool *pgxpool.Pool) *AuthHandler {
	return &AuthHandler{db: pool}
}

func renderAuthTemplate(w http.ResponseWriter, page string, data any) {
	tmpl, err := parseTemplates("templates/base.html", page)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, "template render error", http.StatusInternalServerError)
	}
}

func setSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	renderAuthTemplate(w, "templates/auth/register.html", nil)
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	email := r.FormValue("email")
	name := r.FormValue("name")
	password := r.FormValue("password")

	if email == "" || name == "" || password == "" {
		renderAuthTemplate(w, "templates/auth/register.html", map[string]any{
			"Error": "All fields are required.",
			"Email": email,
			"Name":  name,
		})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		renderAuthTemplate(w, "templates/auth/register.html", map[string]any{
			"Error": "Could not create account. Please try again.",
			"Email": email,
			"Name":  name,
		})
		return
	}

	user, err := models.CreateUser(ctx, h.db, email, name, string(hash))
	if err != nil {
		renderAuthTemplate(w, "templates/auth/register.html", map[string]any{
			"Error": "An account with that email may already exist.",
			"Email": email,
			"Name":  name,
		})
		return
	}

	sessionID := uuid.New().String()
	expiresAt := time.Now().Add(sessionDuration)
	if _, err := models.CreateSession(ctx, h.db, sessionID, user.ID, expiresAt); err != nil {
		http.Error(w, "could not create session", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, sessionID)
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	renderAuthTemplate(w, "templates/auth/login.html", nil)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	email := r.FormValue("email")
	password := r.FormValue("password")

	invalidCreds := func() {
		renderAuthTemplate(w, "templates/auth/login.html", map[string]any{
			"Error": "Invalid email or password.",
			"Email": email,
		})
	}

	user, err := models.FindUserByEmail(ctx, h.db, email)
	if err != nil || user == nil {
		invalidCreds()
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		invalidCreds()
		return
	}

	sessionID := uuid.New().String()
	expiresAt := time.Now().Add(sessionDuration)
	if _, err := models.CreateSession(ctx, h.db, sessionID, user.ID, expiresAt); err != nil {
		http.Error(w, "could not create session", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, sessionID)
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		_ = models.DeleteSession(ctx, h.db, cookie.Value)
	}

	clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusFound)
}
