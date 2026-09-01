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
 * The operation a request asks for: the {@code sum} in cases/op.yaml.
 *
 * <p>Java's sum type is a sealed interface, and that is all the binding needs:
 * {@code permits} names the arms and each arm's record components give its
 * payload. No converter, no registration — and the compiler will not let a
 * request carry two operations at once.
 */
public sealed interface Op permits Op.ListAll, Op.Create, Op.Read, Op.Update, Op.Delete {

    /** arity 0 — a unit variant, no payload */
    record ListAll() implements Op {}

    /** arity 1, and the payload is a model, so it travels as a struct */
    record Create(Draft value) implements Op {}

    /** arity 1 — a scalar payload: the id */
    record Read(Long value) implements Op {}

    record Update(Task value) implements Op {}

    record Delete(Long value) implements Op {}
}
