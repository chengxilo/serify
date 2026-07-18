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
 * NotificationRecord mirrors examples/cases/notification.yaml, whose `channel`
 * field is a `oneof`.
 *
 * PHP has no enum with payloads, but a property union type is its sum type, and
 * that is all the binding needs: the union names the arms and each arm's own
 * properties give its payload. No converter, no registration.
 *
 * PHP's int is only 64-bit, so the `push` payload is carried as a decimal string
 * and converted through the GMP helpers in wire.php.
 *
 * Go is the --ref language and owns the byte layout; see examples/go/wire.go.
 */

declare(strict_types=1);

use Serify\Attributes\SerifyField;
use Serify\Attributes\SerifyModel;

/** arity 0 — a unit variant */
class Silent {}

/** arity 1 — a scalar payload */
class Sms
{
    public function __construct(public string $value = '') {}
}

/** arity 1 — a payload that exceeds 2^53, so it rides as a decimal string */
class Push
{
    public function __construct(public string $value = '0') {}
}

/** arity N — a struct payload */
class Invoice
{
    public function __construct(
        public string $currency = '',
        public string $amountMinor = '0',
    ) {}
}

#[SerifyModel]
class NotificationRecord
{
    #[SerifyField] public int $notificationId = 0;
    #[SerifyField] public Silent|Sms|Push|Invoice $channel;
    #[SerifyField] public bool $urgent = false;

    public function marshal(): string
    {
        $out = pack('V', $this->notificationId);

        // The tag ordinal is the variant's position in the case file's oneof,
        // which is the declaration order of the four arms above. The schema tag
        // *names* are the binding's business, and never appear here.
        $out .= match (true) {
            $this->channel instanceof Silent  => pack('C', 0), // a unit variant is nothing but its tag
            $this->channel instanceof Sms     => pack('C', 1) . lenPrefixed($this->channel->value),
            $this->channel instanceof Push    => pack('C', 2) . encodeInt($this->channel->value, 8),
            $this->channel instanceof Invoice => pack('C', 3)
                . lenPrefixed($this->channel->currency)
                . encodeInt($this->channel->amountMinor, 8),
        };

        return $out . pack('C', $this->urgent ? 1 : 0);
    }

    public static function unmarshal(string $data): self
    {
        $n = new self();
        $n->notificationId = unpack('V', substr($data, 0, 4))[1];

        $off = 5;
        $n->channel = match (unpack('C', substr($data, 4, 1))[1]) {
            0 => new Silent(),
            1 => new Sms(readLenPrefixed($data, $off)),
            2 => new Push(decodeUnsigned(substr($data, $off, 8))),
            3 => new Invoice(readLenPrefixed($data, $off), decodeSigned(substr($data, $off, 8))),
            default => throw new RuntimeException('unknown channel ordinal'),
        };
        // push and invoice each consumed a trailing 8-byte integer that the
        // match arm above could not advance past.
        if ($n->channel instanceof Push || $n->channel instanceof Invoice) {
            $off += 8;
        }

        $n->urgent = unpack('C', substr($data, $off, 1))[1] !== 0;
        return $n;
    }
}
