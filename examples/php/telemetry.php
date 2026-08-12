<?php

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
 * `TelemetryFrame` mirrors examples/cases/telemetry.yaml — one reading from a
 * field device.
 *
 * This is the type that covers the corners the other examples do not: a
 * `uint128` address, two differently shaped fixed arrays, the suite's only
 * `optional<scalar>`, a `map<string,uint64>`, and float cases running through
 * NaN, ±Inf and negative zero. Only `binary` is declared, because NaN and Inf
 * have no JSON spelling.
 *
 * PHP's int is 64-bit and signed, so it cannot hold a u64 at the top of its
 * range and has no hope of a u128. Both travel as decimal strings and go
 * through ext-gmp in wire.php — that is why `$deviceId`, `$ipv6` and the map's
 * values are `string`, while everything 32 bits and under is a plain `int`.
 *
 * Go is the --ref language and owns the layout; see examples/go/wire.go.
 */

declare(strict_types=1);

use Serify\Attributes\SerifyField;
use Serify\Attributes\SerifyModel;

#[SerifyModel]
class TelemetryFrame
{
    #[SerifyField] public string $deviceId = '0';
    #[SerifyField] public string $ipv6 = '0';
    #[SerifyField] public array $localIp = [];
    #[SerifyField] public string $firmware = '';
    #[SerifyField] public int $bootCount = 0;
    #[SerifyField] public int $rssiDbm = 0;
    #[SerifyField] public int $temperatureDc = 0;
    #[SerifyField] public int $clockDriftMs = 0;
    #[SerifyField] public float $batteryVolts = 0.0;
    #[SerifyField] public float $latitude = 0.0;
    #[SerifyField] public float $longitude = 0.0;
    #[SerifyField] public ?float $humidityPct = null;
    #[SerifyField] public array $accelMg = [];
    #[SerifyField] public array $visibleCells = [];
    #[SerifyField] public array $packetCounts = [];
    #[SerifyField] public bool $gpsFix = false;
    #[SerifyField] public string $signature = '';

    public function marshal(): string
    {
        $out = encodeInt($this->deviceId, 8);
        // uint128 is unsigned, so the same 16 little-endian bytes int128 uses
        // serve here with no sign to re-apply on the way back.
        $out .= encodeInt($this->ipv6, 16);

        // array<T,N> carries no count: N is fixed by the schema.
        foreach ($this->localIp as $v) {
            $out .= pack('C', $v);
        }

        $out .= lenPrefixed($this->firmware);
        $out .= pack('v', $this->bootCount);
        $out .= pack('c', $this->rssiDbm);
        $out .= pack('v', $this->temperatureDc & 0xFFFF);
        $out .= pack('V', $this->clockDriftMs & 0xFFFFFFFF);
        $out .= pack('g', $this->batteryVolts);
        $out .= pack('e', $this->latitude);
        $out .= pack('e', $this->longitude);

        // optional<float32>: a presence flag, then the value if present.
        $out .= $this->humidityPct === null
            ? pack('C', 0)
            : pack('C', 1) . pack('g', $this->humidityPct);

        foreach ($this->accelMg as $v) {
            $out .= pack('v', $v & 0xFFFF);
        }

        $out .= pack('V', count($this->visibleCells));
        foreach ($this->visibleCells as $v) {
            $out .= pack('V', $v);
        }

        // Entry order is the array's own — deliberately not sorted. A map is
        // unordered, so telemetry declares `oracle: semantic` and the decoded
        // value is what gets compared. See docs/protocol.md.
        $out .= pack('V', count($this->packetCounts));
        foreach ($this->packetCounts as $k => $v) {
            $out .= lenPrefixed((string) $k) . encodeInt((string) $v, 8);
        }

        $out .= pack('C', $this->gpsFix ? 1 : 0);
        $out .= lenPrefixed($this->signature);

        return $out;
    }

    public static function unmarshal(string $data): self
    {
        $t = new self();
        $off = 0;

        $t->deviceId = decodeUnsigned(substr($data, $off, 8));
        $off += 8;
        $t->ipv6 = decodeUnsigned(substr($data, $off, 16));
        $off += 16;

        $t->localIp = array_values(unpack('C4', substr($data, $off, 4)));
        $off += 4;

        $t->firmware = readLenPrefixed($data, $off);

        $t->bootCount = unpack('v', substr($data, $off, 2))[1];
        $off += 2;
        $t->rssiDbm = unpack('c', substr($data, $off, 1))[1];
        $off += 1;
        $t->temperatureDc = (int) decodeSigned(substr($data, $off, 2));
        $off += 2;
        $t->clockDriftMs = (int) decodeSigned(substr($data, $off, 4));
        $off += 4;
        $t->batteryVolts = unpack('g', substr($data, $off, 4))[1];
        $off += 4;
        $t->latitude = unpack('e', substr($data, $off, 8))[1];
        $off += 8;
        $t->longitude = unpack('e', substr($data, $off, 8))[1];
        $off += 8;

        if (unpack('C', substr($data, $off, 1))[1] === 0) {
            $t->humidityPct = null;
            $off += 1;
        } else {
            $t->humidityPct = unpack('g', substr($data, $off + 1, 4))[1];
            $off += 5;
        }

        $t->accelMg = [];
        for ($i = 0; $i < 3; $i++) {
            $t->accelMg[] = (int) decodeSigned(substr($data, $off, 2));
            $off += 2;
        }

        $cells = unpack('V', substr($data, $off, 4))[1];
        $off += 4;
        $t->visibleCells = [];
        for ($i = 0; $i < $cells; $i++) {
            $t->visibleCells[] = unpack('V', substr($data, $off, 4))[1];
            $off += 4;
        }

        $entries = unpack('V', substr($data, $off, 4))[1];
        $off += 4;
        $t->packetCounts = [];
        for ($i = 0; $i < $entries; $i++) {
            $k = readLenPrefixed($data, $off);
            $t->packetCounts[$k] = decodeUnsigned(substr($data, $off, 8));
            $off += 8;
        }

        $t->gpsFix = unpack('C', substr($data, $off, 1))[1] !== 0;
        $off += 1;

        $t->signature = readLenPrefixed($data, $off);

        return $t;
    }
}
