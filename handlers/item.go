package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"foliocv/middleware"
	"foliocv/models"
)

type ItemHandler struct {
	db *pgxpool.Pool
}

func NewItemHandler(pool *pgxpool.Pool) *ItemHandler {
	return &ItemHandler{db: pool}
}

// ownedSection verifies sectionID belongs to the authenticated user's portfolio.
func (h *ItemHandler) ownedSection(w http.ResponseWriter, r *http.Request, sectionID string) *models.Section {
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

func renderItemTemplate(w http.ResponseWriter, name, page string, data any) {
	tmpl, err := parseTemplates(page)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template render error", http.StatusInternalServerError)
	}
}

// ItemRowData pairs a section with one of its items. Used by item_row.html
// and item_form.html so each row/form can access section.Type without
// losing access to the item itself inside a range.
type ItemRowData struct {
	Section models.Section
	Item    models.SectionItem
}

// ItemFormView carries an item's JSONB meta fields pre-extracted into plain
// strings, since templates can't unmarshal json.RawMessage themselves.
type ItemFormView struct {
	Section          models.Section
	Item             models.SectionItem
	BulletsText      string
	ItemsText        string
	TechnologiesText string
	CredentialID     string
	Grade            string
}

func buildItemFormView(section models.Section, item models.SectionItem) ItemFormView {
	view := ItemFormView{Section: section, Item: item}
	if len(item.Meta) == 0 {
		return view
	}
	var meta map[string]any
	if err := json.Unmarshal(item.Meta, &meta); err != nil {
		return view
	}

	asLines := func(v any) string {
		list, _ := v.([]any)
		parts := make([]string, 0, len(list))
		for _, item := range list {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "\n")
	}
	asCSV := func(v any) string {
		list, _ := v.([]any)
		parts := make([]string, 0, len(list))
		for _, item := range list {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	}
	asStr := func(v any) string {
		s, _ := v.(string)
		return s
	}

	view.BulletsText = asLines(meta["bullets"])
	view.ItemsText = asCSV(meta["items"])
	view.TechnologiesText = asCSV(meta["technologies"])
	view.CredentialID = asStr(meta["credential_id"])
	view.Grade = asStr(meta["grade"])
	return view
}

func (h *ItemHandler) NewForm(w http.ResponseWriter, r *http.Request) {
	sectionID := chi.URLParam(r, "sectionID")
	section := h.ownedSection(w, r, sectionID)
	if section == nil {
		return
	}
	renderItemTemplate(w, "item_form", "templates/editor/item_form.html",
		buildItemFormView(*section, models.SectionItem{SectionID: sectionID}))
}

func (h *ItemHandler) EditForm(w http.ResponseWriter, r *http.Request) {
	sectionID := chi.URLParam(r, "sectionID")
	itemID := chi.URLParam(r, "id")
	section := h.ownedSection(w, r, sectionID)
	if section == nil {
		return
	}

	items, err := models.ListItemsBySection(r.Context(), h.db, sectionID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	for _, it := range items {
		if it.ID == itemID {
			renderItemTemplate(w, "item_form", "templates/editor/item_form.html", buildItemFormView(*section, it))
			return
		}
	}
	http.NotFound(w, r)
}

func (h *ItemHandler) Create(w http.ResponseWriter, r *http.Request) {
	sectionID := chi.URLParam(r, "sectionID")
	section := h.ownedSection(w, r, sectionID)
	if section == nil {
		return
	}

	items, err := models.ListItemsBySection(r.Context(), h.db, sectionID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	item := itemFromForm(r, section.Type)
	item.SortOrder = len(items)

	created, err := models.CreateItem(r.Context(), h.db, sectionID, item)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	renderItemTemplate(w, "item_row", "templates/editor/item_row.html", ItemRowData{*section, *created})
}

func (h *ItemHandler) Update(w http.ResponseWriter, r *http.Request) {
	sectionID := chi.URLParam(r, "sectionID")
	itemID := chi.URLParam(r, "id")
	section := h.ownedSection(w, r, sectionID)
	if section == nil {
		return
	}

	item := itemFromForm(r, section.Type)
	item.ID = itemID
	item.SectionID = sectionID

	if err := models.UpdateItem(r.Context(), h.db, item); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	renderItemTemplate(w, "item_row", "templates/editor/item_row.html", ItemRowData{*section, item})
}

func (h *ItemHandler) Delete(w http.ResponseWriter, r *http.Request) {
	sectionID := chi.URLParam(r, "sectionID")
	itemID := chi.URLParam(r, "id")
	section := h.ownedSection(w, r, sectionID)
	if section == nil {
		return
	}

	if err := models.DeleteItem(r.Context(), h.db, itemID, sectionID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ItemHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	sectionID := chi.URLParam(r, "sectionID")
	section := h.ownedSection(w, r, sectionID)
	if section == nil {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ids := r.Form["id"]

	if err := models.UpdateItemOrder(r.Context(), h.db, sectionID, ids); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// itemFromForm maps submitted form fields to a SectionItem, including the
// section-type-specific extras stored in the JSONB meta column.
func itemFromForm(r *http.Request, sectionType string) models.SectionItem {
	r.ParseForm()

	optStr := func(name string) *string {
		v := r.FormValue(name)
		if v == "" {
			return nil
		}
		return &v
	}

	item := models.SectionItem{
		Title:       optStr("title"),
		Subtitle:    optStr("subtitle"),
		Location:    optStr("location"),
		StartDate:   optStr("start_date"),
		EndDate:     optStr("end_date"),
		Description: optStr("description"),
		URL:         optStr("url"),
	}

	meta := map[string]any{}
	switch sectionType {
	case "experience":
		meta["bullets"] = parseLines(r.FormValue("bullets"))
	case "projects":
		meta["technologies"] = parseCSV(r.FormValue("technologies"))
		meta["bullets"] = parseLines(r.FormValue("bullets"))
	case "skills":
		meta["items"] = parseCSV(r.FormValue("items"))
	case "certifications":
		meta["credential_id"] = r.FormValue("credential_id")
	case "education":
		meta["grade"] = r.FormValue("grade")
	}
	if len(meta) > 0 {
		b, _ := json.Marshal(meta)
		item.Meta = b
	}

	return item
}

func parseLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func parseCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
