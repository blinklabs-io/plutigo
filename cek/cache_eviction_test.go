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

// TestEd25519VerifyCacheResetOnFull fills the ed25519 verify cache to its
// limit, then verifies that one additional unique entry is still admitted
// (i.e., the cache resets rather than stopping admissions permanently).
func TestEd25519VerifyCacheResetOnFull(t *testing.T) {
	// Reset the cache to a known empty state before the test.
	ed25519VerifyCache.Lock()
	ed25519VerifyCache.values = make(map[ed25519VerifyCacheKey]bool)
	ed25519VerifyCache.Unlock()

	// Generate a fixed key pair for all synthetic entries.
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}

	// Fill the cache to exactly the limit using distinct messages.
	// We bypass the builtin machinery and insert directly so the test is fast
	// and deterministic.
	for i := 0; i < ed25519VerifyCacheLimit; i++ {
		var key ed25519VerifyCacheKey
		copy(key.publicKey[:], pubKey)
		// Construct a unique message for each entry.
		msg := [ed25519VerifyMaxCacheMsg]byte{}
		// Encode i as 4 bytes so all 4096 entries are unique.
		msg[0] = byte(i >> 24)
		msg[1] = byte(i >> 16)
		msg[2] = byte(i >> 8)
		msg[3] = byte(i)
		key.message = msg
		key.msgLen = 4
		sig := ed25519.Sign(privKey, msg[:4])
		copy(key.signature[:], sig)
		ed25519VerifyCache.Lock()
		if len(ed25519VerifyCache.values) < ed25519VerifyCacheLimit {
			ed25519VerifyCache.values[key] = true
		}
		ed25519VerifyCache.Unlock()
	}

	// Verify the cache is now at capacity.
	ed25519VerifyCache.RLock()
	cacheSize := len(ed25519VerifyCache.values)
	ed25519VerifyCache.RUnlock()
	if cacheSize != ed25519VerifyCacheLimit {
		t.Fatalf("expected cache size %d, got %d", ed25519VerifyCacheLimit, cacheSize)
	}

	// Now call verifyEd25519Signature via the builtin with a brand-new key/message.
	// After the fix, this new entry should be cached (the cache resets on full).
	newPubKey, newPrivKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate new ed25519 key: %v", err)
	}
	newMsg := []byte("post-full message")
	newSig := ed25519.Sign(newPrivKey, newMsg)

	m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, nil)
	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.VerifyEd25519Signature,
		ArgCount: 0,
		Forces:   0,
	}
	b = b.ApplyArg(&Constant{&syn.ByteString{Inner: newPubKey}})
	b = b.ApplyArg(&Constant{&syn.ByteString{Inner: newMsg}})
	b = b.ApplyArg(&Constant{&syn.ByteString{Inner: newSig}})

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}
	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected *Constant, got %T", val)
	}
	boolVal, ok := constVal.Constant.(*syn.Bool)
	if !ok {
		t.Fatalf("expected *syn.Bool, got %T", constVal.Constant)
	}
	if !boolVal.Inner {
		t.Fatal("expected valid signature to return true")
	}

	// After the fix the new entry must appear in the cache.
	// Build the key that would have been inserted.
	var lookupKey ed25519VerifyCacheKey
	copy(lookupKey.publicKey[:], newPubKey)
	copy(lookupKey.signature[:], newSig)
	copy(lookupKey.message[:], newMsg)
	lookupKey.msgLen = uint8(len(newMsg))

	ed25519VerifyCache.RLock()
	_, cached := ed25519VerifyCache.values[lookupKey]
	ed25519VerifyCache.RUnlock()

	if !cached {
		t.Fatal("new entry was not admitted to cache after cache was full — reset-on-full fix is missing")
	}
}

// TestEd25519VerifyCacheConcurrentAccess exercises the cache under concurrent
// reads and writes to catch data races.
func TestEd25519VerifyCacheConcurrentAccess(t *testing.T) {
	// Reset the cache.
	ed25519VerifyCache.Lock()
	ed25519VerifyCache.values = make(map[ed25519VerifyCacheKey]bool)
	ed25519VerifyCache.Unlock()

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
