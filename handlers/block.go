package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"foliocv/middleware"
	"foliocv/models"
)

// renderBlockView executes the full block_view.html partial (header bar +
// content) — used by Create/Toggle, which replace or insert the whole card.
func renderBlockView(w io.Writer, view BlockView) error {
	tmpl, err := parseTemplates("templates/builder/block_view.html")
	if err != nil {
		return err
	}
	return tmpl.ExecuteTemplate(w, "block_view", view)
}

// renderBlockBody executes only the inner block_body partial — used by
// View/UpdateContent, whose targets are the body div *inside* an
// already-rendered wrapper (swapping the full block_view there would nest
// a duplicate header bar and id).
func renderBlockBody(w io.Writer, view BlockView) error {
	tmpl, err := parseTemplates("templates/builder/block_view.html")
	if err != nil {
		return err
	}
	return tmpl.ExecuteTemplate(w, "block_body", view)
}

// allBlockTypes drives the "+ Add block" palette and Create validation —
// mirrors the block_type Postgres enum (migrations/000006_create_blocks.up.sql).
var allBlockTypes = []string{
	"hero", "heading", "text", "image", "gallery", "project_card",
	"experience_list", "skill_list", "education_list",
	"contact", "social_links", "divider", "embed", "button", "spacer",
}

var validBlockTypes = func() map[string]bool {
	m := make(map[string]bool, len(allBlockTypes))
	for _, t := range allBlockTypes {
		m[t] = true
	}
	return m
}()

// defaultBlockSize gives new blocks a sensible starting footprint by type.
func defaultBlockSize(blockType string) (w, h int) {
	switch blockType {
	case "divider", "spacer":
		return 12, 1
	case "button":
		return 4, 2
	case "image":
		return 6, 4
	case "hero":
		return 12, 3
	default:
		return 12, 4
	}
}

type BlockHandler struct {
	db *pgxpool.Pool
}

func NewBlockHandler(pool *pgxpool.Pool) *BlockHandler {
	return &BlockHandler{db: pool}
}

// ownedBlock verifies that blockID belongs to the authenticated user's
// portfolio, returning the block if so. It writes a 404 and returns nil
// if the block doesn't exist or isn't owned by the requester — same
// ownership-check pattern as SectionHandler.ownedSection / ItemHandler.ownedSection.
func (h *BlockHandler) ownedBlock(w http.ResponseWriter, r *http.Request, blockID string) *models.Block {
	user := middleware.UserFromContext(r.Context())
	portfolio, err := models.FindPortfolioByUserID(r.Context(), h.db, user.ID)
	if err != nil || portfolio == nil {
		http.NotFound(w, r)
		return nil
	}

	block, err := models.FindBlockByID(r.Context(), h.db, blockID)
	if err != nil || block == nil || block.PortfolioID != portfolio.ID {
		http.NotFound(w, r)
		return nil
	}
	return block
}

// Canvas renders the page builder: every block server-rendered into a
// GridStack-recognized grid-stack-item, so initial content (including
// htmx attributes) is real HTML, not client-built strings. Hidden route,
// not linked from nav yet, per the rollout plan.
func (h *BlockHandler) Canvas(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	ctx := r.Context()

	portfolio, err := models.FindPortfolioByUserID(ctx, h.db, user.ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if portfolio == nil {
		http.Redirect(w, r, "/upload", http.StatusFound)
		return
	}

	blocks, err := models.ListBlocksByPortfolio(ctx, h.db, portfolio.ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	views := make([]BlockView, len(blocks))
	for i, b := range blocks {
		views[i] = buildBlockView(b)
	}

	tmpl, err := parseTemplates("templates/base.html", "templates/builder/canvas.html", "templates/builder/block_view.html")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "base", map[string]any{
		"User":       user,
		"Blocks":     views,
		"BlockTypes": allBlockTypes,
	}); err != nil {
		http.Error(w, "template render error", http.StatusInternalServerError)
	}
}

// Create adds a new block at a default position/size below the current
// content and returns its rendered HTML as JSON so the client can register
// it with GridStack via grid.addWidget (plain fetch, not htmx — GridStack's
// own widget registry needs to learn about the new node directly).
func (h *BlockHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	ctx := r.Context()

	portfolio, err := models.FindPortfolioByUserID(ctx, h.db, user.ID)
	if err != nil || portfolio == nil {
		http.NotFound(w, r)
		return
	}

	blockType := r.FormValue("type")
	if !validBlockTypes[blockType] {
		http.Error(w, "invalid block type", http.StatusBadRequest)
		return
	}

	existing, err := models.ListBlocksByPortfolio(ctx, h.db, portfolio.ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	nextY := 0
	for _, b := range existing {
		if b.GridY+b.GridH > nextY {
			nextY = b.GridY + b.GridH
		}
	}

	w_, h_ := defaultBlockSize(blockType)
	block, err := models.CreateBlock(ctx, h.db, portfolio.ID, blockType, 0, nextY, w_, h_, nil)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := renderBlockView(&buf, buildBlockView(*block)); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id": block.ID, "grid_x": block.GridX, "grid_y": block.GridY,
		"grid_w": block.GridW, "grid_h": block.GridH, "html": buf.String(),
	})
}

