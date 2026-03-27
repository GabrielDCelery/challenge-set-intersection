package types

import "context"

type KeyIterator interface {
	// NextBatch returns up to pageSize rows of key fields. Malformed rows are skipped and recorded in ConnectorStats. Returns done=true on the final batch
	NextBatch(ctx context.Context) (keys [][]string, done bool, err error)
	// Stats returns accumulated connector statistics and is safe to call after every batch
	Stats() ConnectorStats
	Close() error
}

type RowError struct {
	RowNumber uint64
	Reason    string
}

type ConnectorStats struct {
	Source      string
	RowsRead    uint64
	RowsSkipped uint64
	Errors      []RowError
}
