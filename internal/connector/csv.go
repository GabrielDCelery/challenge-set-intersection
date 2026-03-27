package connector

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/GabrielDCelery/challenge-set-intersection/internal/types"
)

// CsvKeyIterator streams key fields from a CSV file one batch at a time
type CsvKeyIterator struct {
	file         *os.File
	reader       *csv.Reader
	source       string
	stats        types.ConnectorStats
	keyIndices   []int
	pageSize     int
	maxErrorRate float64
	done         bool
}

func NewCsvKeyIterator(path string, keyColumns []string, pageSize int, maxErrorRate float64) (*CsvKeyIterator, error) {
	// NOTE: word-readable file check omitted for now
	// In production it would reject files with permissions looser than 600
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("can not open file %s: %w", path, err)
	}

	reader := csv.NewReader(file)

	// Read headers into slice []string{"udprn", "name", "postcode"}
	header, err := reader.Read()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("could not read header from: %s: %w", path, err)
	}

	// Create an index map of the headers so we can tell which indecies we will have to extract
	// map[string]int{"udprn": 0, "name": 1, "postcode": 2}
	colIndex := make(map[string]int, len(header))
	for i, col := range header {
		colIndex[col] = i
	}
	keyIndices := make([]int, len(keyColumns))
	for i, col := range keyColumns {
		idx, ok := colIndex[col]
		if !ok {
			file.Close()
			return nil, fmt.Errorf("column %q not found in %s", col, path)
		}
		keyIndices[i] = idx
	}

	return &CsvKeyIterator{
		file:         file,
		reader:       reader,
		source:       path,
		keyIndices:   keyIndices,
		pageSize:     pageSize,
		maxErrorRate: maxErrorRate,
		stats:        types.ConnectorStats{Source: path},
	}, nil
}

func (c *CsvKeyIterator) NextBatch(ctx context.Context) ([][]string, bool, error) {
	if c.done {
		return nil, true, nil
	}

	batch := make([][]string, 0, c.pageSize)
	for i := 0; i < c.pageSize; i++ {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		row, err := c.reader.Read()
		if errors.Is(err, io.EOF) {
			c.done = true
			return batch, true, nil
		}
		c.stats.RowsRead++
		if err != nil {
			c.appendSkipError(err)
			continue
		}
		keyFields, err := c.extractKeyFieldsFromRow(row)
		if err != nil {
			c.appendSkipError(err)
			continue
		}
		batch = append(batch, keyFields)

	}
	if c.maxErrorRate > 0 && c.stats.RowsRead > 0 {
		errorRate := float64(c.stats.RowsSkipped) / float64(c.stats.RowsRead)
		if errorRate > c.maxErrorRate {
			return nil, false, fmt.Errorf("error rate %.2f exceeds max error rate %.4f for %s", errorRate, c.maxErrorRate, c.source)
		}
	}
	return batch, false, nil
}

func (c *CsvKeyIterator) extractKeyFieldsFromRow(row []string) ([]string, error) {
	keyFields := make([]string, len(c.keyIndices))
	for i, keyIdx := range c.keyIndices {
		if keyIdx >= len(row) {
			return nil, fmt.Errorf("row has %d fields, index %d is out of bounds", len(row), keyIdx)
		}
		if row[keyIdx] == "" {
			return nil, fmt.Errorf("empty key field at column index %d", keyIdx)
		}
		keyFields[i] = row[keyIdx]
	}
	return keyFields, nil
}

func (c *CsvKeyIterator) appendSkipError(err error) {
	c.stats.RowsSkipped++
	c.stats.Errors = append(c.stats.Errors, types.RowError{
		RowNumber: c.stats.RowsRead,
		Reason:    fmt.Sprintf("csv parse error: %v", err),
	})
}

func (c *CsvKeyIterator) Stats() types.ConnectorStats {
	return c.stats
}

func (c *CsvKeyIterator) Close() error {
	return c.file.Close()
}
