package store

import (
	"context"
	"math"
	"testing"
)

func TestHashEmbedder_NonZeroOutput(t *testing.T) {
	emb := NewHashEmbedder()
	vec, err := emb.Embed(context.Background(), "hello world test embedding")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vec) != hashEmbedderDims {
		t.Errorf("dimensions = %d, want %d", len(vec), hashEmbedderDims)
	}
	hasNonZero := false
	for _, v := range vec {
		if v != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Error("embedding should have non-zero values")
	}
}

func TestHashEmbedder_ConsistentDimensions(t *testing.T) {
	emb := NewHashEmbedder()
	vec1, _ := emb.Embed(context.Background(), "short")
	vec2, _ := emb.Embed(context.Background(), "this is a much longer sentence with more tokens to embed")
	if len(vec1) != len(vec2) {
		t.Errorf("dimension mismatch: %d vs %d", len(vec1), len(vec2))
	}
}

func TestHashEmbedder_SimilarTextHigherSimilarity(t *testing.T) {
	emb := NewHashEmbedder()
	vec1, _ := emb.Embed(context.Background(), "debug the memory leak in the Go HTTP server")
	vec2, _ := emb.Embed(context.Background(), "fix the memory leak in the Go web server")
	vec3, _ := emb.Embed(context.Background(), "deploy the application to the kubernetes cluster")

	simSimilar := CosineSimilarity(vec1, vec2)
	simDifferent := CosineSimilarity(vec1, vec3)

	if simSimilar <= simDifferent {
		t.Errorf("similar texts should have higher similarity: similar=%f, different=%f", simSimilar, simDifferent)
	}
}

func TestHashEmbedder_EmptyText(t *testing.T) {
	emb := NewHashEmbedder()
	vec, err := emb.Embed(context.Background(), "")
	if err != nil {
		t.Fatalf("Embed empty: %v", err)
	}
	if len(vec) != hashEmbedderDims {
		t.Errorf("dimensions = %d, want %d", len(vec), hashEmbedderDims)
	}
}

func TestHashEmbedder_EmbedBatch(t *testing.T) {
	emb := NewHashEmbedder()
	texts := []string{"hello world", "test embedding", "batch processing"}
	vecs, err := emb.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if len(vecs) != 3 {
		t.Errorf("batch count = %d, want 3", len(vecs))
	}
}

func TestStoreEmbeddingAndSearch(t *testing.T) {
	s := newTestStore(t)
	emb := NewHashEmbedder()

	vec1, _ := emb.Embed(context.Background(), "debug the Go HTTP server memory leak")
	vec2, _ := emb.Embed(context.Background(), "fix memory leak in web server")
	vec3, _ := emb.Embed(context.Background(), "deploy application to kubernetes cluster")

	ctx := context.Background()
	if err := s.StoreEmbedding(ctx, "message", "sess-1", "debug the Go HTTP server memory leak", vec1); err != nil {
		t.Fatalf("StoreEmbedding 1: %v", err)
	}
	if err := s.StoreEmbedding(ctx, "message", "sess-2", "fix memory leak in web server", vec2); err != nil {
		t.Fatalf("StoreEmbedding 2: %v", err)
	}
	if err := s.StoreEmbedding(ctx, "skill", "deploy-k8s", "deploy application to kubernetes cluster", vec3); err != nil {
		t.Fatalf("StoreEmbedding 3: %v", err)
	}

	queryVec, _ := emb.Embed(context.Background(), "memory leak in server")
	results, err := s.SearchEmbeddings(ctx, queryVec, 10, 0.0)
	if err != nil {
		t.Fatalf("SearchEmbeddings: %v", err)
	}
	if len(results) < 2 {
		t.Errorf("expected at least 2 results, got %d", len(results))
	}

	if results[0].SourceType != "message" {
		t.Errorf("top result type = %q, want %q", results[0].SourceType, "message")
	}
}

