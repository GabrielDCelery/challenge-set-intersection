package main

import (
	"context"
	"flag"
	"os"
	"time"

	"github.com/GabrielDCelery/challenge-set-intersection/internal/config"
	"github.com/GabrielDCelery/challenge-set-intersection/internal/types"
	"github.com/rs/zerolog"
)

func main() {
	log := zerolog.New(os.Stderr).With().Timestamp().Logger()

	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	if *configPath == "" {
		log.Fatal().Msg("--config flag is required")
	}

	start := time.Now()

	cfg, err := config.Load(*configPath)

	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	log.Info().
		Str("config", *configPath).
		Strs("key_columns", cfg.KeyColumns).
		Str("algorithm", cfg.Algorithm.Type).
		Msg("starting")

	connectors, err := config.BuildConnectors(cfg, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to build connectors")
	}
	for _, connector := range connectors {
		defer connector.Close()
	}

	algo, err := config.BuildAlgorithm(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to build algorithm")
	}

	w, err := config.BuildWriter(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to build writer")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Run.TimeoutSeconds)*time.Second)
	defer cancel()

	result, err := algo.Compute(ctx, connectors)
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

	if err := w.Write(result); err != nil {
		log.Fatal().Err(err).Msg("failed to write result")
	}

	log.Info().Dur("duration", time.Since(start)).Msg("job finished")
}
