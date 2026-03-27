package algorithm

import (
	"context"
	"fmt"
	"strings"

	"github.com/GabrielDCelery/challenge-set-intersection/internal/types"
	"golang.org/x/sync/errgroup"
)

type PairwiseExact struct{}

func NewPairwiseExact() *PairwiseExact {
	return &PairwiseExact{}
}

func (p *PairwiseExact) Compute(ctx context.Context, datasets []types.KeyIterator) (types.IntersectionResult, error) {
	if len(datasets) != 2 {
		return nil, fmt.Errorf("pairwise extract requires exactly two datasets, got %d", len(datasets))
	}

	g, ctx := errgroup.WithContext(ctx)

	connectorStats := make([]types.ConnectorStats, 2)
	frequencyMaps := []map[string]uint64{make(map[string]uint64), make(map[string]uint64)}

	for datasetIdx, dataset := range datasets {
		g.Go(func() error {
			for {
				batch, done, err := dataset.NextBatch(ctx)
				if err != nil {
					return fmt.Errorf("algorithm failed to retrieve next batch: %w", err)
				}
				for _, keys := range batch {
					uniqueKey := strings.Join(keys, "\x00")
					frequencyMaps[datasetIdx][uniqueKey]++
				}
				if done {
					connectorStats[datasetIdx] = dataset.Stats()
					return nil
				}
			}
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	distinctOverlap, totalOverlap := calculateDistinctAndTotalOverlap(frequencyMaps)

	result := types.PairwiseResult{
		Datasets: []types.PairwiseDatasetStats{
			createDatasetStats(connectorStats[0].Source, frequencyMaps[0]),
			createDatasetStats(connectorStats[1].Source, frequencyMaps[1]),
		},
		ConnectorStats:  connectorStats,
		DistinctOverlap: distinctOverlap,
		TotalOverlap:    totalOverlap,
		ErrorBoundPct:   0, // we are using an exact match algorithm so it is 0
	}

	return result, nil
}

func createDatasetStats(source string, frequencyMap map[string]uint64) types.PairwiseDatasetStats {
	return types.PairwiseDatasetStats{
		Source:        source,
		TotalCount:    calculateTotal(frequencyMap),
		DistinctCount: calculateDistinct(frequencyMap),
	}
}

func calculateTotal(frequencyMap map[string]uint64) uint64 {
	var total uint64 = 0
	for _, count := range frequencyMap {
		total += count
	}
	return total
}

func calculateDistinct(frequencyMap map[string]uint64) uint64 {
	return uint64(len(frequencyMap))
}

func calculateDistinctAndTotalOverlap(frequencyMaps []map[string]uint64) (uint64, uint64) {
	var distinctOverlap, totalOverlap uint64
	for key, countA := range frequencyMaps[0] {
		countB, exists := frequencyMaps[1][key]
		if !exists {
			continue
		}
		distinctOverlap++
		totalOverlap += countA * countB
	}
	return distinctOverlap, totalOverlap
}
