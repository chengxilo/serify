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

import { FieldMap, runSuite } from '@chengxilo/serify';
function ser(fm: FieldMap): Buffer { return Buffer.from(fm.getString('id'), 'utf8'); }
function deser(data: Buffer): FieldMap { const fm = new FieldMap(); fm.setString('id', data.toString('utf8')); return fm; }
runSuite({ invalid_schema: { byte: { serialize: ser, deserialize: deser } } });
