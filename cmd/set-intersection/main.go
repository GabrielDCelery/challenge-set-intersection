package main

import (
	"context"
	"os"
	"time"

	"github.com/GabrielDCelery/challenge-set-intersection/internal/algorithm"
	"github.com/GabrielDCelery/challenge-set-intersection/internal/connector"
	"github.com/GabrielDCelery/challenge-set-intersection/internal/types"
	"github.com/GabrielDCelery/challenge-set-intersection/internal/writer"
	"github.com/rs/zerolog"
)

func main() {
	log := zerolog.New(os.Stderr).With().Timestamp().Logger()

	start := time.Now()

	log.Info().
		Str("source_a", "/data/A_f.csv").
		Str("source_b", "/data/B_f.csv").
		Str("key_columns", "udprn").
		Msg("starting")

	iterA, err := connector.NewCsvKeyIterator("/data/A_f.csv", []string{"udprn"}, 1000, 0, log)

	if err != nil {
		log.Fatal().Err(err).Msg("failed to create iterator A")
	}

	defer iterA.Close()

	iterB, err := connector.NewCsvKeyIterator("/data/B_f.csv", []string{"udprn"}, 1000, 0, log)

	if err != nil {
		log.Fatal().Err(err).Msg("failed to create iterator B")
	}

	defer iterB.Close()

	algo := algorithm.NewPairwiseExact()

	result, err := algo.Compute(context.Background(), []types.KeyIterator{iterA, iterB})

	if err != nil {
		log.Fatal().Err(err).Msg("failed to compute intersection")
	}

	for _, stats := range result.(types.PairwiseResult).ConnectorStats {
		log.Info().
			Str("source", stats.Source).
			Uint64("rows_read", stats.RowsRead).
			Uint64("rows_skipped", stats.RowsSkipped).
			Msg("connector finished")
	}

	w := writer.NewStdoutWriter()

	if err := w.Write(result); err != nil {
		log.Fatal().Err(err).Msg("failed to write result")
	}

	log.Info().Dur("duration", time.Since(start)).Msg("job finished")
}
