package embed

import (
	"strings"
	"testing"

	"github.com/pkoukk/tiktoken-go"
)

func mustEnc(t *testing.T) *tiktoken.Tiktoken {
	t.Helper()
	enc, err := tiktoken.GetEncoding(TokenEncoding)
	if err != nil {
		t.Skipf("tokenizer unavailable: %v", err)
	}
	return enc
}

func TestChunkShortReturnsOriginal(t *testing.T) {
	enc := mustEnc(t)
	s := "hello world"
	out := Chunk(enc, s, 100, 10)
	if len(out) != 1 || out[0] != s {
		t.Errorf("got %v", out)
	}
}

func TestChunkEmpty(t *testing.T) {
	enc := mustEnc(t)
	out := Chunk(enc, "", 100, 10)
	if len(out) != 1 || out[0] != "" {
		t.Errorf("got %v", out)
	}
}

func TestChunkLongSplits(t *testing.T) {
	enc := mustEnc(t)
	s := strings.Repeat("token ", 5000) // ~5000+ toks
	out := Chunk(enc, s, 1000, 100)
	if len(out) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(out))
	}
	// Each chunk must be non-empty and <= window in tokens.
	for i, c := range out {
		if c == "" {
			t.Errorf("chunk %d empty", i)
		}
		if n := len(enc.Encode(c, nil, nil)); n > 1000 {
			t.Errorf("chunk %d has %d toks > 1000", i, n)
		}
	}
}

func TestChunkOverlapAdvances(t *testing.T) {
	enc := mustEnc(t)
	s := strings.Repeat("word ", 3000)
	// overlap < n → step > 0, must terminate.
	out := Chunk(enc, s, 500, 50)
	if len(out) < 2 {
		t.Fatalf("expected splits, got %d", len(out))
	}
}

func TestChunkOverlapGteN(t *testing.T) {
	enc := mustEnc(t)
	s := strings.Repeat("word ", 3000)
	// overlap >= n triggers step=n fallback; must still terminate.
	out := Chunk(enc, s, 500, 500)
	if len(out) == 0 {
		t.Fatal("no output")
	}
}

func TestConstants(t *testing.T) {
	if Dim != 1536 {
		t.Errorf("Dim=%d", Dim)
	}
	if ChunkTokens <= ChunkOverlap {
		t.Errorf("ChunkTokens=%d must exceed ChunkOverlap=%d", ChunkTokens, ChunkOverlap)
	}
}
