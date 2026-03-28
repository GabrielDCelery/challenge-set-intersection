package algorithm

import (
	"context"
	"fmt"
	"strings"

	"github.com/GabrielDCelery/challenge-set-intersection/internal/types"
	"golang.org/x/sync/errgroup"
)

// frequencyMap tracks how many times each composite key appears in a dataset
type frequencyMap map[string]uint64

func (f frequencyMap) totalCount() uint64 {
	var total uint64
	for _, count := range f {
		total += count
	}
	return total
}

func (f frequencyMap) distinctCount() uint64 {
	return uint64(len(f))
}

func (f frequencyMap) overlap(other frequencyMap) (distinctOverlap uint64, totalOverlap uint64) {
	for key, countA := range f {
		countB, exists := other[key]
		if !exists {
			continue
		}
		distinctOverlap++
		totalOverlap += countA * countB
	}
	return distinctOverlap, totalOverlap
}

// PairwiseExact computes exact set intersection statistics for exactly two datasets
type PairwiseExact struct{}

func NewPairwiseExact() *PairwiseExact {
	return &PairwiseExact{}
}

// Compute streams both datasets in parallel and returns exact intersection statistic
func (p *PairwiseExact) Compute(ctx context.Context, datasets []types.KeyIterator) (types.IntersectionResult, error) {
	if len(datasets) != 2 {
		return nil, fmt.Errorf("pairwise exact requires exactly two datasets, got %d", len(datasets))
	}

	g, ctx := errgroup.WithContext(ctx)

	freqMaps := []frequencyMap{{}, {}}
	connectorStats := []types.ConnectorStats{{}, {}}

	for datasetIdx, dataset := range datasets {
		datasetIdx, dataset := datasetIdx, dataset
		g.Go(func() error {
			freqMap, stats, err := streamDataset(ctx, dataset)
			if err != nil {
				return err
			}
			freqMaps[datasetIdx] = freqMap
			connectorStats[datasetIdx] = stats
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	distinctOverlap, totalOverlap := freqMaps[0].overlap(freqMaps[1])

	result := types.PairwiseResult{
		Datasets: []types.PairwiseDatasetStats{
			{
				Source:        connectorStats[0].Source,
				TotalCount:    freqMaps[0].totalCount(),
				DistinctCount: freqMaps[0].distinctCount(),
			},
			{
				Source:        connectorStats[1].Source,
				TotalCount:    freqMaps[1].totalCount(),
				DistinctCount: freqMaps[1].distinctCount(),
			},
		},
		ConnectorStats:  connectorStats,
		DistinctOverlap: distinctOverlap,
		TotalOverlap:    totalOverlap,
		ErrorBoundPct:   0, // we are using an exact match algorithm so it is 0
	}

	return result, nil
}

const keyDelimiter = "\x00"

func streamDataset(ctx context.Context, iter types.KeyIterator) (frequencyMap, types.ConnectorStats, error) {
	freqMap := frequencyMap{}
	for {
		batch, done, err := iter.NextBatch(ctx)
		if err != nil {
			return nil, iter.Stats(), fmt.Errorf("failed to retrieve next batch: %w", err)
		}
		for _, keys := range batch {
			freqMap[strings.Join(keys, keyDelimiter)]++
		}
		if done {
			return freqMap, iter.Stats(), nil
		}
	}
}
