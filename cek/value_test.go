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
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/syn"
)

func TestBoolConstantReturnsDistinctValues(t *testing.T) {
	first := boolConstant(true)
	second := boolConstant(true)

	firstBool, ok := first.Constant.(*syn.Bool)
	if !ok || !firstBool.Inner {
		t.Fatalf("boolConstant(true) = %#v, want true bool constant", first)
	}
	secondBool, ok := second.Constant.(*syn.Bool)
	if !ok || !secondBool.Inner {
		t.Fatalf("boolConstant(true) second call = %#v, want true bool constant", second)
	}
	if first != second {
		t.Fatal("boolConstant(true) should reuse the cached constant")
	}

	falseFirst := boolConstant(false)
	falseSecond := boolConstant(false)
	falseBool, ok := falseFirst.Constant.(*syn.Bool)
	if !ok || falseBool.Inner {
		t.Fatalf("boolConstant(false) = %#v, want false bool constant", falseFirst)
	}
	if falseFirst != falseSecond {
		t.Fatal("boolConstant(false) should reuse the cached constant")
	}
}

func TestCloneConstantPreservesNilInteger(t *testing.T) {
	cloned, ok := cloneConstant(&syn.Integer{}).(*syn.Integer)
	if !ok {
		t.Fatalf("cloneConstant returned %T, want *syn.Integer", cloned)
	}
	if cloned.Inner != nil {
		t.Fatalf("cloned integer Inner = %v, want nil", cloned.Inner)
	}
	if got := cloned.ExMemWords(); got != 1 {
		t.Fatalf("cloned integer ExMemWords() = %d, want 1", got)
	}
}

func TestCloneConstantCopiesIntegerValue(t *testing.T) {
	original := &syn.Integer{}
	original.SetInner(big.NewInt(7))

	cloned, ok := cloneConstant(original).(*syn.Integer)
	if !ok {
		t.Fatalf("cloneConstant returned %T, want *syn.Integer", cloned)
	}
	if cloned.Inner == nil || cloned.Inner.Cmp(big.NewInt(7)) != 0 {
		t.Fatalf("cloned integer Inner = %v, want 7", cloned.Inner)
	}
	if cloned.Inner == original.Inner {
		t.Fatal("cloneConstant reused the original big.Int pointer")
	}
}
