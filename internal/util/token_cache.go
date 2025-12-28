package util

import (
	"hash/fnv"
	"sync"
)

const (
	numShards          = 16
	maxEntriesPerShard = 64
)

type tokenCacheEntry struct {
	hash   uint64
	tokens int
}

type tokenCacheShard struct {
	mu      sync.RWMutex
	entries []tokenCacheEntry
}

type TokenCache struct {
	shards [numShards]*tokenCacheShard
}

var (
	ToolTokenCache        = NewTokenCache()
	InstructionTokenCache = NewTokenCache()
	ContentTokenCache     = NewTokenCache()
)

func NewTokenCache() *TokenCache {
	tc := &TokenCache{}
	for i := range tc.shards {
		tc.shards[i] = &tokenCacheShard{
			entries: make([]tokenCacheEntry, 0, maxEntriesPerShard),
		}
	}
	return tc
}

func hashContent(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func (tc *TokenCache) Get(content string) (int, bool) {
	hash := hashContent(content)
	shard := tc.shards[hash%numShards]

	shard.mu.RLock()
	defer shard.mu.RUnlock()

	for _, e := range shard.entries {
		if e.hash == hash {
			return e.tokens, true
		}
	}
	return 0, false
}

func (tc *TokenCache) Set(content string, tokens int) {
	hash := hashContent(content)
	shard := tc.shards[hash%numShards]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Check if already exists
	for i, e := range shard.entries {
		if e.hash == hash {
			shard.entries[i].tokens = tokens
			return
		}
	}

	// Evict oldest if full
	if len(shard.entries) >= maxEntriesPerShard {
		shard.entries = shard.entries[1:]
	}

	shard.entries = append(shard.entries, tokenCacheEntry{hash: hash, tokens: tokens})
}
