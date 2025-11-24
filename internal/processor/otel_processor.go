package processor

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/converter"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/domain"
)

// Processor defines the interface for metric processing
type Processor interface {
	Process(ctx context.Context, batch domain.MetricBatch) error
}

// ProcessorChain chains multiple processors together
type ProcessorChain struct {
	processors []Processor
}

// NewProcessorChain creates a new processor chain
func NewProcessorChain(processors ...Processor) *ProcessorChain {
	return &ProcessorChain{
		processors: processors,
	}
}

// Process processes a batch through all processors in the chain
func (pc *ProcessorChain) Process(ctx context.Context, batch domain.MetricBatch) error {
	for _, processor := range pc.processors {
		if err := processor.Process(ctx, batch); err != nil {
			return err
		}
	}
	return nil
}

// BatchProcessor accumulates metrics and processes them in batches
type BatchProcessor struct {
	converter    *converter.Converter
	batchSize    int
	batchTimeout time.Duration
	currentBatch domain.MetricBatch
	mu           sync.Mutex
	flushTimer   *time.Timer
	stopChan     chan struct{}
	flushChan    chan struct{}
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(conv *converter.Converter, batchSize int, batchTimeout time.Duration) *BatchProcessor {
	bp := &BatchProcessor{
		converter:    conv,
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
		currentBatch: domain.MetricBatch{
			Metrics:       make([]domain.Metric, 0, batchSize),
			ResourceAttrs: make(map[string]string),
		},
		stopChan:  make(chan struct{}),
		flushChan: make(chan struct{}, 1),
	}

	// Start background flusher
	go bp.backgroundFlusher()

	return bp
}

// Process adds metrics to the batch and flushes if necessary
func (bp *BatchProcessor) Process(ctx context.Context, batch domain.MetricBatch) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	// Add metrics to current batch
	bp.currentBatch.Metrics = append(bp.currentBatch.Metrics, batch.Metrics...)

	// Merge resource attributes
	for k, v := range batch.ResourceAttrs {
		bp.currentBatch.ResourceAttrs[k] = v
	}

	// Update received time if not set
	if bp.currentBatch.ReceivedAt.IsZero() {
		bp.currentBatch.ReceivedAt = batch.ReceivedAt
	}

	// Reset flush timer
	if bp.flushTimer != nil {
		bp.flushTimer.Stop()
	}
	bp.flushTimer = time.AfterFunc(bp.batchTimeout, func() {
		select {
		case bp.flushChan <- struct{}{}:
		default:
		}
	})

	// Check if batch is full
	if len(bp.currentBatch.Metrics) >= bp.batchSize {
		return bp.flushLocked()
	}

	return nil
}

// flushLocked flushes the current batch (caller must hold lock)
func (bp *BatchProcessor) flushLocked() error {
	if len(bp.currentBatch.Metrics) == 0 {
		return nil
	}

	slog.Debug("flushing metric batch",
		"count", len(bp.currentBatch.Metrics))

	// Convert batch to Prometheus format
	if err := bp.converter.Convert(bp.currentBatch); err != nil {
		slog.Error("failed to convert batch", "error", err)
		// Don't return error - just log it and continue
	}

	// Reset batch
	bp.currentBatch = domain.MetricBatch{
		Metrics:       make([]domain.Metric, 0, bp.batchSize),
		ResourceAttrs: make(map[string]string),
	}

	return nil
}

// Flush flushes the current batch
func (bp *BatchProcessor) Flush() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.flushLocked()
}

// backgroundFlusher periodically flushes batches
func (bp *BatchProcessor) backgroundFlusher() {
	for {
		select {
		case <-bp.flushChan:
			bp.mu.Lock()
			bp.flushLocked()
			bp.mu.Unlock()
		case <-bp.stopChan:
			return
		}
	}
}

// Stop stops the batch processor
func (bp *BatchProcessor) Stop() {
	close(bp.stopChan)
	bp.Flush()
}

// DirectProcessor processes metrics immediately without batching
type DirectProcessor struct {
	converter *converter.Converter
}

// NewDirectProcessor creates a new direct processor
func NewDirectProcessor(conv *converter.Converter) *DirectProcessor {
	return &DirectProcessor{
		converter: conv,
	}
}

// Process processes metrics directly
func (dp *DirectProcessor) Process(ctx context.Context, batch domain.MetricBatch) error {
	return dp.converter.Convert(batch)
}
