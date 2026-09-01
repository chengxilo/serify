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
 * The conformance worker: serify's entire footprint in the Node follower.
 *
 * It registers the two types that cross the socket and hands each the very
 * functions client.ts calls — `Request.marshal` is not a test double.
 */

import { runSuite, type } from '@chengxilo/serify';

import { Request, Response } from './message';

runSuite({
  request: type(Request, {
    binary: {
      serialize: (r: Request) => r.marshal(),
      deserialize: (d: Buffer) => Request.unmarshal(d),
    },
  }),
  response: type(Response, {
    binary: {
      serialize: (r: Response) => r.marshal(),
      deserialize: (d: Buffer) => Response.unmarshal(d),
    },
  }),
});
