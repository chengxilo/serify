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
 * SignalCapture mirrors examples/cases/signals.yaml, which uses every scalar the
 * schema allows as a list element.
 *
 * PHP's `array` type says nothing about its elements, so the binding stores each
 * list as-is and the schema decides the element's wire form. PHP's int is only
 * 64-bit, so the 64/128-bit lists carry decimal strings and go through the GMP
 * helpers in wire.php — the same convention ledger.php uses.
 *
 * Go is the --ref language and owns the byte layout; see examples/go/wire.go.
 */

declare(strict_types=1);

use Serify\Attributes\SerifyField;
use Serify\Attributes\SerifyModel;

#[SerifyModel]
class SignalCapture
{
    /** Declaration order of the `mode` enum; the index is the wire ordinal. */
    private const MODES = ['idle', 'active', 'fault', 'calibrating'];

    #[SerifyField] public string $captureId = '0';
    #[SerifyField] public array $flags = [];
    #[SerifyField] public array $rawFrame = [];
    #[SerifyField] public array $portNumbers = [];
    #[SerifyField] public array $sampleCounts = [];
    #[SerifyField] public array $byteTotals = [];
    #[SerifyField] public array $trimOffsets = [];
    #[SerifyField] public array $driftDeltas = [];
    #[SerifyField] public array $temperaturesC = [];
    #[SerifyField] public array $timestampsNs = [];
    #[SerifyField] public array $counters = [];
    #[SerifyField] public array $balances = [];
    #[SerifyField] public array $gains = [];
    #[SerifyField] public array $voltages = [];
    #[SerifyField] public array $channelNames = [];
    #[SerifyField] public array $payloads = [];
    #[SerifyField] public array $checksum = [];
    #[SerifyField] public array $window = [];
    #[SerifyField('dropped_frames')] public ?int $droppedFrames = null;
    #[SerifyField] public string $mode = '';

    public function marshal(): string
    {
        $out = encodeInt($this->captureId, 8);

        $out .= pack('V', count($this->flags));
        foreach ($this->flags as $v) {
            $out .= pack('C', $v ? 1 : 0);
        }

        $out .= self::packEach($this->rawFrame, fn($v) => pack('C', $v));
        $out .= self::packEach($this->portNumbers, fn($v) => pack('v', $v));
        $out .= self::packEach($this->sampleCounts, fn($v) => pack('V', $v));
        $out .= self::packEach($this->byteTotals, fn($v) => encodeInt((string) $v, 8));
        $out .= self::packEach($this->trimOffsets, fn($v) => pack('c', $v));
        $out .= self::packEach($this->driftDeltas, fn($v) => pack('v', $v & 0xFFFF));
        $out .= self::packEach($this->temperaturesC, fn($v) => pack('V', $v & 0xFFFFFFFF));
        $out .= self::packEach($this->timestampsNs, fn($v) => encodeInt((string) $v, 8));
        $out .= self::packEach($this->counters, fn($v) => encodeInt((string) $v, 16));
        $out .= self::packEach($this->balances, fn($v) => encodeInt((string) $v, 16));
        $out .= self::packEach($this->gains, fn($v) => pack('g', $v));
        $out .= self::packEach($this->voltages, fn($v) => pack('e', $v));
        $out .= self::packEach($this->channelNames, fn($v) => lenPrefixed($v));
        $out .= self::packEach($this->payloads, fn($v) => lenPrefixed($v));

        // array<T,N> carries no count: N is fixed by the schema.
        foreach ($this->checksum as $v) {
            $out .= pack('C', $v);
        }
        foreach ($this->window as $v) {
            $out .= pack('v', $v & 0xFFFF);
        }

        // optional<uint32>: a presence flag, then the value if present.
        if ($this->droppedFrames === null) {
            $out .= pack('C', 0);
        } else {
            $out .= pack('C', 1) . pack('V', $this->droppedFrames);
        }

        // enum: a u8 ordinal, the variant's position in the case file.
        $out .= pack('C', array_search($this->mode, self::MODES, true));

        return $out;
    }

    /** u32 element count, then each element rendered by $f. */
    private static function packEach(array $items, callable $f): string
    {
        $out = pack('V', count($items));
        foreach ($items as $v) {
            $out .= $f($v);
        }
        return $out;
    }

    /** Read a u32 count, then that many fixed-width elements of $size bytes. */
    private static function unpackEach(string $data, int &$off, int $size, callable $f): array
    {
        $n = unpack('V', substr($data, $off, 4))[1];
        $off += 4;
        $out = [];
        for ($i = 0; $i < $n; $i++) {
            $out[] = $f(substr($data, $off, $size));
            $off += $size;
        }
        return $out;
    }

    public static function unmarshal(string $data): self
    {
        $s = new self();
        $off = 0;

        $s->captureId = decodeUnsigned(substr($data, $off, 8));
        $off += 8;

        $s->flags = self::unpackEach($data, $off, 1, fn($b) => unpack('C', $b)[1] !== 0);
        $s->rawFrame = self::unpackEach($data, $off, 1, fn($b) => unpack('C', $b)[1]);
        $s->portNumbers = self::unpackEach($data, $off, 2, fn($b) => unpack('v', $b)[1]);
        $s->sampleCounts = self::unpackEach($data, $off, 4, fn($b) => (int) decodeUnsigned($b));
        $s->byteTotals = self::unpackEach($data, $off, 8, fn($b) => decodeUnsigned($b));
        $s->trimOffsets = self::unpackEach($data, $off, 1, fn($b) => unpack('c', $b)[1]);
        $s->driftDeltas = self::unpackEach($data, $off, 2, fn($b) => (int) decodeSigned($b));
        $s->temperaturesC = self::unpackEach($data, $off, 4, fn($b) => (int) decodeSigned($b));
        $s->timestampsNs = self::unpackEach($data, $off, 8, fn($b) => decodeSigned($b));
        $s->counters = self::unpackEach($data, $off, 16, fn($b) => decodeUnsigned($b));
        $s->balances = self::unpackEach($data, $off, 16, fn($b) => decodeSigned($b));
        $s->gains = self::unpackEach($data, $off, 4, fn($b) => unpack('g', $b)[1]);
        $s->voltages = self::unpackEach($data, $off, 8, fn($b) => unpack('e', $b)[1]);

        $nameCount = unpack('V', substr($data, $off, 4))[1];
        $off += 4;
        $s->channelNames = [];
        for ($i = 0; $i < $nameCount; $i++) {
            $s->channelNames[] = readLenPrefixed($data, $off);
        }

        $payloadCount = unpack('V', substr($data, $off, 4))[1];
        $off += 4;
        $s->payloads = [];
        for ($i = 0; $i < $payloadCount; $i++) {
            $s->payloads[] = readLenPrefixed($data, $off);
        }

        $s->checksum = [];
        for ($i = 0; $i < 4; $i++) {
            $s->checksum[] = unpack('C', substr($data, $off, 1))[1];
            $off += 1;
        }
        $s->window = [];
        for ($i = 0; $i < 3; $i++) {
            $s->window[] = (int) decodeSigned(substr($data, $off, 2));
            $off += 2;
        }

        if (unpack('C', substr($data, $off, 1))[1] === 0) {
            $s->droppedFrames = null;
            $off += 1;
        } else {
            $s->droppedFrames = unpack('V', substr($data, $off + 1, 4))[1];
            $off += 5;
        }

        $s->mode = self::MODES[unpack('C', substr($data, $off, 1))[1]];
        $off += 1;

        return $s;
    }
}
