package algorithm

import (
	"context"
	"testing"

	"github.com/GabrielDCelery/challenge-set-intersection/internal/types"
	"github.com/stretchr/testify/require"
)

type mockIterator struct {
	source string
	keys   [][]string
	done   bool
}

func (m *mockIterator) NextBatch(ctx context.Context) ([][]string, bool, error) {
	if m.done {
		return nil, true, nil
	}
	m.done = true
	return m.keys, true, nil
}

func (m *mockIterator) Stats() types.ConnectorStats {
	return types.ConnectorStats{Source: m.source}
}

func (m *mockIterator) Close() error {
	return nil
}

func newMockIterator(source string, keys [][]string) *mockIterator {
	return &mockIterator{
		source: source,
		keys:   keys,
		done:   false,
	}
}

func TestWorkedExample(t *testing.T) {
	// Given the example datasets
	// Dataset 1: A B C D D E F F
	// Dataset 2: A C C D F F F X Y
	iterA := newMockIterator("sA", [][]string{{"A"}, {"B"}, {"C"}, {"D"}, {"D"}, {"E"}, {"F"}, {"F"}})
	iterB := newMockIterator("sB", [][]string{{"A"}, {"C"}, {"C"}, {"D"}, {"F"}, {"F"}, {"F"}, {"X"}, {"Y"}})

	// When the pairwise exact algorithm runs
	algo := NewPairwiseExact()
	result, err := algo.Compute(context.Background(), []types.KeyIterator{iterA, iterB})

	// Then it calculates the output correctly
	r := result.(types.PairwiseResult)
	require.NoError(t, err)
	require.Equal(t, types.PairwiseResult{
		Datasets: []types.PairwiseDatasetStats{
			{Source: "sA", TotalCount: 8, DistinctCount: 6},
			{Source: "sB", TotalCount: 9, DistinctCount: 6},
		},
		ConnectorStats:  []types.ConnectorStats{{Source: "sA"}, {Source: "sB"}},
		DistinctOverlap: 4,
		TotalOverlap:    11,
		ErrorBoundPct:   0,
	}, r)
}

func TestDistinctVsTotalOverlap(t *testing.T) {
	// Given two datasets with a single key repeated multiple times
	iterA := newMockIterator("sA", [][]string{{"A"}, {"A"}})
	iterB := newMockIterator("sB", [][]string{{"A"}, {"A"}, {"A"}})

	// When the pairwise exact algorithm runs
	algo := NewPairwiseExact()
	result, err := algo.Compute(context.Background(), []types.KeyIterator{iterA, iterB})

	// Then it calculates the output correctly
	r := result.(types.PairwiseResult)
	require.NoError(t, err)
	require.Equal(t, types.PairwiseResult{
		Datasets: []types.PairwiseDatasetStats{
			{Source: "sA", TotalCount: 2, DistinctCount: 1},
			{Source: "sB", TotalCount: 3, DistinctCount: 1},
		},
		ConnectorStats:  []types.ConnectorStats{{Source: "sA"}, {Source: "sB"}},
		DistinctOverlap: 1,
		TotalOverlap:    6,
		ErrorBoundPct:   0,
	}, r)
}

func TestNoOverlap(t *testing.T) {
	// Given two datasets where there are no overlaps
	iterA := newMockIterator("sA", [][]string{{"A"}, {"B"}, {"C"}})
	iterB := newMockIterator("sB", [][]string{{"X"}, {"Y"}, {"Z"}})

	// When the pairwise exact algorithm runs
	algo := NewPairwiseExact()
	result, err := algo.Compute(context.Background(), []types.KeyIterator{iterA, iterB})

	// Then it calculates the output correctly
	r := result.(types.PairwiseResult)
	require.NoError(t, err)
	require.Equal(t, types.PairwiseResult{
		Datasets: []types.PairwiseDatasetStats{
			{Source: "sA", TotalCount: 3, DistinctCount: 3},
			{Source: "sB", TotalCount: 3, DistinctCount: 3},
		},
		ConnectorStats:  []types.ConnectorStats{{Source: "sA"}, {Source: "sB"}},
		DistinctOverlap: 0,
		TotalOverlap:    0,
		ErrorBoundPct:   0,
	}, r)
}

func TestEmptyDataset(t *testing.T) {
	// Given two datasets where one is empty
	iterA := newMockIterator("sA", [][]string{})
	iterB := newMockIterator("sB", [][]string{{"A"}, {"B"}, {"C"}})

	// When the pairwise exact algorithm runs
	algo := NewPairwiseExact()
	result, err := algo.Compute(context.Background(), []types.KeyIterator{iterA, iterB})

	// Then it calculates the output correctly
	r := result.(types.PairwiseResult)
	require.NoError(t, err)
	require.Equal(t, types.PairwiseResult{
		Datasets: []types.PairwiseDatasetStats{
			{Source: "sA", TotalCount: 0, DistinctCount: 0},
			{Source: "sB", TotalCount: 3, DistinctCount: 3},
		},
		ConnectorStats:  []types.ConnectorStats{{Source: "sA"}, {Source: "sB"}},
		DistinctOverlap: 0,
		TotalOverlap:    0,
		ErrorBoundPct:   0,
	}, r)
}

func TestCompositeKeys(t *testing.T) {
	// Given two datasets with composite keys
	iterA := newMockIterator("sA", [][]string{{"John", "john@example.com"}, {"John", "other@example.com"}})
	iterB := newMockIterator("sB", [][]string{{"John", "john@example.com"}, {"Jane", "john@example.com"}})

	// When the pairwise exact algorithm runs
	algo := NewPairwiseExact()
	result, err := algo.Compute(context.Background(), []types.KeyIterator{iterA, iterB})

	// Then it calculates the output correctly
	r := result.(types.PairwiseResult)
	require.NoError(t, err)
	require.Equal(t, types.PairwiseResult{
		Datasets: []types.PairwiseDatasetStats{
			{Source: "sA", TotalCount: 2, DistinctCount: 2},
			{Source: "sB", TotalCount: 2, DistinctCount: 2},
		},
		ConnectorStats:  []types.ConnectorStats{{Source: "sA"}, {Source: "sB"}},
		DistinctOverlap: 1,
		TotalOverlap:    1,
		ErrorBoundPct:   0,
	}, r)
}
