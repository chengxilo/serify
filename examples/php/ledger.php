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
 * LedgerEntry mirrors examples/cases/ledger.yaml.
 *
 * The attributes are the entire schema binding — nothing here calls a get/set
 * accessor. Everything else is the byte layout, which is the part a conformance
 * worker exists to exercise.
 *
 * PHP's int is only 64-bit, so every 64/128-bit value is carried as a decimal
 * string and converted through the GMP helpers in wire.php.
 *
 * Go is the --ref language and owns the layout; see examples/go/wire.go.
 */

declare(strict_types=1);

use Serify\Attributes\SerifyField;
use Serify\Attributes\SerifyModel;

#[SerifyModel]
class LedgerEntry
{
    #[SerifyField] public string $entryId = '0';
    #[SerifyField] public string $blockNumber = '0';
    #[SerifyField] public string $blockTime = '0';
    #[SerifyField] public string $txHash = '';
    #[SerifyField] public string $account = '';
    #[SerifyField] public string $asset = '';
    #[SerifyField] public string $amountBaseUnits = '0';
    #[SerifyField] public string $balanceAfter = '0';
    #[SerifyField] public bool $confirmed = false;
    #[SerifyField] public ?string $memo = null;

    public function marshal(): string
    {
        $out = encodeInt($this->entryId, 8)
             . encodeInt($this->blockNumber, 8)
             . encodeInt($this->blockTime, 8);

        $out .= lenPrefixed($this->txHash);
        $out .= lenPrefixed($this->account);
        $out .= lenPrefixed($this->asset);

        $out .= encodeInt($this->amountBaseUnits, 16);
        $out .= encodeInt($this->balanceAfter, 16);
        $out .= pack('C', $this->confirmed ? 1 : 0);

        if ($this->memo === null) {
            $out .= pack('C', 0);
        } else {
            $out .= pack('C', 1) . lenPrefixed($this->memo);
        }

        return $out;
    }

    public static function unmarshal(string $data): self
    {
        $e = new self();

        $e->entryId     = decodeUnsigned(substr($data, 0, 8));
        $e->blockNumber = decodeUnsigned(substr($data, 8, 8));
        $e->blockTime   = decodeSigned(substr($data, 16, 8));
        $off = 24;

        $e->txHash  = readLenPrefixed($data, $off);
        $e->account = readLenPrefixed($data, $off);
        $e->asset   = readLenPrefixed($data, $off);

        $e->amountBaseUnits = decodeSigned(substr($data, $off, 16));
        $e->balanceAfter    = decodeSigned(substr($data, $off + 16, 16));
        $off += 32;

        $e->confirmed = ord($data[$off]) !== 0;
        $hasMemo = ord($data[$off + 1]) !== 0;
        $off += 2;
        $e->memo = $hasMemo ? readLenPrefixed($data, $off) : null;

        return $e;
    }
}
