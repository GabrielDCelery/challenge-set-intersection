package types

type ResultWriter interface {
	// Write formats and outputs the intersection result to the configured destination
	Write(result IntersectionResult) error
}
