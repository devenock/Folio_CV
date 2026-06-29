package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"foliocv/middleware"
	"foliocv/models"
	"foliocv/services"
)

const defaultMaxUploadBytes = 10 * 1024 * 1024

var allowedCVMimeTypes = map[string]bool{
	"application/pdf": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
}

type UploadHandler struct {
	db     *pgxpool.Pool
	claude *services.ClaudeClient
}

func NewUploadHandler(pool *pgxpool.Pool, claude *services.ClaudeClient) *UploadHandler {
	return &UploadHandler{db: pool, claude: claude}
}

func maxUploadBytes() int64 {
	v := os.Getenv("MAX_UPLOAD_BYTES")
	if v == "" {
		return defaultMaxUploadBytes
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return defaultMaxUploadBytes
	}
	return n
}

func uploadsDir() string {
	dir := os.Getenv("UPLOADS_DIR")
	if dir == "" {
		dir = "./data/uploads"
	}
	return dir
}

func renderUploadTemplate(w http.ResponseWriter, page string, data any) {
	tmpl, err := parseTemplates("templates/base.html", page)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, "template render error", http.StatusInternalServerError)
	}
}

func (h *UploadHandler) Page(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	renderUploadTemplate(w, "templates/upload/index.html", map[string]any{"User": user})
}

