package store

import (
	"math"
)

// Vector represents a dense embedding vector.
type Vector []float64

// DotProduct returns the dot product of two vectors.
// Returns 0 if vectors have different lengths.
func DotProduct(a, b Vector) float64 {
	if len(a) != len(b) {
		return 0
	}
	var sum float64
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

// Norm returns the L2 norm (magnitude) of a vector.
func Norm(v Vector) float64 {
	return math.Sqrt(DotProduct(v, v))
}

// CosineSimilarity returns the cosine similarity between two vectors.
// Returns 1.0 for identical vectors, 0.0 for orthogonal, -1.0 for opposite.
// Returns 0 if either vector has zero norm or different lengths.
func CosineSimilarity(a, b Vector) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	normA := Norm(a)
	normB := Norm(b)
	if normA == 0 || normB == 0 {
		return 0
	}
	return DotProduct(a, b) / (normA * normB)
}

// Normalize returns a unit-length copy of the vector.
// Returns a zero vector if the input has zero norm.
func Normalize(v Vector) Vector {
	if len(v) == 0 {
		return v
	}
	n := Norm(v)
	if n == 0 {
		return make(Vector, len(v))
	}
	result := make(Vector, len(v))
	for i := range v {
		result[i] = v[i] / n
	}
	return result
}

// EuclideanDistance returns the L2 distance between two vectors.
// Returns +Inf if vectors have different lengths.
func EuclideanDistance(a, b Vector) float64 {
	if len(a) != len(b) {
		return math.Inf(1)
	}
	var sum float64
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return math.Sqrt(sum)
}
