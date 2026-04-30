package store

import (
	"math"
	"testing"
)

func TestCosineSimilarity_IdenticalVectors(t *testing.T) {
	v := Vector{1.0, 2.0, 3.0}
	sim := CosineSimilarity(v, v)
	if math.Abs(sim-1.0) > 1e-9 {
		t.Errorf("identical vectors: got %f, want 1.0", sim)
	}
}

func TestCosineSimilarity_OrthogonalVectors(t *testing.T) {
	a := Vector{1.0, 0.0}
	b := Vector{0.0, 1.0}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim) > 1e-9 {
		t.Errorf("orthogonal vectors: got %f, want 0.0", sim)
	}
}

func TestCosineSimilarity_OppositeVectors(t *testing.T) {
	a := Vector{1.0, 0.0}
	b := Vector{-1.0, 0.0}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim-(-1.0)) > 1e-9 {
		t.Errorf("opposite vectors: got %f, want -1.0", sim)
	}
}

func TestCosineSimilarity_DifferentLengths(t *testing.T) {
	a := Vector{1.0, 2.0}
	b := Vector{1.0}
	sim := CosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("different lengths: got %f, want 0", sim)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := Vector{0.0, 0.0}
	b := Vector{1.0, 2.0}
	sim := CosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("zero vector: got %f, want 0", sim)
	}
}

func TestDotProduct(t *testing.T) {
	a := Vector{1.0, 2.0, 3.0}
	b := Vector{4.0, 5.0, 6.0}
	dp := DotProduct(a, b)
	expected := 1.0*4.0 + 2.0*5.0 + 3.0*6.0
	if math.Abs(dp-expected) > 1e-9 {
		t.Errorf("dot product: got %f, want %f", dp, expected)
	}
}

func TestNormalize(t *testing.T) {
	v := Vector{3.0, 4.0}
	n := Normalize(v)
	expected := Vector{0.6, 0.8}
	for i := range n {
		if math.Abs(n[i]-expected[i]) > 1e-9 {
			t.Errorf("normalize[%d]: got %f, want %f", i, n[i], expected[i])
		}
	}
	norm := Norm(n)
	if math.Abs(norm-1.0) > 1e-9 {
		t.Errorf("normalized vector norm: got %f, want 1.0", norm)
	}
}

func TestNormalize_ZeroVector(t *testing.T) {
	v := Vector{0.0, 0.0, 0.0}
	n := Normalize(v)
	for _, x := range n {
		if x != 0 {
			t.Errorf("normalize zero vector: got %f, want 0", x)
		}
	}
}

func TestEuclideanDistance(t *testing.T) {
	a := Vector{1.0, 2.0, 3.0}
	b := Vector{4.0, 5.0, 6.0}
	d := EuclideanDistance(a, b)
	expected := math.Sqrt(27)
	if math.Abs(d-expected) > 1e-9 {
		t.Errorf("euclidean distance: got %f, want %f", d, expected)
	}
}

func TestEuclideanDistance_SamePoint(t *testing.T) {
	v := Vector{1.0, 2.0}
	d := EuclideanDistance(v, v)
	if math.Abs(d) > 1e-9 {
		t.Errorf("same point distance: got %f, want 0", d)
	}
}

func TestEuclideanDistance_DifferentLengths(t *testing.T) {
	a := Vector{1.0}
	b := Vector{1.0, 2.0}
	d := EuclideanDistance(a, b)
	if !math.IsInf(d, 1) {
		t.Errorf("different lengths: got %f, want +Inf", d)
	}
}