func (h *UploadHandler) Handle(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	maxBytes := maxUploadBytes()
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+1<<20) // small cushion for form overhead

	if err := r.ParseMultipartForm(maxBytes); err != nil {
		renderUploadTemplate(w, "templates/upload/index.html", map[string]any{
			"User":  user,
			"Error": "File is too large or the upload is malformed.",
		})
		return
	}

	file, header, err := r.FormFile("cv")
	if err != nil {
		renderUploadTemplate(w, "templates/upload/index.html", map[string]any{
			"User":  user,
			"Error": "Please choose a CV file to upload.",
		})
		return
	}
	defer file.Close()

	if header.Size > maxBytes {
		renderUploadTemplate(w, "templates/upload/index.html", map[string]any{
			"User":  user,
			"Error": "File exceeds the maximum upload size.",
		})
		return
	}

	mimeType := header.Header.Get("Content-Type")
	if !allowedCVMimeTypes[mimeType] {
		renderUploadTemplate(w, "templates/upload/index.html", map[string]any{
			"User":  user,
			"Error": "Unsupported file type. Please upload a PDF or DOCX file.",
		})
		return
	}

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		renderUploadTemplate(w, "templates/upload/index.html", map[string]any{
			"User":  user,
			"Error": "Could not read the uploaded file. Please try again.",
		})
		return
	}

	if err := os.MkdirAll(uploadsDir(), 0755); err != nil {
		log.Printf("uploads dir error: %v", err)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	savedFilename := user.ID + "_" + filepath.Base(header.Filename)
	savedPath := filepath.Join(uploadsDir(), savedFilename)
	if err := os.WriteFile(savedPath, fileBytes, 0644); err != nil {
		log.Printf("save upload error: %v", err)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	// One portfolio per user (MVP). Reuse the existing portfolio row if present,
	// resetting it to a processing state; otherwise create a fresh skeleton.
	ctx := r.Context()
	existing, err := models.FindPortfolioByUserID(ctx, h.db, user.ID)
	if err != nil {
		log.Printf("find portfolio error: %v", err)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	isReupload := existing != nil

	var portfolioID string
	if existing != nil {
		portfolioID = existing.ID
		if err := models.MarkPortfolioReprocessing(ctx, h.db, portfolioID, header.Filename); err != nil {
			log.Printf("mark reprocessing error: %v", err)
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
	} else {
		portfolioID = uuid.New().String()
		skeletonSlug := "processing-" + portfolioID[:8]
		if err := models.CreateSkeletonPortfolio(ctx, h.db, portfolioID, user.ID, skeletonSlug, "Processing...", header.Filename); err != nil {
			log.Printf("create skeleton portfolio error: %v", err)
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
	}

	go func() {
		bgCtx := context.Background()
		resume, err := h.claude.ParseCV(bgCtx, fileBytes, mimeType)
		if err != nil {
			log.Printf("Claude parse failed: %v", err)
			if err := models.MarkPortfolioParseFailed(bgCtx, h.db, portfolioID); err != nil {
				log.Printf("mark parse failed error: %v", err)
			}
			return
		}

		if isReupload {
			err = populatePortfolioMerge(bgCtx, h.db, portfolioID, *resume)
		} else {
			err = populatePortfolio(bgCtx, h.db, portfolioID, *resume)
		}
		if err != nil {
			log.Printf("populate portfolio failed: %v", err)
			if err := models.MarkPortfolioParseFailed(bgCtx, h.db, portfolioID); err != nil {
				log.Printf("mark parse failed error: %v", err)
			}
			return
		}

		rawJSON, _ := json.Marshal(resume)
		finalSlug := generateSlug(resume.FullName)
		if isReupload {
			finalSlug = existing.Slug
		}

		if err := models.CompletePortfolioParse(bgCtx, h.db, portfolioID,
			resume.FullName, resume.Headline, resume.Summary,
			resume.Email, resume.Phone, resume.Location,
			resume.LinkedInURL, resume.GithubURL, resume.WebsiteURL,
			resume.CareerType, resume.SuggestedTheme, finalSlug, rawJSON,
		); err != nil {
			log.Printf("complete portfolio parse error: %v", err)
		}
	}()

	http.Redirect(w, r, "/upload/processing", http.StatusFound)
}

func (h *UploadHandler) ProcessingPage(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	renderUploadTemplate(w, "templates/upload/processing.html", map[string]any{"User": user})
}

func (h *UploadHandler) Status(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	portfolio, err := models.FindPortfolioByUserID(r.Context(), h.db, user.ID)
	if err != nil || portfolio == nil {
		w.Write([]byte(`<div id="status-container" class="text-sm text-gray-400">No upload in progress. <a href="/upload" class="text-brand2 hover:underline">Upload a CV</a></div>`))
		return
	}

	if portfolio.FullName == "Parse failed" {
		w.Write([]byte(`<div id="status-container" class="text-sm animate-fade">
			<span class="text-danger">Parsing failed.</span>
			<a href="/upload" class="text-brand2 hover:underline">Try again</a>
		</div>`))
		return
	}

	if portfolio.CVParsedAt != nil {
		w.Header().Set("HX-Redirect", "/dashboard")
		w.Write([]byte(`<div id="status-container" class="text-sm text-success animate-fade">Done! Redirecting…</div>`))
		return
	}

	stage := "Reading your document…"
	switch {
	case time.Since(portfolio.UpdatedAt) > 12*time.Second:
		stage = "Organizing sections…"
	case time.Since(portfolio.UpdatedAt) > 5*time.Second:
		stage = "Extracting your experience, skills, and education…"
	}

	w.Write([]byte(`<div id="status-container" hx-get="/upload/status" hx-trigger="every 1500ms" hx-swap="outerHTML">
		<span class="text-sm text-gray-400">` + stage + `</span>
	</div>`))
}

// sectionData holds one CV section's type/title plus the items extracted
// from the parsed resume, ready to insert under any section ID.
type sectionData struct {
	sectionType string
	title       string
	items       []models.SectionItem
}

// resumeSections maps a parsed resume into the section/item shape stored in
// Postgres. Shared by populatePortfolio (fresh upload) and
// populatePortfolioMerge (re-upload), which differ only in how they decide
// whether to create a new section or append to an existing one.
func resumeSections(resume services.ResumeData) []sectionData {
	var sections []sectionData

	if len(resume.Experience) > 0 {
		items := make([]models.SectionItem, len(resume.Experience))
		for i, e := range resume.Experience {
			meta, _ := json.Marshal(map[string]any{"bullets": e.Bullets})
			items[i] = models.SectionItem{
				Title: strPtr(e.Title), Subtitle: strPtr(e.Company),
				Location: e.Location, StartDate: e.StartDate, EndDate: e.EndDate,
				Description: e.Description, URL: e.URL, Meta: meta,
			}
		}
		sections = append(sections, sectionData{"experience", "Experience", items})
	}

	if len(resume.Education) > 0 {
		items := make([]models.SectionItem, len(resume.Education))
		for i, e := range resume.Education {
			meta, _ := json.Marshal(map[string]any{"grade": e.Grade})
			items[i] = models.SectionItem{
				Title: strPtr(e.Degree), Subtitle: strPtr(e.Institution),
				Location: e.Location, StartDate: e.StartDate, EndDate: e.EndDate,
				Description: e.Description, Meta: meta,
			}
		}
		sections = append(sections, sectionData{"education", "Education", items})
	}

	if len(resume.Skills) > 0 {
		items := make([]models.SectionItem, len(resume.Skills))
		for i, s := range resume.Skills {
			meta, _ := json.Marshal(map[string]any{"items": s.Items})
			items[i] = models.SectionItem{Title: strPtr(s.Category), Meta: meta}
		}
		sections = append(sections, sectionData{"skills", "Skills", items})
	}

	if len(resume.Projects) > 0 {
		items := make([]models.SectionItem, len(resume.Projects))
		for i, p := range resume.Projects {
			meta, _ := json.Marshal(map[string]any{"technologies": p.Technologies, "bullets": p.Bullets})
			items[i] = models.SectionItem{
				Title: strPtr(p.Name), Description: strPtr(p.Description),
				URL: p.URL, StartDate: p.StartDate, EndDate: p.EndDate, Meta: meta,
			}
		}
		sections = append(sections, sectionData{"projects", "Projects", items})
	}

	if len(resume.Certifications) > 0 {
		items := make([]models.SectionItem, len(resume.Certifications))
		for i, c := range resume.Certifications {
			meta, _ := json.Marshal(map[string]any{"credential_id": c.CredentialID})
			items[i] = models.SectionItem{
				Title: strPtr(c.Name), Subtitle: c.Issuer,
				StartDate: c.Date, URL: c.URL, Meta: meta,
			}
		}
		sections = append(sections, sectionData{"certifications", "Certifications", items})
	}

	if len(resume.Awards) > 0 {
		items := make([]models.SectionItem, len(resume.Awards))
		for i, a := range resume.Awards {
			items[i] = models.SectionItem{
				Title: strPtr(a.Title), Subtitle: a.Issuer,
				StartDate: a.Date, Description: a.Description,
			}
		}
		sections = append(sections, sectionData{"awards", "Awards", items})
	}

	if len(resume.Publications) > 0 {
		items := make([]models.SectionItem, len(resume.Publications))
		for i, p := range resume.Publications {
			items[i] = models.SectionItem{
				Title: strPtr(p.Title), Subtitle: p.Publisher,
				StartDate: p.Date, URL: p.URL, Description: p.Description,
			}
		}
		sections = append(sections, sectionData{"publications", "Publications", items})
	}

	if len(resume.Volunteer) > 0 {
		items := make([]models.SectionItem, len(resume.Volunteer))
		for i, v := range resume.Volunteer {
			items[i] = models.SectionItem{
				Title: strPtr(v.Role), Subtitle: strPtr(v.Organization),
				StartDate: v.StartDate, EndDate: v.EndDate, Description: v.Description,
			}
		}
		sections = append(sections, sectionData{"volunteer", "Volunteer", items})
	}

	if len(resume.Languages) > 0 {
		items := make([]models.SectionItem, len(resume.Languages))
		for i, l := range resume.Languages {
			items[i] = models.SectionItem{Title: strPtr(l.Language), Subtitle: strPtr(l.Proficiency)}
		}
		sections = append(sections, sectionData{"languages", "Languages", items})
	}

	if len(resume.Interests) > 0 {
		items := make([]models.SectionItem, len(resume.Interests))
		for i, interest := range resume.Interests {
			items[i] = models.SectionItem{Title: strPtr(interest)}
		}
		sections = append(sections, sectionData{"interests", "Interests", items})
	}

	return sections
}

// sortSectionsForCareerType orders sections per DefaultSectionOrder, with
// unlisted types pushed to the end in their original order.
func sortSectionsForCareerType(sections []sectionData, careerType string) []sectionData {
	order := services.DefaultSectionOrder[careerType]
	if order == nil {
		order = services.DefaultSectionOrder["general"]
	}
	priority := make(map[string]int, len(order))
	for i, t := range order {
		priority[t] = i
	}

	sorted := make([]sectionData, len(sections))
	copy(sorted, sections)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			pi, oki := priority[sorted[i].sectionType]
			pj, okj := priority[sorted[j].sectionType]
			if !oki {
				pi = len(order) + 1
			}
			if !okj {
				pj = len(order) + 1
			}
			if pj < pi {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}

func insertItems(ctx context.Context, pool *pgxpool.Pool, sectionID string, items []models.SectionItem, startOrder int) error {
	for i, item := range items {
		item.SortOrder = startOrder + i
		if _, err := models.CreateItem(ctx, pool, sectionID, item); err != nil {
			return err
		}
	}
	return nil
}

// dedupeAgainstExisting drops any new item that already exists in a section
// (matched on title+subtitle+start_date+end_date+description), so
// re-uploading the same or overlapping CV doesn't pile up duplicate entries
// on every parse.
func dedupeAgainstExisting(existing []models.SectionItem, newItems []models.SectionItem) []models.SectionItem {
	deref := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	key := func(it models.SectionItem) string {
		return deref(it.Title) + "|" + deref(it.Subtitle) + "|" + deref(it.StartDate) + "|" + deref(it.EndDate) + "|" + deref(it.Description)
	}

	seen := make(map[string]bool, len(existing))
	for _, it := range existing {
		seen[key(it)] = true
	}

	out := make([]models.SectionItem, 0, len(newItems))
	for _, it := range newItems {
		k := key(it)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, it)
	}
	return out
}

// populatePortfolio creates all sections and items from the parsed resume data,
// ordered according to DefaultSectionOrder for the detected career type.
// Used for a brand-new portfolio (first upload).
func populatePortfolio(ctx context.Context, pool *pgxpool.Pool, portfolioID string, resume services.ResumeData) error {
	sorted := sortSectionsForCareerType(resumeSections(resume), resume.CareerType)

	sortOrder := 0
	for _, sd := range sorted {
		section, err := models.CreateSection(ctx, pool, portfolioID, sd.sectionType, sd.title, sortOrder)
		if err != nil {
			return err
		}
		if err := insertItems(ctx, pool, section.ID, sd.items, 0); err != nil {
			return err
		}
		sortOrder++
	}

	for i, cs := range resume.CustomSections {
		section, err := models.CreateSection(ctx, pool, portfolioID, "custom", cs.Title, sortOrder+i)
		if err != nil {
			return err
		}
		for j, item := range cs.Items {
			if _, err := models.CreateItem(ctx, pool, section.ID, models.SectionItem{
				SortOrder: j, Title: item.Title, Description: item.Description,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

// populatePortfolioMerge folds a re-uploaded CV's data into an existing
// portfolio: matching section types get their new items appended after the
// existing ones, while section types not already present get created fresh.
// Nothing is deleted, so manual edits made through the dashboard survive a
// re-upload.
func populatePortfolioMerge(ctx context.Context, pool *pgxpool.Pool, portfolioID string, resume services.ResumeData) error {
	sorted := sortSectionsForCareerType(resumeSections(resume), resume.CareerType)

	for _, sd := range sorted {
		existing, err := models.FindSectionByPortfolioAndType(ctx, pool, portfolioID, sd.sectionType)
		if err != nil {
			return err
		}

		if existing != nil {
			existingItems, err := models.ListItemsBySection(ctx, pool, existing.ID)
			if err != nil {
				return err
			}
			newItems := dedupeAgainstExisting(existingItems, sd.items)
			if err := insertItems(ctx, pool, existing.ID, newItems, len(existingItems)); err != nil {
				return err
			}
			continue
		}

		nextOrder, err := models.NextSectionSortOrder(ctx, pool, portfolioID)
		if err != nil {
			return err
		}
		section, err := models.CreateSection(ctx, pool, portfolioID, sd.sectionType, sd.title, nextOrder)
		if err != nil {
			return err
		}
		if err := insertItems(ctx, pool, section.ID, sd.items, 0); err != nil {
			return err
		}
	}

	for _, cs := range resume.CustomSections {
		existing, err := models.FindCustomSectionByTitle(ctx, pool, portfolioID, cs.Title)
		if err != nil {
			return err
		}

		var sectionID string
		var existingItems []models.SectionItem
		if existing != nil {
			sectionID = existing.ID
			existingItems, err = models.ListItemsBySection(ctx, pool, existing.ID)
			if err != nil {
				return err
			}
		} else {
			nextOrder, err := models.NextSectionSortOrder(ctx, pool, portfolioID)
			if err != nil {
				return err
			}
			section, err := models.CreateSection(ctx, pool, portfolioID, "custom", cs.Title, nextOrder)
			if err != nil {
				return err
			}
			sectionID = section.ID
		}

		newItems := make([]models.SectionItem, len(cs.Items))
		for i, item := range cs.Items {
			newItems[i] = models.SectionItem{Title: item.Title, Description: item.Description}
		}
		newItems = dedupeAgainstExisting(existingItems, newItems)

		if err := insertItems(ctx, pool, sectionID, newItems, len(existingItems)); err != nil {
			return err
		}
	}

	return nil
}

func strPtr(s string) *string {
	return &s
}

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

func generateSlug(fullName string) string {
	b := make([]byte, 4)
	rand.Read(b)
	suffix := hex.EncodeToString(b)[:6]
	return slugify(fullName) + "-" + suffix
}
