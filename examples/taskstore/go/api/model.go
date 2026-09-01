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

package api

import (
	"encoding/binary"
)

// The records this server stores and sends. Each mirrors the case file of the
// same name, and each owns its own byte layout — the layout is a property of
// the type, not of some central encoder that has to know about all of them.
//
// The `serify:"…"` tags are the schema binding. They cost this package nothing
// at run time and, more to the point, they cost it no dependency: struct tags
// are inert strings, so `api` imports the standard library and nothing else.
// The conformance worker in ../main.go is the only file here that links serify.

// Priority is the urgency of a task: the `enum<low, normal, high>` in
// cases/task.yaml. It is a plain string because that is what an enum travels
// as; priorityVariants below fixes the ordinals the bytes use.
const (
	PriorityLow    = "low"
	PriorityNormal = "normal"
	PriorityHigh   = "high"
)

// priorityVariants is the declaration order of the enum in cases/task.yaml and
// cases/draft.yaml. Position is the ordinal on the wire.
var priorityVariants = []string{PriorityLow, PriorityNormal, PriorityHigh}

// Error codes: the `enum<not_found, invalid, conflict>` in cases/api_error.yaml.
const (
	CodeNotFound = "not_found"
	CodeInvalid  = "invalid"
	CodeConflict = "conflict"
)

var codeVariants = []string{CodeNotFound, CodeInvalid, CodeConflict}

// Task is a stored task, mirroring cases/task.yaml.
type Task struct {
	ID       uint64   `serify:"id"`
	Title    string   `serify:"title"`
	Done     bool     `serify:"done"`
	Priority string   `serify:"priority"`
	Tags     []string `serify:"tags"`
	DueAt    *int64   `serify:"due_at"` // optional<int64>, unix seconds
}

// Draft is a task the server has not assigned an id to yet: cases/draft.yaml.
type Draft struct {
	Title    string   `serify:"title"`
	Priority string   `serify:"priority"`
	Tags     []string `serify:"tags"`
	DueAt    *int64   `serify:"due_at"`
}

// TaskPage is the body of a list response: cases/task_page.yaml.
type TaskPage struct {
	Items []Task `serify:"items"`
	Total uint32 `serify:"total"`
}

// APIError is a failed request: cases/api_error.yaml.
type APIError struct {
	Code    string `serify:"code"`
	Message string `serify:"message"`
}

// Error lets a handler return an APIError as an ordinary Go error and have the
// server turn it back into a `failed` result. See store.Store.Apply.
func (e APIError) Error() string { return e.Code + ": " + e.Message }

func appendTags(buf []byte, tags []string) []byte {
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(tags)))
	for _, t := range tags {
		buf = appendString(buf, t)
	}
	return buf
}

func readTags(b []byte) ([]string, []byte, error) {
	n, b, err := readU32(b)
	if err != nil {
		return nil, b, err
	}
	tags := make([]string, 0, min(int(n), 1024))
	for range n {
		var t string
		if t, b, err = readString(b); err != nil {
			return nil, b, err
		}
		tags = append(tags, t)
	}
	return tags, b, nil
}

func appendDueAt(buf []byte, due *int64) []byte {
	if due == nil {
		return append(buf, 0)
	}
	buf = append(buf, 1)
	return binary.LittleEndian.AppendUint64(buf, uint64(*due))
}

func readDueAt(b []byte) (*int64, []byte, error) {
	present, b, err := readU8(b)
	if err != nil || present == 0 {
		return nil, b, err
	}
	raw, b, err := readU64(b)
	if err != nil {
		return nil, b, err
	}
	due := int64(raw)
	return &due, b, nil
}

func appendTask(buf []byte, t Task) ([]byte, error) {
	buf = binary.LittleEndian.AppendUint64(buf, t.ID)
	buf = appendString(buf, t.Title)
	buf = appendBool(buf, t.Done)
	buf, err := appendEnum(buf, priorityVariants, t.Priority)
	if err != nil {
		return nil, err
	}
	buf = appendTags(buf, t.Tags)
	return appendDueAt(buf, t.DueAt), nil
}

func readTask(b []byte) (Task, []byte, error) {
	var t Task
	var err error
	if t.ID, b, err = readU64(b); err != nil {
		return t, b, err
	}
	if t.Title, b, err = readString(b); err != nil {
		return t, b, err
	}
	var done uint8
	if done, b, err = readU8(b); err != nil {
		return t, b, err
	}
	t.Done = done != 0
	if t.Priority, b, err = readEnum(b, priorityVariants); err != nil {
		return t, b, err
	}
	if t.Tags, b, err = readTags(b); err != nil {
		return t, b, err
	}
	t.DueAt, b, err = readDueAt(b)
	return t, b, err
}

func appendDraft(buf []byte, d Draft) ([]byte, error) {
	buf = appendString(buf, d.Title)
	buf, err := appendEnum(buf, priorityVariants, d.Priority)
	if err != nil {
		return nil, err
	}
	buf = appendTags(buf, d.Tags)
	return appendDueAt(buf, d.DueAt), nil
}

func readDraft(b []byte) (Draft, []byte, error) {
	var d Draft
	var err error
	if d.Title, b, err = readString(b); err != nil {
		return d, b, err
	}
	if d.Priority, b, err = readEnum(b, priorityVariants); err != nil {
		return d, b, err
	}
	if d.Tags, b, err = readTags(b); err != nil {
		return d, b, err
	}
	d.DueAt, b, err = readDueAt(b)
	return d, b, err
}

func appendPage(buf []byte, p TaskPage) ([]byte, error) {
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(p.Items)))
	for _, t := range p.Items {
		var err error
		if buf, err = appendTask(buf, t); err != nil {
			return nil, err
		}
	}
	return binary.LittleEndian.AppendUint32(buf, p.Total), nil
}

func readPage(b []byte) (TaskPage, []byte, error) {
	var p TaskPage
	n, b, err := readU32(b)
	if err != nil {
		return p, b, err
	}
	p.Items = make([]Task, 0, min(int(n), 1024))
	for range n {
		var t Task
		if t, b, err = readTask(b); err != nil {
			return p, b, err
		}
		p.Items = append(p.Items, t)
	}
	p.Total, b, err = readU32(b)
	return p, b, err
}

func appendAPIError(buf []byte, e APIError) ([]byte, error) {
	buf, err := appendEnum(buf, codeVariants, e.Code)
	if err != nil {
		return nil, err
	}
	return appendString(buf, e.Message), nil
}

func readAPIError(b []byte) (APIError, []byte, error) {
	var e APIError
	var err error
	if e.Code, b, err = readEnum(b, codeVariants); err != nil {
		return e, b, err
	}
	e.Message, b, err = readString(b)
	return e, b, err
}

// ValidPriority reports whether p is one of the declared priorities. The server
// checks it before storing, so a bad value comes back as an api_error the client
// can read rather than a marshal failure it cannot.
func ValidPriority(p string) bool {
	for _, v := range priorityVariants {
		if v == p {
			return true
		}
	}
	return false
}
