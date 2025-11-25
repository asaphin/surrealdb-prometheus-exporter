package api

import (
	"context"
	"log/slog"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/processor"
	"github.com/davecgh/go-spew/spew"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"google.golang.org/grpc"
)

// OTELGRPCServer implements the OTLP metrics service over gRPC
type OTELGRPCServer struct {
	pmetricotlp.UnimplementedGRPCServer
	processor processor.Processor
}

// NewOTELGRPCServer creates a new gRPC server for OTLP metrics
func NewOTELGRPCServer(processor processor.Processor) *OTELGRPCServer {
	return &OTELGRPCServer{
		processor: processor,
	}
}

// Export handles the gRPC export request for metrics
func (s *OTELGRPCServer) Export(ctx context.Context, req pmetricotlp.ExportRequest) (pmetricotlp.ExportResponse, error) {
	// Extract metrics from the request
	metrics := req.Metrics()

	slog.Debug("raw OTel metrics", "metrics", spew.Sdump(metrics))

	// Convert to domain model
	batch := ConvertPmetricToDomain(metrics)

	slog.Debug("received OTLP metrics via gRPC",
		"metric_count", batch.Count(),
		"resource_attrs", len(batch.ResourceAttrs))

	// Pass to consumer for processing
	if err := s.processor.Process(ctx, batch); err != nil {
		slog.Error("failed to consume metrics", "error", err)
		return pmetricotlp.NewExportResponse(), err
	}

	// Return success response
	return pmetricotlp.NewExportResponse(), nil
}

// RegisterWith registers this server with a gRPC server
func (s *OTELGRPCServer) RegisterWith(server *grpc.Server) {
	pmetricotlp.RegisterGRPCServer(server, s)
}
