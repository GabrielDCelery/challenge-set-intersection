package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func TestValidConfigParsesCorrectly(t *testing.T) {
	content := `
datasets:
  - connector: csv
    page_size: 1000
    max_error_rate: 0.05
    params:
      path: /data/A_f.csv
  - connector: csv
    page_size: 1000
    max_error_rate: 0.05
    params:
      path: /data/B_f.csv

key_columns: [udprn]

algorithm:
  type: pairwise_exact

output:
  writer: stdout

run:
  timeout_seconds: 3600
`
	path := writeConfig(t, content)
	cfg, err := Load(path)

	require.NoError(t, err)
	require.Equal(t, &Config{
		Datasets: []DatasetConfig{
			{
				Connector:    "csv",
				PageSize:     1000,
				MaxErrorRate: 0.05,
				Params:       map[string]string{"path": "/data/A_f.csv"},
			},
			{
				Connector:    "csv",
				PageSize:     1000,
				MaxErrorRate: 0.05,
				Params:       map[string]string{"path": "/data/B_f.csv"},
			},
		},
		KeyColumns: []string{"udprn"},
		Algorithm: AlgorithmConfig{
			Type: "pairwise_exact",
		},
		Output: OutputConfig{
			Writer: "stdout",
		},
		Run: RunConfig{
			TimeoutSeconds: 3600,
		},
	}, cfg)
}

func TestEnvVarExpansion(t *testing.T) {
	t.Setenv("DATA_PATH_A", "/data/A_f.csv")

	content := `
datasets:
  - connector: csv
    page_size: 1000
    max_error_rate: 0.05
    params:
      path: ${DATA_PATH_A}
  - connector: csv
    page_size: 1000
    max_error_rate: 0.05
    params:
      path: /data/B_f.csv

key_columns: [udprn]

algorithm:
  type: pairwise_exact

output:
  writer: stdout

run:
  timeout_seconds: 3600
`
	path := writeConfig(t, content)
	cfg, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, "/data/A_f.csv", cfg.Datasets[0].Params["path"])
}

func TestValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		errMsg  string
	}{
		{
			name:   "missing key_columns",
			errMsg: "key_columns",
			content: `
datasets:
  - connector: csv
    page_size: 1000
    max_error_rate: 0.05
    params:
      path: /data/A_f.csv
  - connector: csv
    page_size: 1000
    max_error_rate: 0.05
    params:
      path: /data/B_f.csv
algorithm:
  type: pairwise_exact
output:
  writer: stdout
run:
  timeout_seconds: 3600
`,
		},
		{
			name:   "missing algorithm type",
			errMsg: "algorithm.type",
			content: `
datasets:
  - connector: csv
    page_size: 1000
    max_error_rate: 0.05
    params:
      path: /data/A_f.csv
  - connector: csv
    page_size: 1000
    max_error_rate: 0.05
    params:
      path: /data/B_f.csv
key_columns: [udprn]
algorithm:
  type: ""
output:
  writer: stdout
run:
  timeout_seconds: 3600
`,
		},
		{
			name:   "fewer than 2 datasets",
			errMsg: "2 datasets",
			content: `
datasets:
  - connector: csv
    page_size: 1000
    max_error_rate: 0.05
    params:
      path: /data/A_f.csv
key_columns: [udprn]
algorithm:
  type: pairwise_exact
output:
  writer: stdout
run:
  timeout_seconds: 3600
`,
		},
		{
			name:    "malformed yaml",
			errMsg:  "could not parse",
			content: `this: is: not: valid: yaml:`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, tt.content)
			_, err := Load(path)
			require.ErrorContains(t, err, tt.errMsg)
		})
	}
}
