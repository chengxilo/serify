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

package serify

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chengxilo/serify/internal/config"
	"github.com/chengxilo/serify/internal/protocol"
	"github.com/chengxilo/serify/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func exchange(t *testing.T, suite Suite, messages ...string) []map[string]any {
	t.Helper()
	var out strings.Builder
	suite.In = strings.NewReader(strings.Join(messages, "\n") + "\n")
	suite.Out = &out
	Run(suite)

	var responses []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			require.NoError(t, err, "bad JSON response %q: %v", line, err)
		}
		responses = append(responses, m)
	}
	return responses
}

func mustStatus(t *testing.T, resp map[string]any, wantOp string) {
	t.Helper()
	if resp["op"] != wantOp {
		assert.Equal(t, wantOp, resp["op"], "op: got %v want %v", resp["op"], wantOp)
	}
	if resp["status"] != "OK" {
		assert.Equal(t, "OK", resp["status"], "status: got %v want %v (error: %v)", resp["status"], "OK", resp["error"])
	}
}

// u64Schema is a minimal single-uint64-field schema used by the library unit tests.
var u64Schema = `[{"name":"user_id","type":"uint64"}]`

// fullSchema is a self-contained inline schema assigned at test startup.
var fullSchema string

func TestMain(m *testing.M) {
	// Verify examples/cases parse cleanly; fullSchema is a self-contained inline
	// schema so the library tests below don't depend on the exact example suite layout.
	if _, err := config.LoadSuite(filepath.Join(testutil.RepoRoot(), "examples", "cases")); err != nil {
		panic("load cases: " + err.Error())
	}
	fullSchema = `[{"name":"user_id","type":"uint64"},{"name":"username","type":"string"},{"name":"score","type":"float32"},{"name":"active","type":"bool"},{"name":"metadata","type":"bytes"},{"name":"tags","type":"list<string>"},{"name":"profile","type":"optional<string>"},{"name":"counts","type":"array<uint32,4>"},{"name":"address","type":"struct","fields":[{"name":"street","type":"string"},{"name":"zip","type":"uint32"}]},{"name":"scores","type":"map<string,uint32>","key_type":"string"},{"name":"labels","type":"map<string,struct>","key_type":"string","fields":[{"name":"value","type":"string"},{"name":"priority","type":"uint32"}]}]`
	os.Exit(m.Run())
}

type u64Record struct {
	UserID uint64 `serify:"user_id"`
}

func (u *u64Record) toBinary() ([]byte, error) {
	return binary.LittleEndian.AppendUint64(nil, u.UserID), nil
}

func (u *u64Record) fromBinary(b []byte) error {
	if len(b) < 8 {
		return errors.New("too short")
	}
	u.UserID = binary.LittleEndian.Uint64(b[:8])
	return nil
}

func u64Suite() Suite {
	return Suite{
		Types: map[string]Type{
			"u64rec": {
				Model: &u64Record{},
				Formats: map[string]Format{
					"binary": {Serializer: (*u64Record).toBinary, Deserializer: (*u64Record).fromBinary},
				},
			},
		},
	}
}

func appendLenStr(buf []byte, s string) []byte {
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(s)))
	return append(buf, s...)
}

func readLenStr(b []byte) (string, []byte, error) {
	if len(b) < 4 {
		return "", b, errors.New("truncated length prefix")
	}
	n := int(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	if len(b) < n {
		return "", b, errors.New("truncated string")
	}
	return string(b[:n]), b[n:], nil
}

func (u *testRecord) toBinary() ([]byte, error) {
	var buf []byte
	buf = binary.LittleEndian.AppendUint64(buf, u.UserID)
	buf = appendLenStr(buf, u.Username)
	buf = binary.LittleEndian.AppendUint32(buf, math.Float32bits(u.Score))
	if u.Active {
		buf = append(buf, 1)
	} else {
		buf = append(buf, 0)
	}
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(u.Metadata)))
	buf = append(buf, u.Metadata...)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(u.Tags)))
	for _, s := range u.Tags {
		buf = appendLenStr(buf, s)
	}
	if u.Profile == nil {
		buf = append(buf, 0)
	} else {
		buf = append(buf, 1)
		buf = appendLenStr(buf, *u.Profile)
	}
	for _, n := range u.Counts {
		buf = binary.LittleEndian.AppendUint32(buf, n)
	}
	buf = appendLenStr(buf, u.Address.Street)
	buf = binary.LittleEndian.AppendUint32(buf, u.Address.Zip)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(u.Scores)))
	for k, v := range u.Scores {
		buf = appendLenStr(buf, k)
		buf = binary.LittleEndian.AppendUint32(buf, v)
	}
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(u.Labels)))
	for k, v := range u.Labels {
		buf = appendLenStr(buf, k)
		buf = appendLenStr(buf, v.Value)
		buf = binary.LittleEndian.AppendUint32(buf, v.Priority)
	}
	return buf, nil
}

