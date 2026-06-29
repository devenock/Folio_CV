package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"foliocv/middleware"
	"foliocv/models"
)

type PortfolioHandler struct {
	db *pgxpool.Pool
}

func NewPortfolioHandler(pool *pgxpool.Pool) *PortfolioHandler {
	return &PortfolioHandler{db: pool}
}

// SectionWithItems pairs a section with its items for dashboard rendering.
type SectionWithItems struct {
	Section models.Section
	Items   []models.SectionItem
}

func renderDashboardTemplate(w http.ResponseWriter, page string, data any) {
	tmpl, err := parseTemplates(
		"templates/base.html", page,
		"templates/editor/section_card.html",
		"templates/editor/theme_picker.html",
	)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, "template render error", http.StatusInternalServerError)
	}
}

func (h *PortfolioHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	ctx := r.Context()

	portfolio, err := models.FindPortfolioByUserID(ctx, h.db, user.ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if portfolio == nil || portfolio.CVParsedAt == nil {
		http.Redirect(w, r, "/upload", http.StatusFound)
		return
	}

	sections, err := models.ListSectionsByPortfolio(ctx, h.db, portfolio.ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	sectionsWithItems := make([]SectionWithItems, 0, len(sections))
	for _, s := range sections {
		items, err := models.ListItemsBySection(ctx, h.db, s.ID)
		if err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		sectionsWithItems = append(sectionsWithItems, SectionWithItems{Section: s, Items: items})
	}

	renderDashboardTemplate(w, "templates/dashboard/index.html", map[string]any{
		"User":      user,
		"Portfolio": portfolio,
		"Sections":  sectionsWithItems,
		"BaseURL":   os.Getenv("BASE_URL"),
	})
}

var portfolioFieldAllowList = map[string]bool{
	"full_name": true, "headline": true, "summary": true, "email": true,
	"phone": true, "location": true, "linkedin_url": true, "github_url": true,
	"website_url": true, "avatar_url": true,
}

// EditField returns an inline <input> for the HTMX click-to-edit pattern.
func (h *PortfolioHandler) EditField(w http.ResponseWriter, r *http.Request) {
	field := chi.URLParam(r, "field")
	if !portfolioFieldAllowList[field] {
		http.NotFound(w, r)
		return
	}

	user := middleware.UserFromContext(r.Context())
	portfolio, err := models.FindPortfolioByUserID(r.Context(), h.db, user.ID)
	if err != nil || portfolio == nil {
		http.NotFound(w, r)
		return
	}

	value := portfolioFieldValue(portfolio, field)
	w.Write([]byte(`<input type="text" name="value" value="` + template.HTMLEscapeString(value) + `" autofocus
		hx-put="/dashboard/profile/` + field + `" hx-trigger="blur, keyup[key=='Enter']"
		hx-target="this" hx-swap="outerHTML"
		class="input-modern rounded-lg px-2.5 py-1 text-gray-100 w-full">`))
}

// UpdateField persists the field value and returns the display <span>.
func (h *PortfolioHandler) UpdateField(w http.ResponseWriter, r *http.Request) {
	field := chi.URLParam(r, "field")
	if !portfolioFieldAllowList[field] {
		http.NotFound(w, r)
		return
	}

	user := middleware.UserFromContext(r.Context())
	portfolio, err := models.FindPortfolioByUserID(r.Context(), h.db, user.ID)
	if err != nil || portfolio == nil {
		http.NotFound(w, r)
		return
	}

	value := r.FormValue("value")
	if err := models.UpdatePortfolioField(r.Context(), h.db, portfolio.ID, field, value); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	w.Write([]byte(`<span hx-get="/dashboard/profile/` + field + `/edit" hx-trigger="click" hx-target="this" hx-swap="outerHTML"
		class="cursor-pointer hover:text-brand2 transition-colors">` + template.HTMLEscapeString(value) + `</span>`))
}

// ToggleVisibility flips a portfolio between public and private.
func (h *PortfolioHandler) ToggleVisibility(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	portfolio, err := models.FindPortfolioByUserID(r.Context(), h.db, user.ID)
	if err != nil || portfolio == nil {
		http.NotFound(w, r)
		return
	}

	isPublic, err := models.ToggleIsPublic(r.Context(), h.db, portfolio.ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	label, action := "Public", "Make private"
	if !isPublic {
		label, action = "Private", "Make public"
	}
	w.Write([]byte(`<button id="visibility-toggle" hx-post="/dashboard/visibility" hx-target="this" hx-swap="outerHTML"
		class="text-xs btn-ghost rounded-lg px-2.5 py-1.5 text-gray-300"
		title="` + action + `">` + label + `</button>`))
}

// PublicItemView flattens a SectionItem's pointer fields and JSONB meta into
// plain values the public theme templates can render directly.
type PublicItemView struct {
	Title        string
	Subtitle     string
	Location     string
	StartDate    string
	EndDate      string
	Description  string
	URL          string
	Bullets      []string
	Tags         []string
	Grade        string
	CredentialID string
}

func buildPublicItemView(item models.SectionItem) PublicItemView {
	deref := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	view := PublicItemView{
		Title:       deref(item.Title),
		Subtitle:    deref(item.Subtitle),
		Location:    deref(item.Location),
		StartDate:   deref(item.StartDate),
		EndDate:     deref(item.EndDate),
		Description: deref(item.Description),
		URL:         deref(item.URL),
	}
	if len(item.Meta) == 0 {
		return view
	}
	var meta map[string]any
	if err := json.Unmarshal(item.Meta, &meta); err != nil {
		return view
	}
	toStrings := func(v any) []string {
		list, _ := v.([]any)
		out := make([]string, 0, len(list))
		for _, el := range list {
			if s, ok := el.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	view.Bullets = toStrings(meta["bullets"])
	if tags := toStrings(meta["items"]); len(tags) > 0 {
		view.Tags = tags
	} else {
		view.Tags = toStrings(meta["technologies"])
	}
	if s, ok := meta["grade"].(string); ok {
		view.Grade = s
	}
	if s, ok := meta["credential_id"].(string); ok {
		view.CredentialID = s
	}
	return view
}

// PublicSectionView pairs a section with its rendered items for the public page.
type PublicSectionView struct {
	Section models.Section
	Items   []PublicItemView
}

// PublicView renders a portfolio's public page at /p/{slug}. No auth
// required; private or nonexistent portfolios both 404 so visitors can't
// distinguish "private" from "doesn't exist".
func (h *PortfolioHandler) PublicView(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	ctx := r.Context()

	portfolio, err := models.FindPortfolioBySlug(ctx, h.db, slug)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if portfolio == nil || !portfolio.IsPublic || portfolio.CVParsedAt == nil {
		renderNotFound(w)
		return
	}

	sections, err := models.ListSectionsByPortfolio(ctx, h.db, portfolio.ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	views := make([]PublicSectionView, 0, len(sections))
	for _, s := range sections {
		if !s.IsVisible {
			continue
		}
		items, err := models.ListItemsBySection(ctx, h.db, s.ID)
		if err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		itemViews := make([]PublicItemView, len(items))
		for i, it := range items {
			itemViews[i] = buildPublicItemView(it)
		}
		views = append(views, PublicSectionView{Section: s, Items: itemViews})
	}

	themeFile := "templates/portfolio/" + portfolio.Theme + ".html"
	tmpl, err := parseTemplates(themeFile)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "portfolio_page", map[string]any{
		"Portfolio": portfolio,
		"Sections":  views,
	}); err != nil {
		http.Error(w, "template render error", http.StatusInternalServerError)
	}
}

func renderNotFound(w http.ResponseWriter) {
	tmpl, err := parseTemplates("templates/base.html", "templates/404.html")
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNotFound)
	tmpl.ExecuteTemplate(w, "base", nil)
}

func portfolioFieldValue(p *models.Portfolio, field string) string {
	deref := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	switch field {
	case "full_name":
		return p.FullName
	case "headline":
		return deref(p.Headline)
	case "summary":
		return deref(p.Summary)
	case "email":
		return deref(p.Email)
	case "phone":
		return deref(p.Phone)
	case "location":
		return deref(p.Location)
	case "linkedin_url":
		return deref(p.LinkedInURL)
	case "github_url":
		return deref(p.GithubURL)
	case "website_url":
		return deref(p.WebsiteURL)
	case "avatar_url":
		return deref(p.AvatarURL)
	}
	return ""
}
