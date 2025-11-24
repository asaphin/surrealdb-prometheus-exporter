package api

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/processor"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
)

// OTELHTTPHandler handles incoming OTLP metrics via HTTP
type OTELHTTPHandler struct {
	processor processor.Processor
}

// NewOTELHTTPHandler creates a new HTTP handler for OTLP metrics
func NewOTELHTTPHandler(processor processor.Processor) *OTELHTTPHandler {
	return &OTELHTTPHandler{
		processor: processor,
	}
}

// ServeHTTP handles HTTP POST requests with OTLP metrics
func (h *OTELHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("failed to read request body", "error", err)
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse OTLP metrics based on content type
	contentType := r.Header.Get("Content-Type")
	exportRequest, err := h.parseOTLPMetrics(body, contentType)
	if err != nil {
		slog.Error("failed to parse OTLP metrics", "error", err, "content_type", contentType)
		http.Error(w, fmt.Sprintf("failed to parse metrics: %v", err), http.StatusBadRequest)
		return
	}

	// Extract metrics from the export request
	metrics := exportRequest.Metrics()

	// Convert to domain model
	batch := ConvertPmetricToDomain(metrics)

	slog.Debug("received OTLP metrics batch",
		"metric_count", batch.Count(),
		"resource_attrs", len(batch.ResourceAttrs))

	// Pass to processor for processing
	if err := h.processor.Process(r.Context(), batch); err != nil {
		slog.Error("failed to process metrics", "error", err)
		http.Error(w, "failed to process metrics", http.StatusInternalServerError)
		return
	}

	// Return success response
	w.WriteHeader(http.StatusOK)
}

// parseOTLPMetrics parses OTLP metrics from bytes based on content type
func (h *OTELHTTPHandler) parseOTLPMetrics(data []byte, contentType string) (pmetricotlp.ExportRequest, error) {
	req := pmetricotlp.NewExportRequest()

	switch contentType {
	case "application/x-protobuf", "application/octet-stream":
		if err := req.UnmarshalProto(data); err != nil {
			return pmetricotlp.ExportRequest{}, fmt.Errorf("unmarshal protobuf: %w", err)
		}
	case "application/json":
		if err := req.UnmarshalJSON(data); err != nil {
			return pmetricotlp.ExportRequest{}, fmt.Errorf("unmarshal json: %w", err)
		}
	default:
		// Try protobuf by default (most common)
		if err := req.UnmarshalProto(data); err != nil {
			return pmetricotlp.ExportRequest{}, fmt.Errorf("unmarshal protobuf (default): %w", err)
		}
	}

	return req, nil
}
