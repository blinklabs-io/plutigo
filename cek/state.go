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

import "github.com/blinklabs-io/plutigo/syn"

type MachineState[T syn.Eval] interface {
	isMachineState()
}

type Return[T syn.Eval] struct {
	Ctx   MachineContext[T]
	Value Value[T]
}

func (r Return[T]) isMachineState() {}

type Compute[T syn.Eval] struct {
	Ctx  MachineContext[T]
	Env  *Env[T]
	Term syn.Term[T]
}

func (c Compute[T]) isMachineState() {}

type Done[T syn.Eval] struct {
	term syn.Term[T]
}

func (d Done[T]) isMachineState() {}
