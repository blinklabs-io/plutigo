package cek

import (
	"crypto/ed25519"
	"crypto/rand"
	"sync"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// resetEd25519VerifyCache clears every shard of the ed25519 verify cache,
// returning it to a known empty state for deterministic tests.
func resetEd25519VerifyCache() {
	for i := range ed25519VerifyCache.shards {
		s := &ed25519VerifyCache.shards[i]
		s.Lock()
		s.values = make(map[ed25519VerifyCacheKey]bool, ed25519VerifyShardLimit)
		s.Unlock()
	}
}

// TestEd25519VerifyCacheResetOnFull drives a single cache shard to its limit,
// then verifies that one additional unique entry mapping to that same shard is
// still admitted (i.e., a full shard resets rather than refusing admissions
// permanently). With sharded storage the new entry must be steered to a shard
// known to be full, otherwise admission would prove nothing about reset-on-full.
func TestEd25519VerifyCacheResetOnFull(t *testing.T) {
	// Reset the cache to a known empty state before the test.
	resetEd25519VerifyCache()

	// Pick a target shard and synthesize distinct keys that all hash to it, so
	// we can fill exactly that shard and observe reset-on-full deterministically.
	target := &ed25519VerifyCache.shards[0]
	keys := make([]ed25519VerifyCacheKey, 0, ed25519VerifyShardLimit+1)
	for i := 0; len(keys) < ed25519VerifyShardLimit+1; i++ {
		var key ed25519VerifyCacheKey
		// Vary the bytes the shard hash consumes so keys spread across shards.
		key.publicKey[0] = byte(i)
		key.publicKey[1] = byte(i >> 8)
		key.signature[0] = byte(i >> 16)
		key.signature[1] = byte(i >> 24)
		if ed25519VerifyCache.shard(&key) == target {
			keys = append(keys, key)
		}
	}

	// Fill the target shard exactly to its limit.
	for i := 0; i < ed25519VerifyShardLimit; i++ {
		ed25519VerifyCache.store(&keys[i], true)
	}

	target.Lock()
	full := len(target.values)
	target.Unlock()
	if full != ed25519VerifyShardLimit {
		t.Fatalf("expected shard at capacity %d, got %d", ed25519VerifyShardLimit, full)
	}

	// Storing a brand-new key for the now-full shard must still be admitted:
	// reset-on-full clears the shard rather than refusing entries forever.
	newKey := keys[ed25519VerifyShardLimit]
	ed25519VerifyCache.store(&newKey, true)
	if _, cached := ed25519VerifyCache.get(&newKey); !cached {
		t.Fatal("new entry was not admitted after shard was full — reset-on-full fix is missing")
	}
}

// TestEd25519VerifyCacheConcurrentAccess exercises the cache under concurrent
// reads and writes to catch data races.
func TestEd25519VerifyCacheConcurrentAccess(t *testing.T) {
	// Reset the cache.
	resetEd25519VerifyCache()

	const goroutines = 16
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				// Can't call t.Fatal from goroutine; just return.
				return
			}
			m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, nil)
			for i := 0; i < 300; i++ {
				msg := make([]byte, 8)
				msg[0] = byte(g)
				msg[1] = byte(i)
				sig := ed25519.Sign(privKey, msg)

				b := &Builtin[syn.DeBruijn]{
					Func:     builtin.VerifyEd25519Signature,
					ArgCount: 0,
					Forces:   0,
				}
				b = b.ApplyArg(&Constant{&syn.ByteString{Inner: pubKey}})
				b = b.ApplyArg(&Constant{&syn.ByteString{Inner: msg}})
				b = b.ApplyArg(&Constant{&syn.ByteString{Inner: sig}})
				_, _ = m.evalBuiltinApp(b) //nolint:errcheck
			}
		}()
	}
	wg.Wait()
}

// TestSharedDynamicIntCacheResetOnFull fills the sharedDynamicInts cache to
// its limit, then verifies that a subsequent insert is admitted (reset-on-full).
func TestSharedDynamicIntCacheResetOnFull(t *testing.T) {
	// Reset the shared dynamic int cache to a known empty state.
	sharedDynamicIntMu.Lock()
	sharedDynamicInts = make(map[int64]*Constant, int64ConstantCacheCap)
	sharedDynamicIntMu.Unlock()

	// Fill the cache using values outside the small-int range.
	// We start at cachedIntMax+1 to avoid the static cache range.
	base := int64(cachedIntMax) + 1
	for i := 0; i < int64ConstantCacheCap; i++ {
		v := base + int64(i)
		c := newDynamicIntConstant(v)
		storeSharedDynamicIntConstant(v, c)
	}

	// Verify the cache is at capacity.
	sharedDynamicIntMu.RLock()
	size := len(sharedDynamicInts)
	sharedDynamicIntMu.RUnlock()
	if size != int64ConstantCacheCap {
		t.Fatalf("expected cache size %d, got %d", int64ConstantCacheCap, size)
	}

	// A new value beyond the filled range should be admitted after the fix.
	newVal := base + int64(int64ConstantCacheCap) + 1
	c := newDynamicIntConstant(newVal)
	stored := storeSharedDynamicIntConstant(newVal, c)
	if stored == nil {
		t.Fatal("new entry was not admitted to sharedDynamicInts after cache was full — reset-on-full fix is missing")
	}

	// Verify it is now retrievable from the shared cache.
	got := loadSharedDynamicIntConstant(newVal)
	if got == nil {
		t.Fatal("new entry not found in sharedDynamicInts after reset-on-full insert")
	}
}

// TestSharedDynamicIntCacheConcurrentAccess exercises sharedDynamicInts under
// concurrent access to catch data races.
func TestSharedDynamicIntCacheConcurrentAccess(t *testing.T) {
	// Reset.
	sharedDynamicIntMu.Lock()
	sharedDynamicInts = make(map[int64]*Constant, int64ConstantCacheCap)
	sharedDynamicIntMu.Unlock()

	const goroutines = 16
	var wg sync.WaitGroup
	wg.Add(goroutines)

	base := int64(cachedIntMax) + 1

	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, nil)
			for i := 0; i < 500; i++ {
				v := base + int64(g*500+i)
				_ = m.int64Constant(v)
			}
		}()
	}
	wg.Wait()
}
