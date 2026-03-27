package config

import (
	"fmt"

	"github.com/GabrielDCelery/challenge-set-intersection/internal/algorithm"
	"github.com/GabrielDCelery/challenge-set-intersection/internal/connector"
	"github.com/GabrielDCelery/challenge-set-intersection/internal/types"
	"github.com/GabrielDCelery/challenge-set-intersection/internal/writer"
	"github.com/rs/zerolog"
)

func BuildConnectors(cfg *Config, log zerolog.Logger) ([]types.KeyIterator, error) {
	iterators := make([]types.KeyIterator, 0, len(cfg.Datasets))
	for _, datasetCfg := range cfg.Datasets {
		switch datasetCfg.Connector {
		case "csv":
			path, ok := datasetCfg.Params["path"]
			if !ok {
				return nil, fmt.Errorf("csv connector missing required param: path")
			}
			it, err := connector.NewCsvKeyIterator(path, cfg.KeyColumns, datasetCfg.PageSize, datasetCfg.MaxErrorRate, log)
			if err != nil {
				return nil, fmt.Errorf("failed to create csv connector for %s: %w", path, err)
			}
			iterators = append(iterators, it)
		default:
			return nil, fmt.Errorf("unsupported connector type: %s", datasetCfg.Connector)
		}
	}
	return iterators, nil
}

func BuildAlgorithm(cfg *Config) (types.IntersectionAlgorithm, error) {
	switch cfg.Algorithm.Type {
	case "pairwise_exact":
		return algorithm.NewPairwiseExact(), nil
	default:
		return nil, fmt.Errorf("unsupported algorithm type: %s", cfg.Algorithm.Type)
	}
}

func BuildWriter(cfg *Config) (types.ResultWriter, error) {
	switch cfg.Output.Writer {
	case "stdout":
		return writer.NewStdoutWriter(), nil
	default:
		return nil, fmt.Errorf("unsupported writer type: %s", cfg.Output.Writer)
	}
}
