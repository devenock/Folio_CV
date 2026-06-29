package handlers

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"foliocv/middleware"
	"foliocv/models"
)

var allowedThemes = map[string]bool{"professional": true, "creative": true, "minimal": true}

type ThemeHandler struct {
	db *pgxpool.Pool
}

func NewThemeHandler(pool *pgxpool.Pool) *ThemeHandler {
	return &ThemeHandler{db: pool}
}

func (h *ThemeHandler) Switch(w http.ResponseWriter, r *http.Request) {
	theme := r.FormValue("theme")
	if !allowedThemes[theme] {
		http.Error(w, "invalid theme", http.StatusBadRequest)
		return
	}

	user := middleware.UserFromContext(r.Context())
	portfolio, err := models.FindPortfolioByUserID(r.Context(), h.db, user.ID)
	if err != nil || portfolio == nil {
		http.NotFound(w, r)
		return
	}

	if err := models.UpdateTheme(r.Context(), h.db, portfolio.ID, theme); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	portfolio.Theme = theme

	w.Header().Set("HX-Trigger", `{"themeChanged":"`+theme+`"}`)

	tmpl, err := parseTemplates("templates/editor/theme_picker.html")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	tmpl.ExecuteTemplate(w, "theme_picker", portfolio)
}