// View re-renders a block read-only — used by the edit form's Cancel button.
func (h *BlockHandler) View(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	block := h.ownedBlock(w, r, id)
	if block == nil {
		return
	}
	if err := renderBlockBody(w, buildBlockView(*block)); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// EditForm returns the per-type content form, swapped into the block's body.
func (h *BlockHandler) EditForm(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	block := h.ownedBlock(w, r, id)
	if block == nil {
		return
	}

	tmpl, err := parseTemplates("templates/builder/block_form.html")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "block_form", buildBlockView(*block)); err != nil {
		http.Error(w, "template render error", http.StatusInternalServerError)
	}
}

// UpdatePosition persists grid geometry from a drag/resize-end event.
// Called frequently with a tiny payload, so it's kept separate from
// content updates (see models.UpdateBlockPosition).
func (h *BlockHandler) UpdatePosition(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	block := h.ownedBlock(w, r, id)
	if block == nil {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	gridX := formInt(r, "grid_x", block.GridX)
	gridY := formInt(r, "grid_y", block.GridY)
	gridW := formInt(r, "grid_w", block.GridW)
	gridH := formInt(r, "grid_h", block.GridH)
	zIndex := formInt(r, "z_index", block.ZIndex)

	if err := models.UpdateBlockPosition(r.Context(), h.db, id, block.PortfolioID, gridX, gridY, gridW, gridH, zIndex); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// UpdateContent saves a block's per-type fields, parsed from the submitted
// form (mirrors itemFromForm in handlers/item.go), and returns the
// re-rendered read-only view to swap back into place.
func (h *BlockHandler) UpdateContent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	block := h.ownedBlock(w, r, id)
	if block == nil {
		return
	}

	content := blockContentFromForm(r, block.Type)
	if err := models.UpdateBlockContent(r.Context(), h.db, id, block.PortfolioID, content); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	block.Content = content

	if err := renderBlockBody(w, buildBlockView(*block)); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (h *BlockHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	block := h.ownedBlock(w, r, id)
	if block == nil {
		return
	}

	if err := models.ToggleBlockVisibility(r.Context(), h.db, id, block.PortfolioID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	updated, err := models.FindBlockByID(r.Context(), h.db, id)
	if err != nil || updated == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if err := renderBlockView(w, buildBlockView(*updated)); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (h *BlockHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	block := h.ownedBlock(w, r, id)
	if block == nil {
		return
	}

	if err := models.DeleteBlock(r.Context(), h.db, id, block.PortfolioID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func formInt(r *http.Request, name string, fallback int) int {
	v := r.FormValue(name)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// BlockListItem is one row of a list-type block (skill_list/experience_list/
// education_list), embedded directly in the block's content JSON array
// rather than as separate foreign-keyed rows (per the canvas builder plan).
type BlockListItem struct {
	Title       string
	Subtitle    string
	StartDate   string
	EndDate     string
	Description string
}

// BlockView flattens a Block's JSONB content into plain fields a template
// can range over directly, plus pre-joined strings for form prefill — same
// flattening convention as buildPublicItemView in handlers/portfolio.go.
type BlockView struct {
	ID        string
	Type      string
	GridX     int
	GridY     int
	GridW     int
	GridH     int
	ZIndex    int
	IsVisible bool

	Text     string // heading.text / text.body
	Label    string // button.label
	URL      string // button/embed/project_card.url
	Height   int    // spacer.height
	Email    string
	Phone    string
	Location string
	LinkedIn string
	Github   string
	Website  string
	Alt      string // image.alt
	ImageURL string // image.url
	Headline string // hero.headline

	Title           string // project_card.title
	Body            string // project_card description (separate from Text)
	Technologies    []string
	TechnologiesCSV string

	Items      []BlockListItem
	ItemsCSV   string // skill_list prefill
	ItemsLines string // experience_list/education_list prefill
}

func buildBlockView(b models.Block) BlockView {
	view := BlockView{
		ID: b.ID, Type: b.Type, GridX: b.GridX, GridY: b.GridY, GridW: b.GridW, GridH: b.GridH,
		ZIndex: b.ZIndex, IsVisible: b.IsVisible, Height: 40,
	}
	var content map[string]any
	if len(b.Content) > 0 {
		_ = json.Unmarshal(b.Content, &content)
	}
	str := func(key string) string {
		s, _ := content[key].(string)
		return s
	}
	strList := func(key string) []string {
		list, _ := content[key].([]any)
		out := make([]string, 0, len(list))
		for _, v := range list {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}

	switch b.Type {
	case "hero":
		view.Text = str("name")
		view.Headline = str("headline")
	case "heading":
		view.Text = str("text")
	case "text":
		view.Text = str("body")
	case "button":
		view.Label = str("label")
		view.URL = str("url")
	case "spacer":
		if h, ok := content["height"].(float64); ok && h > 0 {
			view.Height = int(h)
		}
	case "contact":
		view.Email = str("email")
		view.Phone = str("phone")
		view.Location = str("location")
	case "social_links":
		view.LinkedIn = str("linkedin")
		view.Github = str("github")
		view.Website = str("website")
	case "embed":
		view.URL = str("url")
	case "image":
		view.Alt = str("alt")
		view.ImageURL = str("url")
	case "skill_list":
		items := strList("items")
		for _, s := range items {
			view.Items = append(view.Items, BlockListItem{Title: s})
		}
		view.ItemsCSV = strings.Join(items, ", ")
	case "experience_list", "education_list":
		rawItems, _ := content["items"].([]any)
		var lines []string
		for _, raw := range rawItems {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			item := BlockListItem{
				Title: asStr(m["title"]), Subtitle: asStr(m["subtitle"]),
				StartDate: asStr(m["start_date"]), EndDate: asStr(m["end_date"]),
				Description: asStr(m["description"]),
			}
			view.Items = append(view.Items, item)
			lines = append(lines, strings.Join([]string{item.Title, item.Subtitle, item.StartDate, item.EndDate, item.Description}, " | "))
		}
		view.ItemsLines = strings.Join(lines, "\n")
	case "project_card":
		view.Title = str("title")
		view.Body = str("body")
		view.URL = str("url")
		view.Technologies = strList("technologies")
		view.TechnologiesCSV = strings.Join(view.Technologies, ", ")
	}

	return view
}

func asStr(v any) string {
	s, _ := v.(string)
	return s
}

// blockContentFromForm maps submitted form fields into a block's content
// JSON, per type — mirrors itemFromForm's per-section-type dispatch.
func blockContentFromForm(r *http.Request, blockType string) json.RawMessage {
	r.ParseForm()
	content := map[string]any{}

	switch blockType {
	case "hero":
		content["name"] = r.FormValue("name")
		content["headline"] = r.FormValue("headline")
	case "heading":
		content["text"] = r.FormValue("text")
	case "text":
		content["body"] = r.FormValue("body")
	case "button":
		content["label"] = r.FormValue("label")
		content["url"] = r.FormValue("url")
	case "divider":
		// no content
	case "spacer":
		h, err := strconv.Atoi(r.FormValue("height"))
		if err != nil || h <= 0 {
			h = 40
		}
		content["height"] = h
	case "contact":
		content["email"] = r.FormValue("email")
		content["phone"] = r.FormValue("phone")
		content["location"] = r.FormValue("location")
	case "social_links":
		content["linkedin"] = r.FormValue("linkedin")
		content["github"] = r.FormValue("github")
		content["website"] = r.FormValue("website")
	case "embed":
		content["url"] = r.FormValue("url")
	case "image":
		content["alt"] = r.FormValue("alt")
		content["url"] = r.FormValue("url")
	case "gallery":
		// image upload lands in a later phase; nothing to persist yet
	case "skill_list":
		content["items"] = parseCSV(r.FormValue("items_csv"))
	case "experience_list", "education_list":
		content["items"] = parseListLines(r.FormValue("items_lines"))
	case "project_card":
		content["title"] = r.FormValue("title")
		content["body"] = r.FormValue("body")
		content["url"] = r.FormValue("url")
		content["technologies"] = parseCSV(r.FormValue("technologies_csv"))
	}

	b, _ := json.Marshal(content)
	return b
}

// parseListLines parses "Title | Subtitle | Start | End | Description"
// formatted lines into the structured item shape experience_list/
// education_list blocks store in their content JSON.
func parseListLines(s string) []map[string]string {
	var out []map[string]string
	for _, line := range parseLines(s) {
		parts := strings.Split(line, "|")
		get := func(i int) string {
			if i < len(parts) {
				return strings.TrimSpace(parts[i])
			}
			return ""
		}
		out = append(out, map[string]string{
			"title": get(0), "subtitle": get(1),
			"start_date": get(2), "end_date": get(3), "description": get(4),
		})
	}
	return out
}
