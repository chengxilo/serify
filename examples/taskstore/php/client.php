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
 * The PHP client for the taskstore server.
 *
 *     php client.php list
 *     php client.php create "Buy milk" normal errand,home 1755820800
 *     php client.php read 1001
 *     php client.php update 1001 "Buy oat milk" true high errand
 *     php client.php delete 1001
 *
 * Same arguments and same bytes as the other clients, so it talks to the Go
 * server without either side knowing which language the other is.
 *
 * Requires ext-gmp (apt install php-gmp).
 */

declare(strict_types=1);

require_once __DIR__ . '/../../../lib/php/src/FieldMap.php';
require_once __DIR__ . '/../../../lib/php/src/Attributes/SerifyModel.php';
require_once __DIR__ . '/../../../lib/php/src/Attributes/SerifyField.php';

require_once __DIR__ . '/wire.php';
require_once __DIR__ . '/model.php';
require_once __DIR__ . '/message.php';
require_once __DIR__ . '/frame.php';

const USAGE = <<<TXT
usage: client.php [--addr host:port] <command> [args]

  list
  create <title> [priority] [tag,tag] [due-unix]
  read   <id>
  update <id> <title> <done> [priority] [tag,tag] [due-unix]
  delete <id>

priority is low, normal or high (default normal).
TXT;

/**
 * Opens a connection, writes one request frame and reads one response frame.
 */
function send(string $addr, Request $req): Response
{
    $payload = $req->marshal();

    $stream = @stream_socket_client("tcp://$addr", $errno, $errstr, 10);
    if ($stream === false) {
        throw new RuntimeException($errstr !== '' ? $errstr : "cannot connect (errno $errno)");
    }
    try {
        writeFrame($stream, $payload);
        return Response::unmarshal(readFrame($stream));
    } finally {
        fclose($stream);
    }
}

/** @param list<string> $args */
function parseOp(array $args): ListAll|Create|Read|Update|Delete
{
    $arg = static fn(int $i, string $fallback = ''): string
        => isset($args[$i]) && $args[$i] !== '' ? $args[$i] : $fallback;

    $taskId = static function (string $raw): string {
        if ($raw === '' || preg_match('/^\d+$/', $raw) !== 1) {
            throw new InvalidArgumentException("bad id \"$raw\"");
        }
        return $raw;
    };

    return match ($args[0] ?? '') {
        'list' => new ListAll(),

        'create' => (function () use ($args, $arg): Create {
            if (!isset($args[1])) {
                throw new InvalidArgumentException('create needs a title');
            }
            $d           = new Draft();
            $d->title    = $args[1];
            $d->priority = parsePriority($arg(2, 'normal'));
            $d->tags     = parseTags($arg(3));
            $d->dueAt    = parseDue($arg(4));
            return new Create($d);
        })(),

        'read' => new Read($taskId($args[1] ?? '')),
        'delete' => new Delete($taskId($args[1] ?? '')),

        'update' => (function () use ($args, $arg, $taskId): Update {
            if (count($args) < 4) {
                throw new InvalidArgumentException('update needs an id, a title and done');
            }
            if ($args[3] !== 'true' && $args[3] !== 'false') {
                throw new InvalidArgumentException("bad done \"{$args[3]}\", want true or false");
            }
            $t           = new Task();
            $t->id       = $taskId($args[1]);
            $t->title    = $args[2];
            $t->done     = $args[3] === 'true';
            $t->priority = parsePriority($arg(4, 'normal'));
            $t->tags     = parseTags($arg(5));
            $t->dueAt    = parseDue($arg(6));
            return new Update($t);
        })(),

        default => throw new InvalidArgumentException(
            isset($args[0]) && $args[0] !== ''
                ? "unknown command \"{$args[0]}\""
                : 'no command given'
        ),
    };
}

/**
 * Rejects a bad priority before anything is encoded. An enum has no wire
 * representation outside its declared variants, so by the time a request is
 * bytes the value is already known to be good.
 */
function parsePriority(string $s): string
{
    if (!in_array($s, PRIORITIES, true)) {
        throw new InvalidArgumentException("\"$s\" is not a priority (" . implode(', ', PRIORITIES) . ')');
    }
    return $s;
}

/** @return list<string> */
function parseTags(string $s): array
{
    return $s === '' ? [] : explode(',', $s);
}

function parseDue(string $s): ?string
{
    return preg_match('/^-?\d+$/', $s) === 1 ? $s : null;
}

function render(Accepted|Found|Listing|Failed $result): string
{
    if ($result instanceof Accepted) {
        return 'accepted';
    }
    if ($result instanceof Found) {
        return renderTask($result->value);
    }
    if ($result instanceof Listing) {
        $page  = $result->value;
        $lines = array_map('renderTask', $page->items);
        $lines[] = '(' . count($page->items) . " shown, {$page->total} total)";
        return implode("\n", $lines);
    }
    return "error: {$result->value->code}: {$result->value->message}";
}

function renderTask(Task $t): string
{
    // mb_str_pad is PHP 8.3; str_pad counts bytes, so a non-ASCII title lines up
    // differently from the Go and Python clients. Display only.
    $line = sprintf('[%s] %s  %s  %s',
        $t->done ? 'x' : ' ', $t->id, str_pad($t->title, 30), $t->priority);
    if ($t->tags !== []) {
        $line .= '  #' . implode(' #', $t->tags);
    }
    if ($t->dueAt !== null) {
        $line .= "  due={$t->dueAt}";
    }
    return $line;
}

$args = array_slice($argv, 1);

$addr = '127.0.0.1:9977';
if (count($args) >= 2 && $args[0] === '--addr') {
    $addr = $args[1];
    $args = array_slice($args, 2);
}
$args = array_values($args);

try {
    $op = parseOp($args);
} catch (InvalidArgumentException $e) {
    fwrite(STDERR, $e->getMessage() . "\n\n" . USAGE . "\n");
    exit(2);
}

$req            = new Request();
$req->requestId = 1;
$req->op        = $op;

try {
    $resp = send($addr, $req);
} catch (RuntimeException $e) {
    fwrite(STDERR, "$addr: {$e->getMessage()}\n");
    exit(1);
}

echo render($resp->result), "\n";
exit($resp->result instanceof Failed ? 1 : 0);
