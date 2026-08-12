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
 * `OrderRecord` mirrors examples/cases/order.yaml — a placed order.
 *
 * `LineItem` mirrors the reusable line_item.yaml it imports, which itself nests
 * money — so any type using it exercises struct-inside-struct. `Address` and
 * `Money` come from customer.php, as they do in the Go worker.
 *
 * Between them the fields cover the four composite types nothing else in the
 * suite exercises end to end: an `enum`, a `list<struct>`, a
 * `map<string,struct>` and an `optional<struct>`. `$billingAddress` is the
 * suite's only optional<struct>, and unlike the list and map beside it, it
 * needs no `elem:` — its own property type says which class it is.
 *
 * An enum needs nothing from the binding: it travels as its variant *name*, so
 * the property is a plain string. The u8 ordinal in the layout is this worker's
 * own choice, which is why STATUSES has to match the case file's declaration
 * order.
 *
 * PHP's int is 64-bit *signed*, so the two u64 ids travel as decimal strings
 * through ext-gmp, as the other models here do.
 *
 * Go is the --ref language and owns the layout; see examples/go/wire.go.
 */

declare(strict_types=1);

use Serify\Attributes\SerifyField;
use Serify\Attributes\SerifyModel;

/** Declaration order of the `status` enum in examples/cases/order.yaml. */
const ORDER_STATUSES = ['pending', 'paid', 'shipped', 'delivered', 'cancelled'];

#[SerifyModel]
class LineItem
{
    #[SerifyField] public string $sku = '';
    #[SerifyField] public string $productName = '';
    #[SerifyField] public int $quantity = 0;
    #[SerifyField] public Money $unitPrice;
    #[SerifyField] public int $discountPct = 0;
    #[SerifyField] public bool $giftWrap = false;

    public function __construct()
    {
        $this->unitPrice = new Money();
    }

    public function pack(): string
    {
        return lenPrefixed($this->sku)
            . lenPrefixed($this->productName)
            . pack('v', $this->quantity)
            . $this->unitPrice->pack()
            . pack('CC', $this->discountPct, $this->giftWrap ? 1 : 0);
    }

    public static function unpack(string $data, int &$off): self
    {
        $it = new self();
        $it->sku = readLenPrefixed($data, $off);
        $it->productName = readLenPrefixed($data, $off);
        $it->quantity = unpack('v', $data, $off)[1];
        $off += 2;
        $it->unitPrice = Money::unpack($data, $off);
        $it->discountPct = unpack('C', $data, $off)[1];
        $it->giftWrap = unpack('C', $data, $off + 1)[1] !== 0;
        $off += 2;
        return $it;
    }
}

#[SerifyModel]
class OrderRecord
{
    #[SerifyField] public string $orderId = '0';
    #[SerifyField] public string $customerId = '0';
    #[SerifyField] public string $createdAt = '0';
    #[SerifyField] public string $status = '';
    #[SerifyField(elem: LineItem::class)] public array $items = [];
    #[SerifyField] public Money $subtotal;
    #[SerifyField(elem: Money::class)] public array $adjustments = [];
    #[SerifyField] public Money $total;
    #[SerifyField] public Address $shippingAddress;
    #[SerifyField] public ?Address $billingAddress = null;
    #[SerifyField] public array $couponCodes = [];
    #[SerifyField] public ?string $trackingNumber = null;

    public function __construct()
    {
        $this->subtotal = new Money();
        $this->total = new Money();
        $this->shippingAddress = new Address();
    }

    public function marshal(): string
    {
        $out = encodeInt($this->orderId, 8)
            . encodeInt($this->customerId, 8)
            . encodeInt($this->createdAt, 8);

        // enum: a u8 ordinal, the variant's position in the case file.
        $ord = array_search($this->status, ORDER_STATUSES, true);
        if ($ord === false) {
            throw new RuntimeException("unknown order status \"{$this->status}\"");
        }
        $out .= pack('C', $ord);

        $out .= pack('V', count($this->items));
        foreach ($this->items as $it) {
            $out .= $it->pack();
        }

        $out .= $this->subtotal->pack();

        // Entry order is the array's own — deliberately not sorted. A map is
        // unordered, so order declares `oracle: semantic` and the decoded value
        // is what gets compared. See docs/protocol.md.
        $out .= pack('V', count($this->adjustments));
        foreach ($this->adjustments as $k => $m) {
            $out .= lenPrefixed((string) $k) . $m->pack();
        }

        $out .= $this->total->pack();
        $out .= $this->shippingAddress->pack();

        // optional<struct>: a presence flag, then the struct's fields inline.
        if ($this->billingAddress === null) {
            $out .= pack('C', 0);
        } else {
            $out .= pack('C', 1) . $this->billingAddress->pack();
        }

        $out .= pack('V', count($this->couponCodes));
        foreach ($this->couponCodes as $c) {
            $out .= lenPrefixed($c);
        }

        if ($this->trackingNumber === null) {
            $out .= pack('C', 0);
        } else {
            $out .= pack('C', 1) . lenPrefixed($this->trackingNumber);
        }

        return $out;
    }

    public static function unmarshal(string $data): self
    {
        $o = new self();

        $o->orderId = decodeUnsigned(substr($data, 0, 8));
        $o->customerId = decodeUnsigned(substr($data, 8, 8));
        $o->createdAt = decodeSigned(substr($data, 16, 8));

        $ord = unpack('C', $data, 24)[1];
        if (!isset(ORDER_STATUSES[$ord])) {
            throw new RuntimeException("status ordinal $ord is out of range");
        }
        $o->status = ORDER_STATUSES[$ord];
        $off = 25;

        $o->items = [];
        for ($n = self::readCount($data, $off); $n > 0; $n--) {
            $o->items[] = LineItem::unpack($data, $off);
        }

        $o->subtotal = Money::unpack($data, $off);

        $o->adjustments = [];
        for ($n = self::readCount($data, $off); $n > 0; $n--) {
            $k = readLenPrefixed($data, $off);
            $o->adjustments[$k] = Money::unpack($data, $off);
        }

        $o->total = Money::unpack($data, $off);
        $o->shippingAddress = Address::unpack($data, $off);

        if (unpack('C', $data, $off)[1] === 0) {
            $o->billingAddress = null;
            $off += 1;
        } else {
            $off += 1;
            $o->billingAddress = Address::unpack($data, $off);
        }

        $o->couponCodes = [];
        for ($n = self::readCount($data, $off); $n > 0; $n--) {
            $o->couponCodes[] = readLenPrefixed($data, $off);
        }

        if (unpack('C', $data, $off)[1] === 0) {
            $o->trackingNumber = null;
        } else {
            $off += 1;
            $o->trackingNumber = readLenPrefixed($data, $off);
        }

        return $o;
    }

    private static function readCount(string $data, int &$off): int
    {
        $n = unpack('V', $data, $off)[1];
        $off += 4;
        return $n;
    }
}
