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
 * The two sums and the two messages that carry them.
 *
 * PHP has no enum with payloads, but a property union type is its sum type, and
 * that is all the binding needs: the union names the arms and each arm's own
 * public properties give its payload. No converter, no registration — and no
 * way to build a request carrying two operations at once.
 */

declare(strict_types=1);

use Serify\Attributes\SerifyField;
use Serify\Attributes\SerifyModel;

/** arity 0 — a unit variant, no payload */
class ListAll {}

/** arity 1, and the one property is a model, so it travels as a struct */
class Create
{
    public function __construct(public Draft $value = new Draft()) {}
}

/** arity 1 — a scalar payload: the id, as a decimal string */
class Read
{
    public function __construct(public string $value = '0') {}
}

class Update
{
    public function __construct(public Task $value = new Task()) {}
}

class Delete
{
    public function __construct(public string $value = '0') {}
}

/** a delete that worked: nothing to send back */
class Accepted {}

class Found
{
    public function __construct(public Task $value = new Task()) {}
}

class Listing
{
    public function __construct(public TaskPage $value = new TaskPage()) {}
}

class Failed
{
    public function __construct(public ApiError $value = new ApiError()) {}
}

/** One request frame: cases/request.yaml. */
#[SerifyModel]
class Request
{
    #[SerifyField] public int $requestId = 0;
    #[SerifyField] public ListAll|Create|Read|Update|Delete $op;

    public function __construct()
    {
        $this->op = new ListAll();
    }

    public function marshal(): string
    {
        // The tag ordinal is the arm's position in the case file's sum, which is
        // the declaration order above. The schema tag *names* are the binding's
        // business and never appear here.
        return pack('V', $this->requestId) . match (true) {
            $this->op instanceof ListAll => pack('C', 0),  // a unit variant is nothing but its tag
            $this->op instanceof Create  => pack('C', 1) . $this->op->value->pack(),
            $this->op instanceof Read    => pack('C', 2) . encodeInt($this->op->value, 8),
            $this->op instanceof Update  => pack('C', 3) . $this->op->value->pack(),
            $this->op instanceof Delete  => pack('C', 4) . encodeInt($this->op->value, 8),
        };
    }

    public static function unmarshal(string $data): self
    {
        $r   = new Reader($data);
        $req = new self();
        $req->requestId = $r->u32();
        $req->op = match ($tag = $r->u8()) {
            0       => new ListAll(),
            1       => new Create(Draft::take($r)),
            2       => new Read($r->u64()),
            3       => new Update(Task::take($r)),
            4       => new Delete($r->u64()),
            default => throw new RuntimeException("unknown op tag $tag"),
        };
        return $req;
    }
}

/**
 * One response frame: cases/response.yaml. `$requestId` echoes the request's, so
 * a client with several in flight can tell them apart.
 */
#[SerifyModel]
class Response
{
    #[SerifyField] public int $requestId = 0;
    #[SerifyField] public Accepted|Found|Listing|Failed $result;

    public function __construct()
    {
        $this->result = new Accepted();
    }

    public function marshal(): string
    {
        return pack('V', $this->requestId) . match (true) {
            $this->result instanceof Accepted => pack('C', 0),
            $this->result instanceof Found    => pack('C', 1) . $this->result->value->pack(),
            $this->result instanceof Listing  => pack('C', 2) . $this->result->value->pack(),
            $this->result instanceof Failed   => pack('C', 3) . $this->result->value->pack(),
        };
    }

    public static function unmarshal(string $data): self
    {
        $r    = new Reader($data);
        $resp = new self();
        $resp->requestId = $r->u32();
        $resp->result = match ($tag = $r->u8()) {
            0       => new Accepted(),
            1       => new Found(Task::take($r)),
            2       => new Listing(TaskPage::take($r)),
            3       => new Failed(ApiError::take($r)),
            default => throw new RuntimeException("unknown result tag $tag"),
        };
        return $resp;
    }
}
