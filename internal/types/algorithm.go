package types

import "context"

type IntersectionResult interface{}

type IntersectionAlgorithm interface {
	// Compute streams both datasets in parallel and returns intersection statistics
	Compute(ctx context.Context, datasets []KeyIterator) (IntersectionResult, error)
}

type PairwiseDatasetStats struct {
	Source        string
	TotalCount    uint64
	DistinctCount uint64
}

type PairwiseResult struct {
	Datasets        []PairwiseDatasetStats
	ConnectorStats  []ConnectorStats
	DistinctOverlap uint64
	TotalOverlap    uint64
	ErrorBoundPct   float64
}
