package writer

import (
	"bytes"
	"testing"

	"github.com/GabrielDCelery/challenge-set-intersection/internal/types"
	"github.com/stretchr/testify/require"
)

func TestWritePairwiseResult(t *testing.T) {
	result := types.PairwiseResult{
		Datasets: []types.PairwiseDatasetStats{
			{Source: "data/A.csv", TotalCount: 8, DistinctCount: 6},
			{Source: "data/B.csv", TotalCount: 9, DistinctCount: 7},
		},
		ConnectorStats:  []types.ConnectorStats{{Source: "data/A.csv", RowsSkipped: 1}, {Source: "data/B.csv"}},
		DistinctOverlap: 4,
		TotalOverlap:    11,
		ErrorBoundPct:   0,
	}

	var buf bytes.Buffer
	w := &StdoutWriter{out: &buf}
	require.NoError(t, w.Write(result))

	expected := `Dataset: data/A.csv
  Total rows:    8
  Distinct keys: 6
  Rows skipped:  1

Dataset: data/B.csv
  Total rows:    9
  Distinct keys: 7
  Rows skipped:  0

Overlap
  Distinct: 4
  Total:    11
`
	require.Equal(t, expected, buf.String())
}

func TestWritePairwiseResultWithErrorBound(t *testing.T) {
	result := types.PairwiseResult{
		Datasets: []types.PairwiseDatasetStats{
			{Source: "data/A.csv", TotalCount: 8, DistinctCount: 6},
			{Source: "data/B.csv", TotalCount: 9, DistinctCount: 7},
		},
		ConnectorStats:  []types.ConnectorStats{{Source: "data/A.csv"}, {Source: "data/B.csv"}},
		DistinctOverlap: 4,
		TotalOverlap:    11,
		ErrorBoundPct:   0.8,
	}

	var buf bytes.Buffer
	w := &StdoutWriter{out: &buf}
	require.NoError(t, w.Write(result))

	expected := `Dataset: data/A.csv
  Total rows:    8
  Distinct keys: 6
  Rows skipped:  0

Dataset: data/B.csv
  Total rows:    9
  Distinct keys: 7
  Rows skipped:  0

Overlap
  Distinct: 4
  Total:    11
  Error bound: ±0.8%
`
	require.Equal(t, expected, buf.String())
}
