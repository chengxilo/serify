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
 * The Node client for the taskstore server.
 *
 *     node dist/client.js list
 *     node dist/client.js create "Buy milk" normal errand,home 1755820800
 *     node dist/client.js read 1001
 *     node dist/client.js update 1001 "Buy oat milk" true high errand
 *     node dist/client.js delete 1001
 *
 * Same arguments and same bytes as the Go, Python and Rust clients, so it talks
 * to the Go server without either side knowing which language the other is.
 */

import { createConnection } from 'net';

import { readFrame, writeFrame } from './frame';
import {
  Accepted, ApiResult, Create, Delete, Failed, Found, ListAll, Listing,
  Op, Read, Request, Response, Update,
} from './message';
import { ApiError, Draft, PRIORITIES, Task, TaskPage } from './model';

const USAGE = `usage: client [--addr host:port] <command> [args]

  list
  create <title> [priority] [tag,tag] [due-unix]
  read   <id>
  update <id> <title> <done> [priority] [tag,tag] [due-unix]
  delete <id>

priority is low, normal or high (default normal).`;

async function main(): Promise<number> {
  const argv = process.argv.slice(2);

  let addr = '127.0.0.1:9977';
  if (argv[0] === '--addr' && argv.length >= 2) {
    addr = argv[1];
    argv.splice(0, 2);
  }

  let op: Op;
  try {
    op = parseOp(argv);
  } catch (e) {
    console.error(`${(e as Error).message}\n\n${USAGE}`);
    return 2;
  }

  const req = new Request();
  req.request_id = 1;
  req.op = op;

  let resp: Response;
  try {
    resp = await send(addr, req);
  } catch (e) {
    console.error(`${addr}: ${(e as Error).message}`);
    return 1;
  }

  console.log(render(resp.result));
  return resp.result instanceof Failed ? 1 : 0;
}

/** Opens a connection, writes one request frame and reads one response frame. */
function send(addr: string, req: Request): Promise<Response> {
  const idx = addr.lastIndexOf(':');
  const host = addr.slice(0, idx);
  const port = Number(addr.slice(idx + 1));
  const payload = req.marshal();

  return new Promise((resolve, reject) => {
    const sock = createConnection({ host, port }, () => {
      const frame = readFrame(sock);
      writeFrame(sock, payload);
      frame.then((data) => {
        sock.end();
        resolve(Response.unmarshal(data));
      }, (e) => {
        sock.destroy();
        reject(e);
      });
    });
    sock.on('error', reject);
  });
}

function parseOp(args: string[]): Op {
  const arg = (i: number, fallback = ''): string => (args[i] ? args[i] : fallback);
  const taskId = (raw: string): bigint => {
    if (!/^\d+$/.test(raw)) throw new Error(`bad id "${raw}"`);
    return BigInt(raw);
  };

  switch (args[0]) {
    case 'list':
      return new ListAll();

    case 'create': {
      if (args.length < 2) throw new Error('create needs a title');
      const d = new Draft();
      d.title = args[1];
      d.priority = parsePriority(arg(2, 'normal'));
      d.tags = parseTags(arg(3));
      d.due_at = parseDue(arg(4));
      return new Create(d);
    }

    case 'read':
      if (args.length < 2) throw new Error('read needs an id');
      return new Read(taskId(args[1]));

    case 'delete':
      if (args.length < 2) throw new Error('delete needs an id');
      return new Delete(taskId(args[1]));

    case 'update': {
      if (args.length < 4) throw new Error('update needs an id, a title and done');
      if (args[3] !== 'true' && args[3] !== 'false') {
        throw new Error(`bad done "${args[3]}", want true or false`);
      }
      const t = new Task();
      t.id = taskId(args[1]);
      t.title = args[2];
      t.done = args[3] === 'true';
      t.priority = parsePriority(arg(4, 'normal'));
      t.tags = parseTags(arg(5));
      t.due_at = parseDue(arg(6));
      return new Update(t);
    }

    default:
      throw new Error(args[0] ? `unknown command "${args[0]}"` : 'no command given');
  }
}

/**
 * Rejects a bad priority here, before anything is encoded. This is the only
 * place in the project where one can exist: an enum has no wire representation
 * outside its declared variants, so by the time a request is bytes the value is
 * already known to be good — which is why the server does not check it again.
 */
function parsePriority(s: string): string {
  if (!(PRIORITIES as readonly string[]).includes(s)) {
    throw new Error(`"${s}" is not a priority (${PRIORITIES.join(', ')})`);
  }
  return s;
}

function parseTags(s: string): string[] {
  return s ? s.split(',') : [];
}

function parseDue(s: string): bigint | null {
  return /^-?\d+$/.test(s) ? BigInt(s) : null;
}

function render(result: ApiResult): string {
  if (result instanceof Accepted) return 'accepted';
  if (result instanceof Found) return renderTask(result.value);
  if (result instanceof Listing) {
    const page: TaskPage = result.value;
    const lines = page.items.map(renderTask);
    lines.push(`(${page.items.length} shown, ${page.total} total)`);
    return lines.join('\n');
  }
  const err: ApiError = (result as Failed).value;
  return `error: ${err.code}: ${err.message}`;
}

function renderTask(t: Task): string {
  let line = `[${t.done ? 'x' : ' '}] ${t.id}  ${t.title.padEnd(30)}  ${t.priority}`;
  if (t.tags.length > 0) line += `  #${t.tags.join(' #')}`;
  if (t.due_at !== null) line += `  due=${t.due_at}`;
  return line;
}

main().then((code) => process.exit(code));
