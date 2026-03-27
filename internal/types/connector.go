package types

import "context"

type KeyIterator interface {
	NextBatch(ctx context.Context) (keys [][]string, done bool, err error)
	Stats() ConnectorStats
	Close() error
}

type RowError struct {
	RowNumbern uint64
	Reason     string
}

type ConnectorStats struct {
	Source      string
	RowsRead    uint64
	RowsSkipped uint64
	Errors      []RowError
}
