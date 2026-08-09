<?php

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

declare(strict_types=1);

namespace Serify;

// resolve() calls SerifyModelHelper whenever a model is registered, and a
// worker loads this library with require_once rather than an autoloader — so
// the model path pulls in what it needs instead of failing at bind time in a
// worker that required one file too few.
require_once __DIR__ . '/SerifyModelHelper.php';

/**
 * One data type: a model, and the formats whose functions speak it.
 *
 * With a model, serify converts FieldMap <-> model on the way in and out, so
 * the worker's functions never see a FieldMap:
 *
 *     'ledger' => new Type(LedgerEntry::class, [
 *         'binary' => [fn(LedgerEntry $e) => $e->marshal(), LedgerEntry::unmarshal(...)],
 *     ])
 *
 * Passing null for $model is the other path, for a type with no natural class
 * (the audit fixtures mutate a FieldMap on purpose): the functions then take
 * and return the FieldMap itself, exactly as the older nested-array
 * registration does.
 */
class Type
{
    /**
     * @param class-string|null                       $model
     * @param array<string, array{0: ?callable, 1: ?callable}> $formats
     */
    public function __construct(
        private readonly ?string $model,
        private readonly array $formats,
    ) {
    }

    /**
     * The FieldMap-level pair the run loop calls, or null if this type does not
     * implement $format. '*' matches any format name.
     *
     * @return array{0: ?callable, 1: ?callable}|null
     */
    public function resolve(string $format): ?array
    {
        $pair = $this->formats[$format] ?? $this->formats['*'] ?? null;
        if (!is_array($pair)) {
            return null;
        }
        if ($this->model === null) {
            return $pair;
        }

        $model = $this->model;
        $ser = $pair[0] ?? null;
        $deser = $pair[1] ?? null;

        return [
            $ser === null
                ? null
                : fn(FieldMap $fm): string => $ser(SerifyModelHelper::fromFieldMap($fm, $model)),
            $deser === null
                ? null
                : fn(string $data): FieldMap => SerifyModelHelper::toFieldMap($deser($data)),
        ];
    }
}