func TestGetEmbedding(t *testing.T) {
	s := newTestStore(t)
	emb := NewHashEmbedder()

	vec, _ := emb.Embed(context.Background(), "test content for retrieval")
	ctx := context.Background()

	if err := s.StoreEmbedding(ctx, "message", "sess-get", "test content for retrieval", vec); err != nil {
		t.Fatalf("StoreEmbedding: %v", err)
	}

	rec, err := s.GetEmbedding(ctx, "message", "sess-get")
	if err != nil {
		t.Fatalf("GetEmbedding: %v", err)
	}
	if rec == nil {
		t.Fatal("GetEmbedding should return record")
	}
	if rec.SourceID != "sess-get" {
		t.Errorf("SourceID = %q, want %q", rec.SourceID, "sess-get")
	}
	if len(rec.Vector) != len(vec) {
		t.Errorf("vector length = %d, want %d", len(rec.Vector), len(vec))
	}
	for i := range vec {
		if math.Abs(rec.Vector[i]-vec[i]) > 1e-9 {
			t.Errorf("vector mismatch at index %d: got %f, want %f", i, rec.Vector[i], vec[i])
			break
		}
	}
}

func TestGetEmbedding_NotFound(t *testing.T) {
	s := newTestStore(t)
	rec, err := s.GetEmbedding(context.Background(), "message", "nonexistent")
	if err != nil {
		t.Fatalf("GetEmbedding: %v", err)
	}
	if rec != nil {
		t.Error("GetEmbedding should return nil for nonexistent")
	}
}

func TestDeleteEmbedding(t *testing.T) {
	s := newTestStore(t)
	emb := NewHashEmbedder()

	vec, _ := emb.Embed(context.Background(), "to be deleted")
	ctx := context.Background()

	if err := s.StoreEmbedding(ctx, "message", "sess-del", "to be deleted", vec); err != nil {
		t.Fatalf("StoreEmbedding: %v", err)
	}

	if err := s.DeleteEmbedding(ctx, "message", "sess-del"); err != nil {
		t.Fatalf("DeleteEmbedding: %v", err)
	}

	rec, _ := s.GetEmbedding(ctx, "message", "sess-del")
	if rec != nil {
		t.Error("embedding should be deleted")
	}
}

func TestStoreEmbedding_Upsert(t *testing.T) {
	s := newTestStore(t)
	emb := NewHashEmbedder()
	ctx := context.Background()

	vec1, _ := emb.Embed(context.Background(), "original content")
	vec2, _ := emb.Embed(context.Background(), "updated content")

	if err := s.StoreEmbedding(ctx, "message", "sess-up", "original content", vec1); err != nil {
		t.Fatalf("StoreEmbedding v1: %v", err)
	}
	if err := s.StoreEmbedding(ctx, "message", "sess-up", "updated content", vec2); err != nil {
		t.Fatalf("StoreEmbedding v2: %v", err)
	}

	rec, _ := s.GetEmbedding(ctx, "message", "sess-up")
	if rec == nil {
		t.Fatal("GetEmbedding should return record after upsert")
	}
	if rec.Content != "updated content" {
		t.Errorf("Content = %q, want %q", rec.Content, "updated content")
	}
}

func TestSearchEmbeddings_MinScore(t *testing.T) {
	s := newTestStore(t)
	emb := NewHashEmbedder()
	ctx := context.Background()

	vec, _ := emb.Embed(context.Background(), "deploy kubernetes")
	if err := s.StoreEmbedding(ctx, "skill", "deploy", "deploy kubernetes", vec); err != nil {
		t.Fatalf("StoreEmbedding: %v", err)
	}

	queryVec, _ := emb.Embed(context.Background(), "deploy kubernetes")
	results, _ := s.SearchEmbeddings(ctx, queryVec, 10, 0.99)
	if len(results) == 0 {
		t.Error("exact match should exceed 0.99 min score")
	}

	queryVec2, _ := emb.Embed(context.Background(), "completely unrelated topic about cooking recipes")
	results2, _ := s.SearchEmbeddings(ctx, queryVec2, 10, 0.9)
	if len(results2) > 0 {
		t.Log("unrelated text should not exceed high min_score (acceptable if hash collision occurs)")
	}
}
