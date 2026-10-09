// Copyright 2026 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cek

import (
	"github.com/blinklabs-io/plutigo/syn"
)

// Env is a persistent environment: a linked list of bindings, newest first.
//
// Ancestor lookup uses Myers' skew-binary jump pointers ("An applicative
// random-access stack", 1983): every node carries one extra pointer to an
// ancestor chosen so that any ancestor is reachable in O(log depth) steps.
// One pointer and a depth per node replace a per-node table of
// power-of-two ancestors, which made each binding several times larger than
// the binding itself while most environments stay shallow.
//
// A node with a nil jump was built without Extend or Machine.extendEnv (a
// literal in package-local code); lookups walk such nodes linearly until they
// reach an indexed node.
type Env[T syn.Eval] struct {
	data  Value[T]
	next  *Env[T]
	jump  *Env[T]
	depth int
}

func lookupEnv[T syn.Eval](env *Env[T], idx int) (Value[T], bool) {
	var zero Value[T]
	if idx <= 0 || env == nil {
		return zero, false
	}
	switch idx {
	case 1:
		return env.data, true
	case 2:
		env = env.next
		if env == nil {
			return zero, false
		}
		return env.data, true
	case 3:
		env = env.next
		if env == nil {
			return zero, false
		}
		env = env.next
		if env == nil {
			return zero, false
		}
		return env.data, true
	}
	env = envAncestor(env, idx-1)
	if env == nil {
		return zero, false
	}
	return env.data, true
}

// envAncestor returns the node distance links above env, or nil when the
// chain is shorter than that.
func envAncestor[T syn.Eval](env *Env[T], distance int) *Env[T] {
	for env.jump == nil {
		if distance == 0 {
			return env
		}
		env = env.next
		if env == nil {
			return nil
		}
		distance--
	}
	target := env.depth - distance
	if target < 1 {
		return nil
	}
	// Every indexed node's jump is a strict ancestor except the root's, which
	// points at itself; the root has depth 1 <= target, so the loop ends.
	for env.depth != target {
		if env.jump.depth >= target {
			env = env.jump
		} else {
			env = env.next
		}
	}
	return env
}

// initEnvLink links env below parent and picks its jump pointer. When
// parent's jump spans the same distance as parent.jump's own jump, the two
// combine into one jump of twice that distance plus one; otherwise the jump
// is a single step. The resulting jump lengths follow the skew-binary
// decomposition of the depth.
func initEnvLink[T syn.Eval](env, parent *Env[T]) {
	env.next = parent
	if parent == nil {
		env.depth = 1
		env.jump = env
		return
	}
	if parent.jump == nil {
		// Below a manually built node the depth is unknown, so stay linear.
		env.depth = 0
		env.jump = nil
		return
	}
	env.depth = parent.depth + 1
	j := parent.jump
	if j != parent && parent.depth-j.depth == j.depth-j.jump.depth {
		env.jump = j.jump
	} else {
		env.jump = parent
	}
}

func (e *Env[T]) Extend(data Value[T]) *Env[T] {
	env := &Env[T]{
		data: data,
	}
	initEnvLink(env, e)
	return env
}

func (e *Env[T]) Lookup(name int) (Value[T], bool) {
	return lookupEnv(e, name)
}
