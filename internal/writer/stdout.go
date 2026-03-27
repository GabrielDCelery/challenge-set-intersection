package writer

import (
	"fmt"
	"io"
	"os"

	"github.com/GabrielDCelery/challenge-set-intersection/internal/types"
)

type StdoutWriter struct {
	out io.Writer
}

func NewStdoutWriter() *StdoutWriter {
	return &StdoutWriter{out: os.Stdout}
}

func (w *StdoutWriter) Write(result types.IntersectionResult) error {
	switch r := result.(type) {
	case types.PairwiseResult:
		return w.writePairwiseResult(r)
	default:
		return fmt.Errorf("unsupported result type: %T", result)
	}
}

func (w *StdoutWriter) writePairwiseResult(r types.PairwiseResult) error {
	for i, dataset := range r.Datasets {
		fmt.Fprintf(w.out, "Dataset: %s\n", dataset.Source)
		fmt.Fprintf(w.out, "  Total rows:    %d\n", dataset.TotalCount)
		fmt.Fprintf(w.out, "  Distinct keys: %d\n", dataset.DistinctCount)
		fmt.Fprintf(w.out, "  Rows skipped:  %d\n", r.ConnectorStats[i].RowsSkipped)
		fmt.Fprintf(w.out, "\n")
	}
	fmt.Fprintf(w.out, "Overlap\n")
	fmt.Fprintf(w.out, "  Distinct: %d\n", r.DistinctOverlap)
	fmt.Fprintf(w.out, "  Total:    %d\n", r.TotalOverlap)
	if r.ErrorBoundPct != 0 {
		fmt.Fprintf(w.out, "  Error bound: ±%.1f%%\n", r.ErrorBoundPct)
	}
	return nil
}
