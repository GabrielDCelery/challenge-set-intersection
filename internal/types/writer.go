package types

type ResultWriter interface {
	Write(result IntersectionResult) error
}
