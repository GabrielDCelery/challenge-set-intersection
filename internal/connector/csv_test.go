package connector

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.csv")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func TestLeadingZerosPreserved(t *testing.T) {
	// Given a CSV file with UDPRN keys that have leading zeros
	path := writeFixture(t, "udprn\n08034283\n00000001\n")

	// When the connector reads the file
	it, err := NewCsvKeyIterator(path, []string{"udprn"}, 10, 0)
	require.NoError(t, err)
	defer it.Close()

	// Then leading zeros are preserved as raw strings
	batch, done, err := it.NextBatch(context.Background())
	require.NoError(t, err)
	require.True(t, done)
	require.Equal(t, [][]string{{"08034283"}, {"00000001"}}, batch)
}

func TestColumnResolutionByName(t *testing.T) {
	// Given in the CSV file the headers are out of order (key column is second not first)
	path := writeFixture(t, "name,udprn\nJohn,08034283\n")

	// When the connector reads the file
	it, err := NewCsvKeyIterator(path, []string{"udprn"}, 10, 0)
	require.NoError(t, err)
	defer it.Close()

	// Then the correct column is extracted regardless of its position
	batch, done, err := it.NextBatch(context.Background())
	require.NoError(t, err)
	require.True(t, done)
	require.Equal(t, [][]string{{"08034283"}}, batch)
}
