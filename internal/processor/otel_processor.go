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

// Chain chains multiple processors together
type Chain struct {
	processors []Processor
}

// NewChain creates a new processor chain
func NewChain(processors ...Processor) *Chain {
	return &Chain{
		processors: processors,
	}
}

// Process processes a batch through all processors in the chain
func (c *Chain) Process(ctx context.Context, batch domain.MetricBatch) error {
	for _, processor := range c.processors {
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

	go bp.backgroundFlusher()

	return bp
}

// Process adds metrics to the batch and flushes if necessary
func (p *BatchProcessor) Process(ctx context.Context, batch domain.MetricBatch) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.currentBatch.Metrics = append(p.currentBatch.Metrics, batch.Metrics...)

	for k, v := range batch.ResourceAttrs {
		p.currentBatch.ResourceAttrs[k] = v
	}

	if p.currentBatch.ReceivedAt.IsZero() {
		p.currentBatch.ReceivedAt = batch.ReceivedAt
	}

	if p.flushTimer != nil {
		p.flushTimer.Stop()
	}

	p.flushTimer = time.AfterFunc(p.batchTimeout, func() {
		select {
		case p.flushChan <- struct{}{}:
		default:
		}
	})

	if len(p.currentBatch.Metrics) >= p.batchSize {
		return p.flushLocked()
	}

	return nil
}

// flushLocked flushes the current batch (caller must hold lock)
func (p *BatchProcessor) flushLocked() error {
	if len(p.currentBatch.Metrics) == 0 {
		return nil
	}

	slog.Debug("flushing metric batch",
		"count", len(p.currentBatch.Metrics))

	if err := p.converter.Convert(p.currentBatch); err != nil {
		slog.Error("failed to convert batch", "error", err)
		// Don't return error - just log it and continue
	}

	p.currentBatch = domain.MetricBatch{
		Metrics:       make([]domain.Metric, 0, p.batchSize),
		ResourceAttrs: make(map[string]string),
	}

	return nil
}

// Flush flushes the current batch
func (p *BatchProcessor) Flush() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.flushLocked()
}

// backgroundFlusher periodically flushes batches
func (p *BatchProcessor) backgroundFlusher() {
	for {
		select {
		case <-p.flushChan:
			p.mu.Lock()
			p.flushLocked()
			p.mu.Unlock()
		case <-p.stopChan:
			return
		}
	}
}

// Stop stops the batch processor
func (p *BatchProcessor) Stop() {
	close(p.stopChan)
	p.Flush()
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
