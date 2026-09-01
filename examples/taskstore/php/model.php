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


/**
 * The records the server stores and sends. Each mirrors the case file of the
 * same name and owns its own byte layout.
 *
 * `#[SerifyModel]` plus one `#[SerifyField]` per property is the entire schema
 * binding. An enum needs nothing from it — it travels as its variant *name*, so
 * `$priority` is a plain string and PRIORITIES fixes the ordinal this codec
 * writes. The 64-bit values are strings for the reason wire.php explains.
 */

declare(strict_types=1);

use Serify\Attributes\SerifyField;
use Serify\Attributes\SerifyModel;

/** A stored task: cases/task.yaml. */
#[SerifyModel]
class Task
{
    #[SerifyField] public string $id = '0';
    #[SerifyField] public string $title = '';
    #[SerifyField] public bool $done = false;
    #[SerifyField] public string $priority = 'normal';
    /** @var list<string> */
    #[SerifyField] public array $tags = [];
    #[SerifyField] public ?string $dueAt = null;

    public function pack(): string
    {
        return encodeInt($this->id, 8)
            . putStr($this->title)
            . pack('C', $this->done ? 1 : 0)
            . putEnum(PRIORITIES, $this->priority)
            . putTags($this->tags)
            . putOptionalI64($this->dueAt);
    }

    public static function take(Reader $r): self
    {
        $t           = new self();
        $t->id       = $r->u64();
        $t->title    = $r->str();
        $t->done     = $r->bool();
        $t->priority = $r->enumOf(PRIORITIES);
        $t->tags     = $r->tags();
        $t->dueAt    = $r->optionalI64();
        return $t;
    }
}

/** A task the server has not assigned an id to yet: cases/draft.yaml. */
#[SerifyModel]
class Draft
{
    #[SerifyField] public string $title = '';
    #[SerifyField] public string $priority = 'normal';
    /** @var list<string> */
    #[SerifyField] public array $tags = [];
    #[SerifyField] public ?string $dueAt = null;

    public function pack(): string
    {
        return putStr($this->title)
            . putEnum(PRIORITIES, $this->priority)
            . putTags($this->tags)
            . putOptionalI64($this->dueAt);
    }

    public static function take(Reader $r): self
    {
        $d           = new self();
        $d->title    = $r->str();
        $d->priority = $r->enumOf(PRIORITIES);
        $d->tags     = $r->tags();
        $d->dueAt    = $r->optionalI64();
        return $d;
    }
}

/** The body of a list response: cases/task_page.yaml. */
#[SerifyModel]
class TaskPage
{
    /** @var list<Task> */
    #[SerifyField(elem: Task::class)] public array $items = [];
    #[SerifyField] public int $total = 0;

    public function pack(): string
    {
        $out = pack('V', count($this->items));
        foreach ($this->items as $t) {
            $out .= $t->pack();
        }
        return $out . pack('V', $this->total);
    }

    public static function take(Reader $r): self
    {
        $p = new self();
        $n = $r->u32();
        for ($i = 0; $i < $n; $i++) {
            $p->items[] = Task::take($r);
        }
        $p->total = $r->u32();
        return $p;
    }
}

/** A failed request: cases/api_error.yaml. */
#[SerifyModel]
class ApiError
{
    #[SerifyField] public string $code = 'not_found';
    #[SerifyField] public string $message = '';

    public function pack(): string
    {
        return putEnum(ERROR_CODES, $this->code) . putStr($this->message);
    }

    public static function take(Reader $r): self
    {
        $e          = new self();
        $e->code    = $r->enumOf(ERROR_CODES);
        $e->message = $r->str();
        return $e;
    }
}