func (u *testRecord) fromBinary(b []byte) error {
	var err error
	if len(b) < 8 {
		return errors.New("truncated")
	}
	u.UserID = binary.LittleEndian.Uint64(b[:8])
	b = b[8:]
	if u.Username, b, err = readLenStr(b); err != nil {
		return err
	}
	if len(b) < 4 {
		return errors.New("truncated")
	}
	u.Score = math.Float32frombits(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	if len(b) < 1 {
		return errors.New("truncated")
	}
	u.Active = b[0] != 0
	b = b[1:]
	if len(b) < 4 {
		return errors.New("truncated")
	}
	n := int(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	if len(b) < n {
		return errors.New("truncated metadata")
	}
	u.Metadata = make([]byte, n)
	copy(u.Metadata, b[:n])
	b = b[n:]
	if len(b) < 4 {
		return errors.New("truncated")
	}
	listLen := int(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	u.Tags = make([]string, listLen)
	for i := range u.Tags {
		if u.Tags[i], b, err = readLenStr(b); err != nil {
			return err
		}
	}
	if len(b) < 1 {
		return errors.New("truncated")
	}
	hasProfile := b[0] != 0
	b = b[1:]
	if hasProfile {
		var s string
		if s, b, err = readLenStr(b); err != nil {
			return err
		}
		u.Profile = &s
	}
	for i := range u.Counts {
		if len(b) < 4 {
			return errors.New("truncated")
		}
		u.Counts[i] = binary.LittleEndian.Uint32(b[:4])
		b = b[4:]
	}
	if u.Address.Street, b, err = readLenStr(b); err != nil {
		return err
	}
	if len(b) < 4 {
		return errors.New("truncated")
	}
	u.Address.Zip = binary.LittleEndian.Uint32(b[:4])
	b = b[4:]
	if len(b) < 4 {
		return errors.New("truncated")
	}
	scoresLen := int(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	u.Scores = make(map[string]uint32, scoresLen)
	for range scoresLen {
		var k string
		if k, b, err = readLenStr(b); err != nil {
			return err
		}
		if len(b) < 4 {
			return errors.New("truncated")
		}
		u.Scores[k] = binary.LittleEndian.Uint32(b[:4])
		b = b[4:]
	}
	if len(b) < 4 {
		return errors.New("truncated")
	}
	labelsLen := int(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	u.Labels = make(map[string]labelFields, labelsLen)
	for range labelsLen {
		var k, val string
		if k, b, err = readLenStr(b); err != nil {
			return err
		}
		if val, b, err = readLenStr(b); err != nil {
			return err
		}
		if len(b) < 4 {
			return errors.New("truncated")
		}
		u.Labels[k] = labelFields{Value: val, Priority: binary.LittleEndian.Uint32(b[:4])}
		b = b[4:]
	}
	return nil
}

func fullSuite() Suite {
	return Suite{
		Types: map[string]Type{
			"record": {
				Model: &testRecord{},
				Formats: map[string]Format{
					"binary": {Serializer: (*testRecord).toBinary, Deserializer: (*testRecord).fromBinary},
				},
			},
		},
	}
}

func TestRun_Init(t *testing.T) {
	resps := exchange(t, u64Suite(),
		`{"op":"bind","schema":`+u64Schema+`,"type":"u64rec","format":"binary"}`,
	)
	require.Len(t, resps, 1, "expected 1 response, got %d", len(resps))
	assert.Equal(t, "bind", resps[0]["op"])
}

func TestRun_UnknownOp(t *testing.T) {
	// An unknown op must be answered with an ERROR response, not dropped:
	// a silently dropped request leaves the runner waiting until its timeout.
	resps := exchange(t, u64Suite(),
		`{"op":"bind","schema":`+u64Schema+`,"type":"u64rec","format":"binary"}`,
		`{"op":"bogus","id":"x"}`,
	)
	require.Len(t, resps, 2, "expected 2 responses (bind + unknown-op error), got %d: %v", len(resps), resps)
	assert.Equal(t, string(protocol.StatusError), resps[1]["status"], "status: %v, want ERROR", resps[1]["status"])
	assert.Equal(t, "x", resps[1]["id"], "id: %v, want x", resps[1]["id"])
}

func TestRun_RoundTrip_U64(t *testing.T) {
	resps := exchange(t, u64Suite(),
		`{"op":"bind","schema":`+u64Schema+`,"type":"u64rec","format":"binary"}`,
		`{"op":"serialize","id":"s1","data":{"user_id":"42"}}`,
		`{"op":"serialize","id":"s2","data":{"user_id":"18446744073709551615"}}`,
	)
	require.Len(t, resps, 3, "expected 3 responses, got %d", len(resps))
	mustStatus(t, resps[1], "serialize")
	mustStatus(t, resps[2], "serialize")

	hex42 := resps[1]["hex"].(string)
	hexMax := resps[2]["hex"].(string)

	resps2 := exchange(t, u64Suite(),
		`{"op":"bind","schema":`+u64Schema+`,"type":"u64rec","format":"binary"}`,
		`{"op":"deserialize","id":"d1","hex":"`+hex42+`"}`,
		`{"op":"deserialize","id":"d2","hex":"`+hexMax+`"}`,
	)
	mustStatus(t, resps2[1], "deserialize")
	mustStatus(t, resps2[2], "deserialize")

	data1 := resps2[1]["data"].(map[string]any)
	assert.Equal(t, "42", data1["user_id"], "user_id: got %v", data1["user_id"])
	data2 := resps2[2]["data"].(map[string]any)
	assert.Equal(t, "18446744073709551615", data2["user_id"], "user_id max: got %v", data2["user_id"])
}

func TestRun_FullSchema_RoundTrip(t *testing.T) {
	suite := fullSuite()
	resps := exchange(
		t,
		suite,
		`{"op":"bind","schema":`+fullSchema+`,"type":"record","format":"binary"}`,
		`{"op":"serialize","id":"s1","data":{"user_id":"1","username":"Alice","score":"3e440000","active":true,"metadata":"deadbeef","tags":["admin","user"],"profile":"bio","counts":[1,2,3,4],"address":{"street":"Main St","zip":10001}}}`,
	)
	require.Len(t, resps, 2, "expected 2 responses, got %d: %v", len(resps), resps)
	mustStatus(t, resps[1], "serialize")

	hexVal := resps[1]["hex"].(string)

	resps2 := exchange(t, suite,
		`{"op":"bind","schema":`+fullSchema+`,"type":"record","format":"binary"}`,
		`{"op":"deserialize","id":"d1","hex":"`+hexVal+`"}`,
	)
	mustStatus(t, resps2[1], "deserialize")
	data := resps2[1]["data"].(map[string]any)
	assert.Equal(t, "1", data["user_id"], "user_id: got %v", data["user_id"])
	assert.Equal(t, "Alice", data["username"], "username: got %v", data["username"])
	assert.Equal(t, true, data["active"], "active: got %v", data["active"])
	tags := data["tags"].([]any)
	if len(tags) != 2 || tags[0] != "admin" {
		assert.Len(t, tags, 2, "tags: got %v", tags)
		if len(tags) >= 1 {
			assert.Equal(t, "admin", tags[0], "tags: got %v", tags)
		}
	}
	addr := data["address"].(map[string]any)
	assert.Equal(t, "Main St", addr["street"], "address.street: got %v", addr["street"])
}

func TestRun_FullSchema_UnicodeAndBoundary(t *testing.T) {
	suite := fullSuite()
	resps := exchange(
		t,
		suite,
		`{"op":"bind","schema":`+fullSchema+`,"type":"record","format":"binary"}`,
		`{"op":"serialize","id":"s1","data":{"user_id":"18446744073709551615","username":"用户名🎉","score":"00000000","active":false,"metadata":"cafebabe","tags":["中文"],"profile":null,"counts":[4294967295,0,0,0],"address":{"street":"大道","zip":99}}}`,
	)
	mustStatus(t, resps[1], "serialize")
	hexVal := resps[1]["hex"].(string)

	resps2 := exchange(t, suite,
		`{"op":"bind","schema":`+fullSchema+`,"type":"record","format":"binary"}`,
		`{"op":"deserialize","id":"d1","hex":"`+hexVal+`"}`,
	)
	mustStatus(t, resps2[1], "deserialize")
	data := resps2[1]["data"].(map[string]any)
	assert.Equal(t, "18446744073709551615", data["user_id"], "uint64 max: got %v", data["user_id"])
	assert.Equal(t, "用户名🎉", data["username"], "username unicode: got %v", data["username"])
	addr := data["address"].(map[string]any)
	assert.Equal(t, float64(99), addr["zip"], "zip: got %v (%T)", addr["zip"], addr["zip"])
}

func TestRun_NullOptional(t *testing.T) {
	suite := fullSuite()
	resps := exchange(
		t,
		suite,
		`{"op":"bind","schema":`+fullSchema+`,"type":"record","format":"binary"}`,
		`{"op":"serialize","id":"s1","data":{"user_id":"2","username":"Bob","score":"00000000","active":false,"metadata":"","tags":[],"profile":null,"counts":[0,0,0,0],"address":{"street":"","zip":0}}}`,
	)
	mustStatus(t, resps[1], "serialize")
	hexVal := resps[1]["hex"].(string)

	resps2 := exchange(t, suite,
		`{"op":"bind","schema":`+fullSchema+`,"type":"record","format":"binary"}`,
		`{"op":"deserialize","id":"d1","hex":"`+hexVal+`"}`,
	)
	mustStatus(t, resps2[1], "deserialize")
	data := resps2[1]["data"].(map[string]any)
	assert.Nil(t, data["profile"], "profile should be null, got %v", data["profile"])
}

func TestRun_ExplicitTypeAndFormat(t *testing.T) {
	suite := Suite{
		Types: map[string]Type{
			"user": {
				Model: &u64Record{},
				Formats: map[string]Format{
					"binary": {Serializer: (*u64Record).toBinary, Deserializer: (*u64Record).fromBinary},
				},
			},
		},
	}
	resps := exchange(t, suite,
		`{"op":"bind","schema":`+u64Schema+`,"type":"user","format":"binary"}`,
		`{"op":"serialize","id":"s1","data":{"user_id":"77"}}`,
	)
	require.Len(t, resps, 2, "expected 2 responses, got %d: %v", len(resps), resps)
	mustStatus(t, resps[1], "serialize")
}

func TestRun_PingReportsProtocolVersion(t *testing.T) {
	resps := exchange(t, u64Suite(), `{"op":"ping"}`)
	require.Len(t, resps, 1, "expected 1 response, got %d", len(resps))
	assert.Equal(t, "ping", resps[0]["op"], "expected ping OK, got %v", resps[0])
	assert.Equal(t, "OK", resps[0]["status"], "expected ping OK, got %v", resps[0])
	// JSON numbers decode as float64.
	assert.Equal(t, float64(protocol.ProtocolVersion), resps[0]["protocol_version"], "protocol_version = %v, want %v", resps[0]["protocol_version"], protocol.ProtocolVersion)
}

// Ping binds nothing, so a serialize after only a ping must still fail.
func TestRun_PingDoesNotBind(t *testing.T) {
	resps := exchange(t, u64Suite(),
		`{"op":"ping"}`,
		`{"id":"t1","op":"serialize","data":{}}`,
	)
	require.Len(t, resps, 2, "expected 2 responses, got %d", len(resps))
	assert.Equal(t, string(protocol.StatusError), resps[1]["status"], "serialize after a bare ping should be an ERROR, got %v", resps[1])
}

func TestRun_UnknownType_Skipped(t *testing.T) {
	resps := exchange(t, u64Suite(),
		`{"op":"bind","schema":`+u64Schema+`,"type":"nonexistent"}`,
	)
	require.Len(t, resps, 1, "expected 1 response, got %d", len(resps))
	assert.Equal(t, "bind", resps[0]["op"], "expected bind SKIPPED, got %v", resps[0])
	assert.Equal(t, "SKIPPED", resps[0]["status"], "expected bind SKIPPED, got %v", resps[0])
}

func TestRun_UnknownFormat_Skipped(t *testing.T) {
	resps := exchange(t, u64Suite(),
		`{"op":"bind","schema":`+u64Schema+`,"type":"u64rec","format":"nonexistent"}`,
	)
	assert.Equal(t, 1, len(resps), "expected bind SKIPPED, got %v", resps)
	if len(resps) >= 1 {
		assert.Equal(t, "SKIPPED", resps[0]["status"], "expected bind SKIPPED, got %v", resps)
	}
}

type orderRecord struct {
	OrderID uint64 `serify:"order_id"`
}

func TestRun_MultipleTypes_RequiresTypeField(t *testing.T) {
	orderSer := func(o *orderRecord) ([]byte, error) {
		return binary.LittleEndian.AppendUint64(nil, o.OrderID), nil
	}
	orderDes := func(o *orderRecord, b []byte) error {
		if len(b) < 8 {
			return errors.New("too short")
		}
		o.OrderID = binary.LittleEndian.Uint64(b[:8])
		return nil
	}
	suite := Suite{
		Types: map[string]Type{
			"user": {
				Model: &u64Record{},
				Formats: map[string]Format{
					"binary": {Serializer: (*u64Record).toBinary, Deserializer: (*u64Record).fromBinary},
				},
			},
			"order": {
				Model: &orderRecord{},
				Formats: map[string]Format{
					"binary": {Serializer: orderSer, Deserializer: orderDes},
				},
			},
		},
	}

	// Bind without a type field always errors (type is required).
	resps := exchange(t, suite,
		`{"op":"bind","schema":[{"name":"user_id","type":"uint64"}]}`,
	)
	assert.Equal(t, string(protocol.StatusError), resps[0]["status"], "expected ERROR for missing type, got %v", resps[0])

	// Bind with explicit type and format works
	resps2 := exchange(t, suite,
		`{"op":"bind","schema":[{"name":"user_id","type":"uint64"}],"type":"user","format":"binary"}`,
		`{"op":"serialize","id":"s1","data":{"user_id":"10"}}`,
	)
	mustStatus(t, resps2[1], "serialize")
}

func TestRun_MultipleFormats(t *testing.T) {
	type jsonRec struct {
		UserID uint64 `serify:"user_id" json:"user_id"`
	}
	toJSON := func(r *jsonRec) ([]byte, error) { return json.Marshal(r) }
	toBin := func(r *jsonRec) ([]byte, error) {
		return binary.LittleEndian.AppendUint64(nil, r.UserID), nil
	}
	fromJSON := func(r *jsonRec, b []byte) error { return json.Unmarshal(b, r) }
	fromBin := func(r *jsonRec, b []byte) error {
		if len(b) < 8 {
			return errors.New("too short")
		}
		r.UserID = binary.LittleEndian.Uint64(b[:8])
		return nil
	}

	suite := Suite{
		Types: map[string]Type{
			"rec": {
				Model: &jsonRec{},
				Formats: map[string]Format{
					"json":   {Serializer: toJSON, Deserializer: fromJSON},
					"binary": {Serializer: toBin, Deserializer: fromBin},
				},
			},
		},
	}
	schema := `[{"name":"user_id","type":"uint64"}]`

	respsJSON := exchange(t, suite,
		`{"op":"bind","schema":`+schema+`,"type":"rec","format":"json"}`,
		`{"op":"serialize","id":"s1","data":{"user_id":"55"}}`,
	)
	mustStatus(t, respsJSON[1], "serialize")

	respsBin := exchange(t, suite,
		`{"op":"bind","schema":`+schema+`,"type":"rec","format":"binary"}`,
		`{"op":"serialize","id":"s1","data":{"user_id":"55"}}`,
	)
	mustStatus(t, respsBin[1], "serialize")

	// Different formats produce different bytes
	assert.NotEqual(t, respsJSON[1]["hex"], respsBin[1]["hex"], "json and binary serializer should produce different bytes")
}

func TestRun_FactoryDeserializer(t *testing.T) {
	type factoryRec struct {
		UserID uint64 `serify:"user_id"`
	}
	toB := func(r *factoryRec) ([]byte, error) {
		return binary.LittleEndian.AppendUint64(nil, r.UserID), nil
	}
	// factory variant: func([]byte) (*T, error)
	fromB := func(b []byte) (*factoryRec, error) {
		if len(b) < 8 {
			return nil, errors.New("too short")
		}
		return &factoryRec{UserID: binary.LittleEndian.Uint64(b[:8])}, nil
	}
	suite := Suite{
		Types: map[string]Type{
			"fr": {
				Model: &factoryRec{},
				Formats: map[string]Format{
					"binary": {Serializer: toB, Deserializer: fromB},
				},
			},
		},
	}
	sc := `[{"name":"user_id","type":"uint64"}]`
	resps := exchange(t, suite,
		`{"op":"bind","schema":`+sc+`,"type":"fr","format":"binary"}`,
		`{"op":"serialize","id":"s1","data":{"user_id":"123"}}`,
	)
	mustStatus(t, resps[1], "serialize")
	hexVal := resps[1]["hex"].(string)

	resps2 := exchange(t, suite,
		`{"op":"bind","schema":`+sc+`,"type":"fr","format":"binary"}`,
		`{"op":"deserialize","id":"d1","hex":"`+hexVal+`"}`,
	)
	mustStatus(t, resps2[1], "deserialize")
	assert.Equal(t, "123", resps2[1]["data"].(map[string]any)["user_id"], "factory deserialize: got %v", resps2[1]["data"])
}

// --- Type with no Model: the worker speaks FieldMap directly -----------------

// fieldMapSuite registers "u64rec" without a Model, so the format functions take
// and return the FieldMap itself — the path a type with no natural struct uses.
func fieldMapSuite(mutate bool) Suite {
	return Suite{
		Types: map[string]Type{
			"u64rec": {
				Formats: map[string]Format{
					"binary": {
						Serializer: func(fm *FieldMap) ([]byte, error) {
							v, _ := fm.GetU64("user_id")
							if mutate {
								fm.SetU64("user_id", 0)
							}
							return binary.LittleEndian.AppendUint64(nil, v), nil
						},
						Deserializer: func(b []byte) (*FieldMap, error) {
							if len(b) < 8 {
								return nil, errors.New("too short")
							}
							fm := NewFieldMap()
							fm.SetU64("user_id", binary.LittleEndian.Uint64(b[:8]))
							return fm, nil
						},
					},
				},
			},
		},
	}
}

func TestRun_NoModel_RoundTrip(t *testing.T) {
	resps := exchange(t, fieldMapSuite(false),
		`{"op":"bind","schema":`+u64Schema+`,"type":"u64rec","format":"binary"}`,
		`{"op":"serialize","id":"s1","data":{"user_id":"42"}}`,
	)
	require.Len(t, resps, 2, "expected 2 responses, got %d", len(resps))
	mustStatus(t, resps[1], "serialize")

	resps2 := exchange(t, fieldMapSuite(false),
		`{"op":"bind","schema":`+u64Schema+`,"type":"u64rec","format":"binary"}`,
		`{"op":"deserialize","id":"d1","hex":"`+resps[1]["hex"].(string)+`"}`,
	)
	mustStatus(t, resps2[1], "deserialize")
	data := resps2[1]["data"].(map[string]any)
	assert.Equal(t, "42", data["user_id"], "user_id: got %v", data["user_id"])
}

// Audit has no struct to compare on this path, so it must fall back to the
// FieldMap the worker was handed. Without that, a Model-less worker would be
// silently exempt from mutation detection.
func TestRun_NoModel_AuditSeesFieldMapMutation(t *testing.T) {
	resps := exchange(t, fieldMapSuite(true),
		`{"op":"bind","schema":`+u64Schema+`,"type":"u64rec","format":"binary","audit":true}`,
		`{"op":"serialize","id":"s1","data":{"user_id":"42"}}`,
	)
	require.Len(t, resps, 2, "expected 2 responses, got %d", len(resps))
	mustStatus(t, resps[1], "serialize")

	audit, ok := resps[1]["audit"].(map[string]any)
	require.True(t, ok, "serialize response carried no audit block: %v", resps[1])
	assert.Contains(t, audit, "mutations", "audit did not report the FieldMap mutation: %v", audit)
}

func TestRun_NoModel_RejectsModelTypedFunc(t *testing.T) {
	suite := Suite{
		Types: map[string]Type{
			"u64rec": {
				Formats: map[string]Format{
					"binary": {Serializer: (*u64Record).toBinary},
				},
			},
		},
	}
	_, _, err := buildSerializer(suite.Types["u64rec"].Model, suite.Types["u64rec"].Formats["binary"].Serializer, nil)
	require.Error(t, err, "a model-typed serializer with no Model must not be accepted")
	assert.Contains(t, err.Error(), "func(*FieldMap) ([]byte, error)", "error should name the required shape: %v", err)
}
