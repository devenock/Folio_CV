package handlers

import (
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"foliocv/middleware"
	"foliocv/models"
)

type SectionHandler struct {
	db *pgxpool.Pool
}

func NewSectionHandler(pool *pgxpool.Pool) *SectionHandler {
	return &SectionHandler{db: pool}
}

// ownedSection verifies that sectionID belongs to the authenticated user's
// portfolio, returning the section if so. It writes a 404 and returns nil
// if the section doesn't exist or isn't owned by the requester.
func (h *SectionHandler) ownedSection(w http.ResponseWriter, r *http.Request, sectionID string) *models.Section {
	user := middleware.UserFromContext(r.Context())
	portfolio, err := models.FindPortfolioByUserID(r.Context(), h.db, user.ID)
	if err != nil || portfolio == nil {
		http.NotFound(w, r)
		return nil
	}

	section, err := models.FindSectionByID(r.Context(), h.db, sectionID)
	if err != nil || section == nil || section.PortfolioID != portfolio.ID {
		http.NotFound(w, r)
		return nil
	}
	return section
}

func (h *SectionHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	portfolio, err := models.FindPortfolioByUserID(r.Context(), h.db, user.ID)
	if err != nil || portfolio == nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ids := r.Form["id"]

	if err := models.UpdateSectionOrder(r.Context(), h.db, portfolio.ID, ids); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *SectionHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	section := h.ownedSection(w, r, id)
	if section == nil {
		return
	}

	if err := models.ToggleSectionVisibility(r.Context(), h.db, id, section.PortfolioID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	updated, err := models.FindSectionByID(r.Context(), h.db, id)
	if err != nil || updated == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	items, err := models.ListItemsBySection(r.Context(), h.db, id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	renderSectionCard(w, SectionWithItems{Section: *updated, Items: items})
}

func (h *SectionHandler) UpdateTitle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	section := h.ownedSection(w, r, id)
	if section == nil {
		return
	}

	title := r.FormValue("title")
	if title == "" {
		title = section.Title
	}

	if err := models.UpdateSectionTitle(r.Context(), h.db, id, section.PortfolioID, title); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	w.Write([]byte(template.HTMLEscapeString(title)))
}

func (h *SectionHandler) Edit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	section := h.ownedSection(w, r, id)
	if section == nil {
		return
	}

	items, err := models.ListItemsBySection(r.Context(), h.db, id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	rows := make([]ItemRowData, len(items))
	for i, it := range items {
		rows[i] = ItemRowData{Section: *section, Item: it}
	}

	tmpl, err := parseTemplates(
		"templates/editor/section_editor.html",
		"templates/editor/item_row.html",
	)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "section_editor", sectionEditorData{Section: *section, ItemRows: rows}); err != nil {
		http.Error(w, "template render error", http.StatusInternalServerError)
	}
}

type sectionEditorData struct {
	Section  models.Section
	ItemRows []ItemRowData
}

var manualSectionTypes = []string{
	"experience", "education", "skills", "projects", "certifications",
	"awards", "publications", "volunteer", "languages", "interests", "custom",
}

var manualSectionTypeAllowed = func() map[string]bool {
	m := make(map[string]bool, len(manualSectionTypes))
	for _, t := range manualSectionTypes {
		m[t] = true
	}
	return m
}()

// NewPage renders the small inline form for manually adding a section.
func (h *SectionHandler) NewPage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := parseTemplates("templates/editor/section_new_form.html")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	tmpl.ExecuteTemplate(w, "section_new_form", manualSectionTypes)
}

func (h *SectionHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	portfolio, err := models.FindPortfolioByUserID(r.Context(), h.db, user.ID)
	if err != nil || portfolio == nil {
		http.NotFound(w, r)
		return
	}

	sectionType := r.FormValue("type")
	title := r.FormValue("title")
	if !manualSectionTypeAllowed[sectionType] || title == "" {
		http.Error(w, "invalid section", http.StatusBadRequest)
		return
	}

	nextOrder, err := models.NextSectionSortOrder(r.Context(), h.db, portfolio.ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	section, err := models.CreateSection(r.Context(), h.db, portfolio.ID, sectionType, title, nextOrder)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	renderSectionCard(w, SectionWithItems{Section: *section, Items: nil})
}

func (h *SectionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	section := h.ownedSection(w, r, id)
	if section == nil {
		return
	}

	if err := models.DeleteSection(r.Context(), h.db, id, section.PortfolioID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func renderSectionCard(w http.ResponseWriter, data SectionWithItems) {
	tmpl, err := parseTemplates("templates/editor/section_card.html")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "section_card", data); err != nil {
		http.Error(w, "template render error", http.StatusInternalServerError)
	}
}
