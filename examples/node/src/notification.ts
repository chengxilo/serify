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
 * `NotificationRecord` mirrors examples/cases/notification.yaml, whose
 * `channel` field is a `sum`.
 *
 * TypeScript's union type is erased before the code runs, so unlike Rust, Java,
 * Python, PHP, C# and Elixir — where the binding reads the arms off the
 * language's own sum type — the arms have to be named at runtime. That is what
 * `@Serify.sum([...])` is, and it is the only extra line: each arm is a plain
 * class whose own properties are its payload.
 *
 * Go is the --ref language and owns the byte layout; see examples/go/wire.go.
 */

import { Serify } from '@chengxilo/serify';

import { lenPrefixedStr } from './wire';

/** arity 0 — a unit variant */
export class Silent {}

/** arity 1 — a scalar payload */
export class Sms {
  constructor(public value = '') {}
}

/** arity 1 — a payload that exceeds 2^53, hence bigint */
export class Push {
  constructor(public value = 0n) {}
}

/** arity N — a struct payload */
export class Invoice {
  constructor(public currency = '', public amountMinor = 0n) {}
}

export type Channel = Silent | Sms | Push | Invoice;

@Serify.Model()
export class NotificationRecord {
  @Serify.field() notification_id = 0;
  @Serify.sum([Silent, Sms, Push, Invoice]) channel: Channel = new Silent();
  @Serify.field() urgent = false;

  marshal(): Buffer {
    const id = Buffer.alloc(4);
    id.writeUInt32LE(this.notification_id, 0);
    const parts: Buffer[] = [id];

    // The tag ordinal is the arm's position in the case file's sum, which is
    // the declaration order of the four classes above. The schema tag *names*
    // are the binding's business, and never appear here.
    if (this.channel instanceof Silent) {
      parts.push(Buffer.from([0])); // a unit variant is nothing but its tag
    } else if (this.channel instanceof Sms) {
      parts.push(Buffer.from([1]), lenPrefixedStr(this.channel.value));
    } else if (this.channel instanceof Push) {
      const b = Buffer.alloc(8);
      b.writeBigUInt64LE(this.channel.value, 0);
      parts.push(Buffer.from([2]), b);
    } else if (this.channel instanceof Invoice) {
      const amount = Buffer.alloc(8);
      amount.writeBigInt64LE(this.channel.amountMinor, 0);
      parts.push(Buffer.from([3]), lenPrefixedStr(this.channel.currency), amount);
    } else {
      throw new Error(`unhandled channel ${(this.channel as object)?.constructor?.name}`);
    }

    parts.push(Buffer.from([this.urgent ? 1 : 0]));
    return Buffer.concat(parts);
  }

  static unmarshal(data: Buffer): NotificationRecord {
    const n = new NotificationRecord();
    n.notification_id = data.readUInt32LE(0);
    let off = 5;

    const readLenStr = (): string => {
      const len = data.readUInt32LE(off);
      off += 4;
      const s = data.toString('utf8', off, off + len);
      off += len;
      return s;
    };

    switch (data[4]) {
      case 0:
        n.channel = new Silent();
        break;
      case 1:
        n.channel = new Sms(readLenStr());
        break;
      case 2:
        n.channel = new Push(data.readBigUInt64LE(off));
        off += 8;
        break;
      case 3: {
        const currency = readLenStr();
        n.channel = new Invoice(currency, data.readBigInt64LE(off));
        off += 8;
        break;
      }
      default:
        throw new Error(`unknown channel ordinal ${data[4]}`);
    }

    n.urgent = data[off] !== 0;
    return n;
  }
}
