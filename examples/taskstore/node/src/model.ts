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
 * The records the server stores and sends. Each mirrors the case file of the
 * same name and owns its own byte layout.
 *
 * `@Serify.Model()` plus one `@Serify.field()` per property is the entire schema
 * binding. An enum needs nothing from it — it travels as its variant *name*, so
 * `priority` is a plain string and PRIORITIES fixes the ordinal this codec
 * writes.
 */

import { Serify } from '@chengxilo/serify';

import { Reader, putEnum, putOptionalI64, putStr, putTags, putU32, putU64 } from './wire';

/** Declaration order of the `enum<low, normal, high>` in cases/task.yaml. */
export const PRIORITIES = ['low', 'normal', 'high'] as const;

/** Declaration order of the enum in cases/api_error.yaml. */
export const ERROR_CODES = ['not_found', 'invalid', 'conflict'] as const;

/** A stored task: cases/task.yaml. */
@Serify.Model()
export class Task {
  @Serify.field() id = 0n;
  @Serify.field() title = '';
  @Serify.field() done = false;
  @Serify.field() priority = 'normal';
  @Serify.field() tags: string[] = [];
  @Serify.field() due_at: bigint | null = null;

  pack(): Buffer {
    return Buffer.concat([
      putU64(this.id),
      putStr(this.title),
      Buffer.from([this.done ? 1 : 0]),
      putEnum(PRIORITIES, this.priority),
      putTags(this.tags),
      putOptionalI64(this.due_at),
    ]);
  }

  static unpack(r: Reader): Task {
    const t = new Task();
    t.id = r.u64();
    t.title = r.str();
    t.done = r.bool();
    t.priority = r.enumOf(PRIORITIES);
    t.tags = r.tags();
    t.due_at = r.optionalI64();
    return t;
  }
}

/** A task the server has not assigned an id to yet: cases/draft.yaml. */
@Serify.Model()
export class Draft {
  @Serify.field() title = '';
  @Serify.field() priority = 'normal';
  @Serify.field() tags: string[] = [];
  @Serify.field() due_at: bigint | null = null;

  pack(): Buffer {
    return Buffer.concat([
      putStr(this.title),
      putEnum(PRIORITIES, this.priority),
      putTags(this.tags),
      putOptionalI64(this.due_at),
    ]);
  }

  static unpack(r: Reader): Draft {
    const d = new Draft();
    d.title = r.str();
    d.priority = r.enumOf(PRIORITIES);
    d.tags = r.tags();
    d.due_at = r.optionalI64();
    return d;
  }
}

/** The body of a list response: cases/task_page.yaml. */
@Serify.Model()
export class TaskPage {
  @Serify.field({ model: Task }) items: Task[] = [];
  @Serify.field() total = 0;

  pack(): Buffer {
    return Buffer.concat([
      putU32(this.items.length),
      ...this.items.map((t) => t.pack()),
      putU32(this.total),
    ]);
  }

  static unpack(r: Reader): TaskPage {
    const p = new TaskPage();
    const n = r.u32();
    for (let i = 0; i < n; i++) p.items.push(Task.unpack(r));
    p.total = r.u32();
    return p;
  }
}

/** A failed request: cases/api_error.yaml. */
@Serify.Model()
export class ApiError {
  @Serify.field() code = 'not_found';
  @Serify.field() message = '';

  pack(): Buffer {
    return Buffer.concat([putEnum(ERROR_CODES, this.code), putStr(this.message)]);
  }

  static unpack(r: Reader): ApiError {
    const e = new ApiError();
    e.code = r.enumOf(ERROR_CODES);
    e.message = r.str();
    return e;
  }
}
