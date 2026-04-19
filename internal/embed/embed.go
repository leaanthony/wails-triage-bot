// Package embed wraps OpenAI-compatible embeddings with token-accurate chunking
// and mean-pooling so callers get one 1536-dim unit vector per input, no matter
// how long.
package embed

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/pkoukk/tiktoken-go"
	openai "github.com/sashabaranov/go-openai"
)

const (
	Model         = openai.SmallEmbedding3
	Dim           = 1536
	ChunkTokens   = 6000
	ChunkOverlap  = 200
	TokenEncoding = "cl100k_base"
)

type Embedder struct {
	oa  *openai.Client
	enc *tiktoken.Tiktoken
}

func New(oa *openai.Client) (*Embedder, error) {
	enc, err := tiktoken.GetEncoding(TokenEncoding)
	if err != nil {
		return nil, fmt.Errorf("load tokenizer: %w", err)
	}
	return &Embedder{oa: oa, enc: enc}, nil
}

// Embed returns a 1536-dim unit-normalized vector for s and the total tokens used.
func (e *Embedder) Embed(ctx context.Context, s string) ([]float32, int, error) {
	chunks := Chunk(e.enc, s, ChunkTokens, ChunkOverlap)
	if len(chunks) == 0 {
		return nil, 0, errors.New("no chunks to embed")
	}
	resp, err := e.oa.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Model: Model,
		Input: chunks,
	})
	if err != nil {
		return nil, 0, err
	}
	if len(resp.Data) != len(chunks) {
		return nil, 0, fmt.Errorf("embedding response has %d items, want %d", len(resp.Data), len(chunks))
	}
	pooled := make([]float32, Dim)
	for _, d := range resp.Data {
		if len(d.Embedding) != Dim {
			return nil, 0, fmt.Errorf("embedding dim %d, want %d", len(d.Embedding), Dim)
		}
		for i, v := range d.Embedding {
			pooled[i] += v
		}
	}
	inv := float32(1.0 / float64(len(resp.Data)))
	for i := range pooled {
		pooled[i] *= inv
	}
	var norm float64
	for _, v := range pooled {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		invN := float32(1.0 / norm)
		for i := range pooled {
			pooled[i] *= invN
		}
	}
	return pooled, resp.Usage.TotalTokens, nil
}

// Chunk splits s into token windows of max size n with the given overlap.
func Chunk(enc *tiktoken.Tiktoken, s string, n, overlap int) []string {
	toks := enc.Encode(s, nil, nil)
	if len(toks) <= n {
		return []string{s}
	}
	step := n - overlap
	if step <= 0 {
		step = n
	}
	var out []string
	for start := 0; start < len(toks); start += step {
		end := start + n
		if end > len(toks) {
			end = len(toks)
		}
		out = append(out, enc.Decode(toks[start:end]))
		if end == len(toks) {
			break
		}
	}
	return out
}
