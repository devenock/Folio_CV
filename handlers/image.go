package handlers

import (
	"bytes"
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"foliocv/middleware"
	"foliocv/models"
)

const defaultMaxImageBytes = 5 * 1024 * 1024

var allowedImageMimeTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
}

type ImageHandler struct {
	db *pgxpool.Pool
}

func NewImageHandler(pool *pgxpool.Pool) *ImageHandler {
	return &ImageHandler{db: pool}
}

func maxImageBytes() int64 {
	v := os.Getenv("MAX_IMAGE_BYTES")
	if v == "" {
		return defaultMaxImageBytes
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return defaultMaxImageBytes
	}
	return n
}

func imagesDir() string {
	return filepath.Join(uploadsDir(), "images")
}

// ownedImage verifies imageID belongs to the authenticated user's
// portfolio — same ownership-check pattern as ownedSection/ownedBlock.
func (h *ImageHandler) ownedImage(w http.ResponseWriter, r *http.Request, imageID string) *models.Image {
	user := middleware.UserFromContext(r.Context())
	portfolio, err := models.FindPortfolioByUserID(r.Context(), h.db, user.ID)
	if err != nil || portfolio == nil {
		http.NotFound(w, r)
		return nil
	}

	img, err := models.FindImageByID(r.Context(), h.db, imageID)
	if err != nil || img == nil || img.PortfolioID != portfolio.ID {
		http.NotFound(w, r)
		return nil
	}
	return img
}

// Upload accepts a (client-cropped) image file, validates it's a real
// JPEG/PNG, stores it under UPLOADS_DIR/images/{portfolio_id}/, and
// returns its ID + public /media/ URL as JSON.
func (h *ImageHandler) Upload(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	ctx := r.Context()

	portfolio, err := models.FindPortfolioByUserID(ctx, h.db, user.ID)
	if err != nil || portfolio == nil {
		http.NotFound(w, r)
		return
	}

	maxBytes := maxImageBytes()
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+1<<20)

	if err := r.ParseMultipartForm(maxBytes); err != nil {
		http.Error(w, "image is too large or the upload is malformed", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "please choose an image file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if header.Size > maxBytes {
		http.Error(w, "image exceeds the maximum upload size", http.StatusBadRequest)
		return
	}

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "could not read the uploaded image", http.StatusBadRequest)
		return
	}

	mimeType := http.DetectContentType(fileBytes)
	ext, ok := allowedImageMimeTypes[mimeType]
	if !ok {
		http.Error(w, "unsupported image type — please upload a JPEG or PNG", http.StatusBadRequest)
		return
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(fileBytes))
	if err != nil {
		http.Error(w, "could not decode image", http.StatusBadRequest)
		return
	}

	kind := r.FormValue("kind")
	if kind == "" {
		kind = "image"
	}

	imageID := uuid.New().String()
	filename := imageID + ext

	dir := filepath.Join(imagesDir(), portfolio.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, filename), fileBytes, 0644); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	img, err := models.CreateImage(ctx, h.db, imageID, portfolio.ID, filename, kind, cfg.Width, cfg.Height)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":     img.ID,
		"url":    "/media/" + portfolio.ID + "/" + img.Filename,
		"width":  img.Width,
		"height": img.Height,
	})
}

func (h *ImageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	img := h.ownedImage(w, r, id)
	if img == nil {
		return
	}

	if err := models.DeleteImage(r.Context(), h.db, id, img.PortfolioID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	_ = os.Remove(filepath.Join(imagesDir(), img.PortfolioID, img.Filename))

	w.WriteHeader(http.StatusOK)
}
