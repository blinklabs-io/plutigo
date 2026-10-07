package cek

import (
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func envValueInt(t *testing.T, value Value[syn.DeBruijn]) int64 {
	t.Helper()
	constant, ok := value.(*Constant)
	if !ok {
		t.Fatalf("value = %T, want *Constant", value)
	}
	integer, ok := constant.Constant.(*syn.Integer)
	if !ok {
		t.Fatalf("constant = %T, want *syn.Integer", constant.Constant)
	}
	return integer.Inner.Int64()
}

// Every index at every depth must resolve to the binding made idx-1
// extensions earlier, through both the Machine arena and the exported API,
// and both lookup entry points.
func TestEnvLookupEveryIndexEveryDepth(t *testing.T) {
	const maxDepth = 1100

	machine := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, testEvalContext())
	var arenaEnv, heapEnv *Env[syn.DeBruijn]
	for depth := 1; depth <= maxDepth; depth++ {
		value := int64Constant(int64(depth))
		arenaEnv = machine.extendEnv(arenaEnv, value)
		heapEnv = heapEnv.Extend(value)
		for _, env := range []*Env[syn.DeBruijn]{arenaEnv, heapEnv} {
			for idx := 1; idx <= depth; idx++ {
				want := int64(depth - idx + 1)
				got, ok := lookupEnv(env, idx)
				if !ok || envValueInt(t, got) != want {
					t.Fatalf("depth %d: lookupEnv(%d) = %v, %v; want %d", depth, idx, got, ok, want)
				}
				got, ok = lookupEnvDeBruijn(env, idx)
				if !ok || envValueInt(t, got) != want {
					t.Fatalf("depth %d: lookupEnvDeBruijn(%d) = %v, %v; want %d", depth, idx, got, ok, want)
				}
			}
			if _, ok := lookupEnv(env, depth+1); ok {
				t.Fatalf("depth %d: lookupEnv(%d) succeeded beyond depth", depth, depth+1)
			}
			if _, ok := lookupEnvDeBruijn(env, depth+1); ok {
				t.Fatalf("depth %d: lookupEnvDeBruijn(%d) succeeded beyond depth", depth, depth+1)
			}
		}
	}
}

// Literal Env nodes carry no jump pointer. Chains that mix them with indexed
// nodes, in either order, must still resolve every index.
func TestEnvLookupMixedLiteralAndIndexedNodes(t *testing.T) {
	var indexed *Env[syn.DeBruijn]
	for i := int64(1); i <= 20; i++ {
		indexed = indexed.Extend(int64Constant(i))
	}
	literalOverIndexed := indexed
	for i := int64(21); i <= 25; i++ {
		literalOverIndexed = &Env[syn.DeBruijn]{
			data: int64Constant(i),
			next: literalOverIndexed,
		}
	}
	extendedOverLiteral := literalOverIndexed
	for i := int64(26); i <= 40; i++ {
		extendedOverLiteral = extendedOverLiteral.Extend(int64Constant(i))
	}

	for _, tc := range []struct {
		name  string
		env   *Env[syn.DeBruijn]
		depth int
	}{
		{"literal over indexed", literalOverIndexed, 25},
		{"extended over literal", extendedOverLiteral, 40},
	} {
		for idx := 1; idx <= tc.depth; idx++ {
			got, ok := tc.env.Lookup(idx)
			want := int64(tc.depth - idx + 1)
			if !ok || envValueInt(t, got) != want {
				t.Fatalf("%s: Lookup(%d) = %v, %v; want %d", tc.name, idx, got, ok, want)
			}
		}
		if _, ok := tc.env.Lookup(tc.depth + 1); ok {
			t.Fatalf("%s: Lookup(%d) succeeded beyond depth", tc.name, tc.depth+1)
		}
	}
}

// Chunks a Machine releases to envChunkPool must hold no pointers into the
// evaluation that used them, or a later Machine would read stale bindings and
// the pool would pin the old graph.
func TestReleasedEnvChunksAreCleared(t *testing.T) {
	released := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, testEvalContext())
	var env *Env[syn.DeBruijn]
	for i := range envFirstChunkSize * 8 {
		env = released.extendEnv(env, int64Constant(int64(i)))
	}
	released.dropEnvArena()

	reused := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, testEvalContext())
	reused.takePooledEnvChunks()
	if len(reused.envChunks) == 0 {
		t.Skip("envChunkPool was emptied by a GC cycle")
	}
	for ci, chunk := range reused.envChunks {
		for si, slot := range chunk {
			if slot != (Env[syn.DeBruijn]{}) {
				t.Fatalf("pooled envChunks[%d][%d] = %+v, want zero", ci, si, slot)
			}
		}
	}
}
