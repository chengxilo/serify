/**
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


/**
 * The two types that cross the socket, and the only two the suite tests, plus
 * the two sums they carry.
 *
 * TypeScript's union type is erased before the code runs, so the arms have to be
 * named at runtime — that is what `@Serify.sum([...])` is, and it is the only
 * extra line: each arm is a plain class whose own properties are its payload.
 */

import { Serify } from '@chengxilo/serify';

import { ApiError, Draft, Task, TaskPage } from './model';
import { Reader, putU32, putU64 } from './wire';

// --- the operation: the sum in cases/op.yaml ---------------------------------

/** arity 0 — a unit variant, no payload */
export class ListAll {}

/** arity N — a struct payload */
export class Create {
  constructor(public value: Draft = new Draft()) {}
}

/** arity 1 — a scalar payload: the id */
export class Read {
  constructor(public value = 0n) {}
}

/** arity N — the whole task, id included */
export class Update {
  constructor(public value: Task = new Task()) {}
}

export class Delete {
  constructor(public value = 0n) {}
}

export type Op = ListAll | Create | Read | Update | Delete;

/** Tag ordinals, in the declaration order of cases/op.yaml. */
const OP_ARMS = [ListAll, Create, Read, Update, Delete];

// --- the outcome: the sum in cases/result.yaml -------------------------------

/** a delete that worked: nothing to send back */
export class Accepted {}

export class Found {
  constructor(public value: Task = new Task()) {}
}

export class Listing {
  constructor(public value: TaskPage = new TaskPage()) {}
}

export class Failed {
  constructor(public value: ApiError = new ApiError()) {}
}

export type ApiResult = Accepted | Found | Listing | Failed;

/** Tag ordinals, in the declaration order of cases/result.yaml. */
const RESULT_ARMS = [Accepted, Found, Listing, Failed];

function tagOf(arms: Function[], v: object): Buffer {
  const ord = arms.indexOf(v.constructor);
  if (ord < 0) throw new Error(`unhandled arm ${v.constructor.name}`);
  return Buffer.from([ord]);
}

function packOp(op: Op): Buffer {
  const tag = tagOf(OP_ARMS, op);
  if (op instanceof ListAll) return tag; // a unit variant is nothing but its tag
  if (op instanceof Create) return Buffer.concat([tag, op.value.pack()]);
  if (op instanceof Update) return Buffer.concat([tag, op.value.pack()]);
  return Buffer.concat([tag, putU64((op as Read | Delete).value)]);
}

function unpackOp(r: Reader): Op {
  const tag = r.u8();
  switch (OP_ARMS[tag]) {
    case ListAll: return new ListAll();
    case Create: return new Create(Draft.unpack(r));
    case Read: return new Read(r.u64());
    case Update: return new Update(Task.unpack(r));
    case Delete: return new Delete(r.u64());
    default: throw new Error(`unknown op tag ${tag}`);
  }
}

function packResult(res: ApiResult): Buffer {
  const tag = tagOf(RESULT_ARMS, res);
  if (res instanceof Accepted) return tag;
  return Buffer.concat([tag, (res as Found | Listing | Failed).value.pack()]);
}

function unpackResult(r: Reader): ApiResult {
  const tag = r.u8();
  switch (RESULT_ARMS[tag]) {
    case Accepted: return new Accepted();
    case Found: return new Found(Task.unpack(r));
    case Listing: return new Listing(TaskPage.unpack(r));
    case Failed: return new Failed(ApiError.unpack(r));
    default: throw new Error(`unknown result tag ${tag}`);
  }
}

/** One request frame: cases/request.yaml. */
@Serify.Model()
export class Request {
  @Serify.field() request_id = 0;
  @Serify.sum([ListAll, Create, Read, Update, Delete]) op: Op = new ListAll();

  marshal(): Buffer {
    return Buffer.concat([putU32(this.request_id), packOp(this.op)]);
  }

  static unmarshal(data: Buffer): Request {
    const r = new Reader(data);
    const req = new Request();
    req.request_id = r.u32();
    req.op = unpackOp(r);
    return req;
  }
}

/**
 * One response frame: cases/response.yaml. `request_id` echoes the request's,
 * so a client with several in flight can tell them apart.
 */
@Serify.Model()
export class Response {
  @Serify.field() request_id = 0;
  @Serify.sum([Accepted, Found, Listing, Failed]) result: ApiResult = new Accepted();

  marshal(): Buffer {
    return Buffer.concat([putU32(this.request_id), packResult(this.result)]);
  }

  static unmarshal(data: Buffer): Response {
    const r = new Reader(data);
    const resp = new Response();
    resp.request_id = r.u32();
    resp.result = unpackResult(r);
    return resp;
  }
}
