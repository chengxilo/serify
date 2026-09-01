// Copyright 2026 Chengxi Luo
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

package store

import (
	"fmt"
	"sync"

	"github.com/chengxilo/serify/example-taskstore/api"
)

// PageLimit caps how many tasks one listing returns. The response carries the
// full count separately, so a client can tell it did not get everything.
const PageLimit = 50

// FirstID is where ids start. Well above zero, so a zero id in a request is
// visibly a client that forgot to set one rather than a plausible task.
const FirstID = 1001

// Store is the server's state: tasks by id, plus the order they were created
// in, because a listing that comes back in a different order every time is not
// something a client can page through.
type Store struct {
	mu     sync.Mutex
	tasks  map[uint64]api.Task
	order  []uint64
	nextID uint64
}

func New() *Store {
	return &Store{tasks: make(map[uint64]api.Task), nextID: FirstID}
}

// Apply runs one request and returns the response to send back. It is the
// entire server: everything else is sockets and bytes.
//
// Note what the signature does not have — no error return. Every way this can
// go wrong is an api.Failed the client is meant to read, so the failure modes
// are in the protocol rather than beside it.
func (s *Store) Apply(req api.Request) api.Response {
	s.mu.Lock()
	defer s.mu.Unlock()

	reply := func(r api.Result) api.Response {
		return api.Response{RequestID: req.RequestID, Result: r}
	}
	failf := func(code, format string, args ...any) api.Response {
		return reply(api.Failed{Code: code, Message: fmt.Sprintf(format, args...)})
	}

	switch op := req.Op.(type) {
	case api.ListAll:
		items := make([]api.Task, 0, min(len(s.order), PageLimit))
		for _, id := range s.order {
			if len(items) == PageLimit {
				break
			}
			items = append(items, s.tasks[id])
		}
		return reply(api.Listing{Items: items, Total: uint32(len(s.order))})

	case api.Create:
		draft := api.Draft(op)
		// An empty title is checked here because the wire can carry one: a
		// zero-length string is a perfectly good string. A bad *priority* is not
		// checked, because an enum travels as an ordinal into a fixed list — a
		// value outside it has no encoding, so it cannot arrive. Validating it
		// again here would be dead code that reads like a safety net.
		if draft.Title == "" {
			return failf(api.CodeInvalid, "a task needs a title")
		}
		task := api.Task{
			ID:       s.nextID,
			Title:    draft.Title,
			Priority: draft.Priority,
			Tags:     draft.Tags,
			DueAt:    draft.DueAt,
		}
		s.nextID++
		s.tasks[task.ID] = task
		s.order = append(s.order, task.ID)
		return reply(api.Found(task))

	case api.Read:
		task, ok := s.tasks[uint64(op)]
		if !ok {
			return failf(api.CodeNotFound, "no task with id %d", uint64(op))
		}
		return reply(api.Found(task))

	case api.Update:
		task := api.Task(op)
		if _, ok := s.tasks[task.ID]; !ok {
			return failf(api.CodeNotFound, "no task with id %d", task.ID)
		}
		if task.Title == "" {
			return failf(api.CodeInvalid, "a task needs a title")
		}
		s.tasks[task.ID] = task
		return reply(api.Found(task))

	case api.Delete:
		id := uint64(op)
		if _, ok := s.tasks[id]; !ok {
			return failf(api.CodeNotFound, "no task with id %d", id)
		}
		delete(s.tasks, id)
		for i, existing := range s.order {
			if existing == id {
				s.order = append(s.order[:i], s.order[i+1:]...)
				break
			}
		}
		return reply(api.Accepted{})

	default:
		// Unreachable while Op stays sealed, but a sealed interface is not an
		// exhaustive one: this is what a sixth operation nobody wired up here
		// would hit.
		return failf(api.CodeInvalid, "unsupported operation %T", req.Op)
	}
}
