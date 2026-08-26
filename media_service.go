package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
)

type MediaService struct {
	pipeline *AssetPipeline
}

func (s *MediaService) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /assets", s.beginUpload)
	mux.HandleFunc("POST /assets/{id}/uploaded", s.confirmUpload)
	mux.HandleFunc("POST /assets/{id}/jobs/complete", s.completeJob)
	mux.HandleFunc("GET /assets/{id}/delivery", s.delivery)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return requestLog(mux)
}

type beginUploadRequest struct {
	ID          string `json:"id"`
	CreatorID   string `json:"creator_id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	MaxBytes    int64  `json:"max_bytes"`
}

func (s *MediaService) beginUpload(w http.ResponseWriter, r *http.Request) {
	var input beginUploadRequest
	if err := decodeJSON(r, &input); err != nil || input.ID == "" || input.CreatorID == "" || input.Filename == "" || input.ContentType == "" || input.MaxBytes <= 0 || strings.Contains(input.Filename, "/") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid upload intent"})
		return
	}
	intent, err := s.pipeline.BeginUpload(r.Context(), input.ID, input.CreatorID, input.Filename, input.ContentType, input.MaxBytes)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, intent)
}

func (s *MediaService) confirmUpload(w http.ResponseWriter, r *http.Request) {
	asset, err := s.pipeline.ConfirmUpload(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

type completeJobRequest struct {
	JobID string `json:"job_id"`
}

func (s *MediaService) completeJob(w http.ResponseWriter, r *http.Request) {
	var input completeJobRequest
	if err := decodeJSON(r, &input); err != nil || input.JobID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid processing result"})
		return
	}
	asset, err := s.pipeline.CompleteJob(r.PathValue("id"), input.JobID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

func (s *MediaService) delivery(w http.ResponseWriter, r *http.Request) {
	creatorID := r.URL.Query().Get("creator_id")
	if creatorID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "creator_id is required"})
		return
	}
	signed, err := s.pipeline.CreatorDelivery(r.Context(), r.PathValue("id"), creatorID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"download_url": signed.URL, "method": "GET"})
}

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func writeServiceError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrAssetNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrObjectNotFound), errors.Is(err, ErrInvalidTransition):
		status = http.StatusConflict
	default:
		var apiErr *InfraiError
		if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
			status = apiErr.HTTPStatus
		}
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("event=response_encode_error error=%q", err)
	}
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("event=request method=%s path=%s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
