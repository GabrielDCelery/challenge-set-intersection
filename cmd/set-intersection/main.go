package main

import (
	"context"
	"log"

	"github.com/GabrielDCelery/challenge-set-intersection/internal/algorithm"
	"github.com/GabrielDCelery/challenge-set-intersection/internal/connector"
	"github.com/GabrielDCelery/challenge-set-intersection/internal/types"
	"github.com/GabrielDCelery/challenge-set-intersection/internal/writer"
)

func main() {
	iterA, err := connector.NewCsvKeyIterator("data/A_f.csv", []string{"udprn"}, 1000, 0)

	if err != nil {
		log.Fatalf("failed to create iterator A: %v", err)
	}

	defer iterA.Close()

	iterB, err := connector.NewCsvKeyIterator("data/B_f.csv", []string{"udprn"}, 1000, 0)

	if err != nil {
		log.Fatalf("failed to create iterator B: %v", err)
	}

	defer iterB.Close()

	algo := algorithm.NewPairwiseExact()

	result, err := algo.Compute(context.Background(), []types.KeyIterator{iterA, iterB})

	if err != nil {
		log.Fatalf("failed to compute intersection: %v", err)
	}

	w := writer.NewStdoutWriter()

	if err := w.Write(result); err != nil {
		log.Fatalf("failed to write result: %v", err)
	}
}
