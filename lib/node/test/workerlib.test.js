/*
 * Copyright 2026 Chengxi Luo
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Unit tests for the built library (dist/workerlib.js). Run via `npm test`.
// Plain JS on purpose: the library's own tsconfig does not enable
// experimentalDecorators (only consumers do), so the decorator functions are
// applied explicitly here.

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { FieldMap, Serify, Variant, decodeFieldMap, detectZeroCopy, encodeFieldMap } = require('../dist/workerlib.js');

function roundTrip(schema, data) {
  const fm = decodeFieldMap(data, schema);
  return { fm, out: encodeFieldMap(fm, schema) };
}

test('list<int64> decodes to bigints and re-encodes to the same strings', () => {
  const schema = [{ name: 'a', type: 'list<int64>' }];
  const data = { a: ['-9223372036854775808', '42', '9223372036854775807'] };
  const { fm, out } = roundTrip(schema, data);
  assert.deepEqual(fm.getListI64('a'), [-9223372036854775808n, 42n, 9223372036854775807n]);
  assert.deepEqual(out, data);
});

test('list<uint128> keeps full 128-bit precision', () => {
  const schema = [{ name: 'a', type: 'list<uint128>' }];
  const data = { a: ['340282366920938463463374607431768211455'] };
  const { fm, out } = roundTrip(schema, data);
  assert.deepEqual(fm.getListU128('a'), [340282366920938463463374607431768211455n]);
  assert.deepEqual(out, data);
});

test('list<uint8>, list<int32>, list<bool> round-trip', () => {
  const schema = [
    { name: 'u8s', type: 'list<uint8>' },
    { name: 'i32s', type: 'list<int32>' },
    { name: 'bools', type: 'list<bool>' },
  ];
  const data = { u8s: [0, 128, 255], i32s: [-2147483648, 0, 2147483647], bools: [true, false] };
  const { fm, out } = roundTrip(schema, data);
  assert.deepEqual(fm.getListU8('u8s'), [0, 128, 255]);
  assert.deepEqual(fm.getListI32('i32s'), [-2147483648, 0, 2147483647]);
  assert.deepEqual(fm.getListBool('bools'), [true, false]);
  assert.deepEqual(out, data);
});

test('unsupported list element type throws instead of passing through silently', () => {
  const schema = [{ name: 'a', type: 'list<bogus>' }];
  assert.throws(() => decodeFieldMap({ a: [1] }, schema), /unsupported list element type/);
});

// --- model helper (decorators applied explicitly) ---------------------------

function makeUserClass() {
  class User {
    constructor() {
      this.id = 0n;
      this.name = '';
      this.email = '';
    }
  }
  Serify.field()(User.prototype, 'id');
  Serify.field()(User.prototype, 'name');
  Serify.field({ rename: 'email_addr' })(User.prototype, 'email');
  Serify.Model()(User);
  return User;
}

test('model round-trip with rename', () => {
  const User = makeUserClass();
  const u = new User();
  u.id = 42n;
  u.name = 'Dana';
  u.email = 'dana@example.com';

  const fm = Serify.toFieldMap(u);
  assert.equal(fm.getString('name'), 'Dana');
  assert.equal(fm.getString('email_addr'), 'dana@example.com');

  const back = Serify.fromFieldMap(User, fm);
  assert.equal(back.id, 42n);
  assert.equal(back.name, 'Dana');
  assert.equal(back.email, 'dana@example.com');
});

test('model helper keeps bigint values exact beyond 2^53', () => {
  const User = makeUserClass();
  const u = new User();
  u.id = 18446744073709551615n;
  u.name = '';
  u.email = '';

  const fm = Serify.toFieldMap(u);
  assert.equal(fm.getI64('id'), 18446744073709551615n);
  assert.equal(Serify.fromFieldMap(User, fm).id, 18446744073709551615n);
});

test('model helper converts safe integer numbers exactly', () => {
  const User = makeUserClass();
  const u = new User();
  u.id = 9007199254740991; // Number.MAX_SAFE_INTEGER, as a plain number
  u.name = '';
  u.email = '';

  const fm = Serify.toFieldMap(u);
  assert.equal(fm.getI64('id'), 9007199254740991n);
});

test('model helper rejects unsafe integer numbers instead of truncating', () => {
  const User = makeUserClass();
  const u = new User();
  u.id = 18446744073709551615; // silently already imprecise as a number
  u.name = '';
  u.email = '';

  assert.throws(() => Serify.toFieldMap(u), /not exactly representable/);
});

test('FieldMap basic accessors', () => {
  const fm = new FieldMap();
  fm.setU64('n', 123n);
  fm.setString('s', 'x');
  fm.setBool('b', true);
  assert.equal(fm.getU64('n'), 123n);
  assert.equal(fm.getString('s'), 'x');
  assert.equal(fm.getBool('b'), true);
});

// --- oneof ------------------------------------------------------------------

// A oneof carrying a bytes payload — the shape zero-copy detection must see.
const oneofSchema = [{
  name: 'channel',
  type: 'oneof<silent, receipt: bytes>',
  fields: [],
  variants: [
    { name: 'silent' },
    { name: 'receipt', payload: { name: 'receipt', type: 'bytes', fields: [] } },
  ],
}];

test('oneof round-trips through decode/encode', () => {
  for (const data of [{ channel: { silent: null } }, { channel: { receipt: 'deadbeef' } }]) {
    const { out } = roundTrip(oneofSchema, data);
    assert.deepEqual(out, data);
  }
});

test('detectZeroCopy sees an aliased oneof payload', () => {
  // A deserializer that aliases the input buffer into a oneof bytes payload must
  // be reported. Before collectByteSnaps grew a Variant branch the walker never
  // descended into the variant and this returned [].
  const buf = Buffer.from([1, 2, 3, 4]);
  const fm = new FieldMap();
  fm.setVariant('channel', 'receipt', buf); // aliased, not copied

  assert.deepEqual(detectZeroCopy(fm, buf), ['channel']);
});

test('detectZeroCopy stays silent on an owned oneof payload', () => {
  // The mirror image: a payload copied out of the buffer is not aliasing and
  // must not warn, or every correct worker would be flagged.
  const buf = Buffer.from([1, 2, 3, 4]);
  const fm = new FieldMap();
  fm.setVariant('channel', 'receipt', Buffer.from(buf)); // copied

  assert.deepEqual(detectZeroCopy(fm, buf), []);
});

// --- list element types -----------------------------------------------------

test('list supports every scalar element type', () => {
  // decodeList used to carry its own switch that omitted
  // uint16/int8/int16/float64/bytes: declarable in a case file, accepted by
  // `serify validate`, and only failing once a worker actually ran.
  const f32 = (x) => { const b = Buffer.alloc(4); b.writeFloatLE(x, 0); return b.toString('hex'); };
  const f64 = (x) => { const b = Buffer.alloc(8); b.writeDoubleLE(x, 0); return b.toString('hex'); };

  const cases = [
    ['uint8', [0, 255]],
    ['uint16', [0, 65535]],
    ['uint32', [0, 4294967295]],
    ['uint64', ['0', '18446744073709551615']],
    ['int8', [-128, 127]],
    ['int16', [-32768, 32767]],
    ['int32', [-2147483648, 2147483647]],
    ['int64', ['-9223372036854775808', '0']],
    ['uint128', ['340282366920938463463374607431768211455', '0']],
    ['int128', ['-170141183460469231731687303715884105728', '0']],
    ['float32', [f32(1.5), f32(-2)]],
    ['float64', [f64(1.5), f64(-2)]],
    ['bool', [true, false]],
    ['string', ['a', '']],
    ['bytes', ['dead', '']],
  ];

  for (const [elem, sent] of cases) {
    const schema = [{ name: 'v', type: `list<${elem}>` }];
    // Re-encoding must reproduce the wire form, so the two directions cannot
    // drift apart for any element type.
    const { out } = roundTrip(schema, { v: sent });
    assert.deepEqual(out, { v: sent }, `list<${elem}> did not round-trip`);
  }
});

test('list<float64> decodes to numbers, not the hex it travelled as', () => {
  const b = Buffer.alloc(8); b.writeDoubleLE(1.5, 0);
  const fm = decodeFieldMap({ v: [b.toString('hex')] }, [{ name: 'v', type: 'list<float64>' }]);
  assert.deepEqual(fm.getListF64('v'), [1.5]);
});

test('list rejects an unknown element type', () => {
  assert.throws(
    () => decodeFieldMap({ v: [1] }, [{ name: 'v', type: 'list<nope>' }]),
    /unsupported list element type "nope"/,
  );
});

test('an unknown field type fails loudly in both directions', () => {
  // Decoding used to fall off the chain (leaving the field absent) and encoding
  // used to return the value untouched. Both surface far downstream as a missing
  // or wrong value rather than as "this library does not know that type".
  const schema = [{ name: 'v', type: 'nope' }];
  assert.throws(() => decodeFieldMap({ v: 1 }, schema), /unknown type "nope"/);

  const fm = new FieldMap();
  fm.setU32('v', 1);
  assert.throws(() => encodeFieldMap(fm, schema), /unknown type "nope"/);
});
