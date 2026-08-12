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
 * `CustomerRecord` mirrors examples/cases/customer.yaml — a store account.
 *
 * `Address` and `Money` mirror the reusable address.yaml and money.yaml it
 * imports; order.php reuses both, as the Go worker does.
 *
 * This is the only type in the suite carrying two formats, and the second one
 * is the point: `binary` is a layout written by hand below, `json` goes through
 * json_encode, so the two fail in completely different ways. Both declare
 * `oracle: semantic`, so what has to match is the decoded value rather than the
 * bytes — Go's encoder HTML-escapes `<`, `>` and `&` and always escapes
 * U+2028/U+2029, and under a byte oracle every worker would have to reproduce
 * that quirk.
 *
 * PHP's int is 64-bit *signed*, so a uint64 does not fit and `$customerId`
 * travels as a decimal string through ext-gmp, as the other models here do. The
 * same limit is what makes the JSON side awkward, since json_encode would then
 * quote it; see the two helpers at the bottom.
 *
 * It is also the first PHP model with nested structs. A property typed with
 * another #[SerifyModel] needs no declaration — its own type says which class
 * it is — but `array` says nothing about what it holds, so the list and map
 * properties name their element class with `elem:`.
 *
 * Go is the --ref language and owns the byte layout; see examples/go/wire.go.
 */

declare(strict_types=1);

use Serify\Attributes\SerifyField;
use Serify\Attributes\SerifyModel;

#[SerifyModel]
class Address
{
    #[SerifyField] public string $recipient = '';
    #[SerifyField] public string $street = '';
    #[SerifyField] public string $city = '';
    #[SerifyField] public string $country = '';
    #[SerifyField] public string $postalCode = '';

    /** A struct is its fields back to back, in schema order — nothing frames it. */
    public function pack(): string
    {
        return lenPrefixed($this->recipient) . lenPrefixed($this->street)
            . lenPrefixed($this->city) . lenPrefixed($this->country)
            . lenPrefixed($this->postalCode);
    }

    public static function unpack(string $data, int &$off): self
    {
        $a = new self();
        $a->recipient = readLenPrefixed($data, $off);
        $a->street = readLenPrefixed($data, $off);
        $a->city = readLenPrefixed($data, $off);
        $a->country = readLenPrefixed($data, $off);
        $a->postalCode = readLenPrefixed($data, $off);
        return $a;
    }

    public function toArray(): array
    {
        return [
            'recipient' => $this->recipient, 'street' => $this->street,
            'city' => $this->city, 'country' => $this->country,
            'postal_code' => $this->postalCode,
        ];
    }

    public static function fromArray(array $o): self
    {
        $a = new self();
        $a->recipient = $o['recipient'];
        $a->street = $o['street'];
        $a->city = $o['city'];
        $a->country = $o['country'];
        $a->postalCode = $o['postal_code'];
        return $a;
    }
}

#[SerifyModel]
class Money
{
    #[SerifyField] public string $currency = '';
    #[SerifyField] public string $amountMinor = '0';

    public function pack(): string
    {
        return lenPrefixed($this->currency) . encodeInt($this->amountMinor, 8);
    }

    public static function unpack(string $data, int &$off): self
    {
        $m = new self();
        $m->currency = readLenPrefixed($data, $off);
        $m->amountMinor = decodeSigned(substr($data, $off, 8));
        $off += 8;
        return $m;
    }

    public function toArray(): array
    {
        return ['currency' => $this->currency, 'amount_minor' => $this->amountMinor];
    }

    public static function fromArray(array $o): self
    {
        $m = new self();
        $m->currency = $o['currency'];
        $m->amountMinor = (string) $o['amount_minor'];
        return $m;
    }
}

#[SerifyModel]
class CustomerRecord
{
    #[SerifyField] public string $customerId = '0';
    #[SerifyField] public string $email = '';
    #[SerifyField] public string $displayName = '';
    #[SerifyField] public int $age = 0;
    #[SerifyField] public bool $emailVerified = false;
    #[SerifyField] public float $fraudScore = 0.0;
    #[SerifyField] public int $loyaltyPoints = 0;
    #[SerifyField] public string $signupTs = '0';
    #[SerifyField] public string $avatarSha256 = '';
    #[SerifyField] public array $pin = [];
    #[SerifyField] public ?string $referralCode = null;
    #[SerifyField] public Money $storeCredit;
    #[SerifyField(elem: Address::class)] public array $shippingAddresses = [];
    #[SerifyField(elem: Address::class)] public array $addressBook = [];
    #[SerifyField] public array $wishlistSkus = [];
    #[SerifyField] public array $preferences = [];

    public function __construct()
    {
        $this->storeCredit = new Money();
    }

    public function marshal(): string
    {
        $out = encodeInt($this->customerId, 8);
        $out .= lenPrefixed($this->email);
        $out .= lenPrefixed($this->displayName);
        $out .= pack('C', $this->age);
        $out .= pack('C', $this->emailVerified ? 1 : 0);
        $out .= pack('g', $this->fraudScore);
        $out .= pack('V', $this->loyaltyPoints);
        $out .= encodeInt($this->signupTs, 8);

        $out .= lenPrefixed($this->avatarSha256);

        // array<T,N> carries no count: N is fixed by the schema.
        foreach ($this->pin as $b) {
            $out .= pack('C', $b);
        }

        // optional<string>: a presence flag, then the value if present. An empty
        // string is present, which is why the flag cannot be inferred from it.
        if ($this->referralCode === null) {
            $out .= pack('C', 0);
        } else {
            $out .= pack('C', 1) . lenPrefixed($this->referralCode);
        }

        $out .= $this->storeCredit->pack();

        $out .= pack('V', count($this->shippingAddresses));
        foreach ($this->shippingAddresses as $a) {
            $out .= $a->pack();
        }

        // Entry order is the array's own — deliberately not sorted. A map is
        // unordered, so customer declares `oracle: semantic` and the decoded
        // value is what gets compared. See docs/protocol.md.
        $out .= pack('V', count($this->addressBook));
        foreach ($this->addressBook as $k => $a) {
            $out .= lenPrefixed((string) $k) . $a->pack();
        }

        $out .= pack('V', count($this->wishlistSkus));
        foreach ($this->wishlistSkus as $s) {
            $out .= lenPrefixed($s);
        }

        $out .= pack('V', count($this->preferences));
        foreach ($this->preferences as $k => $v) {
            $out .= lenPrefixed((string) $k) . lenPrefixed($v);
        }

        return $out;
    }

    public static function unmarshal(string $data): self
    {
        $c = new self();
        $off = 0;

        $c->customerId = decodeUnsigned(substr($data, 0, 8));
        $off = 8;
        $c->email = readLenPrefixed($data, $off);
        $c->displayName = readLenPrefixed($data, $off);

        $c->age = unpack('C', $data, $off)[1];
        $c->emailVerified = unpack('C', $data, $off + 1)[1] !== 0;
        $c->fraudScore = unpack('g', $data, $off + 2)[1];
        $c->loyaltyPoints = unpack('V', $data, $off + 6)[1];
        $c->signupTs = decodeSigned(substr($data, $off + 10, 8));
        $off += 18;

        $c->avatarSha256 = readLenPrefixed($data, $off);

        $c->pin = array_values(unpack('C4', substr($data, $off, 4)));
        $off += 4;

        if (unpack('C', $data, $off)[1] === 0) {
            $c->referralCode = null;
            $off += 1;
        } else {
            $off += 1;
            $c->referralCode = readLenPrefixed($data, $off);
        }

        $c->storeCredit = Money::unpack($data, $off);

        $c->shippingAddresses = [];
        for ($n = self::readCount($data, $off); $n > 0; $n--) {
            $c->shippingAddresses[] = Address::unpack($data, $off);
        }

        $c->addressBook = [];
        for ($n = self::readCount($data, $off); $n > 0; $n--) {
            $k = readLenPrefixed($data, $off);
            $c->addressBook[$k] = Address::unpack($data, $off);
        }

        $c->wishlistSkus = [];
        for ($n = self::readCount($data, $off); $n > 0; $n--) {
            $c->wishlistSkus[] = readLenPrefixed($data, $off);
        }

        $c->preferences = [];
        for ($n = self::readCount($data, $off); $n > 0; $n--) {
            $k = readLenPrefixed($data, $off);
            $c->preferences[$k] = readLenPrefixed($data, $off);
        }

        return $c;
    }

    private static function readCount(string $data, int &$off): int
    {
        $n = unpack('V', $data, $off)[1];
        $off += 4;
        return $n;
    }

    /**
     * `bytes` is base64 and the 64-bit integers are JSON numbers: that is what
     * the reference worker's `[]byte` and `uint64` marshal to, and the semantic
     * oracle decodes our output with it.
     */
    public function toJson(): string
    {
        $book = [];
        foreach ($this->addressBook as $k => $a) {
            $book[(string) $k] = $a->toArray();
        }

        $o = [
            'customer_id' => $this->customerId,
            'email' => $this->email,
            'display_name' => $this->displayName,
            'age' => $this->age,
            'email_verified' => $this->emailVerified,
            'fraud_score' => $this->fraudScore,
            'loyalty_points' => $this->loyaltyPoints,
            'signup_ts' => $this->signupTs,
            'avatar_sha256' => base64_encode($this->avatarSha256),
            'pin' => $this->pin,
            'referral_code' => $this->referralCode,
            'store_credit' => $this->storeCredit->toArray(),
            'shipping_addresses' => array_map(fn($a) => $a->toArray(), $this->shippingAddresses),
            // An empty PHP array encodes as [], not {}, and Go's decoder wants an
            // object here. JSON_FORCE_OBJECT would apply to every array in the
            // document, including the lists, so the cast is per-field.
            'address_book' => (object) $book,
            'wishlist_skus' => array_values($this->wishlistSkus),
            'preferences' => (object) $this->preferences,
        ];

        return unquoteBigints(json_encode($o, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE));
    }

    public static function fromJson(string $data): self
    {
        $o = json_decode($data, true, 512, JSON_BIGINT_AS_STRING);
        $c = new self();

        // Every 64-bit field arrives as a string or an int depending on whether
        // it happened to fit in PHP's signed int; the model wants the decimal
        // string either way.
        $c->customerId = (string) $o['customer_id'];
        $c->email = $o['email'];
        $c->displayName = $o['display_name'];
        $c->age = $o['age'];
        $c->emailVerified = $o['email_verified'];
        // A JSON number is a double; narrow it the way the wire does, so a
        // float32 field holds a value float32 can actually represent.
        $c->fraudScore = unpack('g', pack('g', (float) $o['fraud_score']))[1];
        $c->loyaltyPoints = $o['loyalty_points'];
        $c->signupTs = (string) $o['signup_ts'];
        $c->avatarSha256 = base64_decode($o['avatar_sha256'] ?? '');
        $c->pin = $o['pin'];
        $c->referralCode = $o['referral_code'];
        $c->storeCredit = Money::fromArray($o['store_credit']);
        $c->shippingAddresses = array_map([Address::class, 'fromArray'], $o['shipping_addresses'] ?? []);
        $c->addressBook = array_map([Address::class, 'fromArray'], $o['address_book'] ?? []);
        $c->wishlistSkus = $o['wishlist_skus'] ?? [];
        $c->preferences = $o['preferences'] ?? [];

        return $c;
    }
}

// 64-bit integers in JSON
//
// PHP's int is signed, so max uint64 has no int form at all and json_encode
// would have to quote it. Reading is handled by JSON_BIGINT_AS_STRING; writing
// has no equivalent, so the three 64-bit fields are encoded as strings and
// unquoted in the text. They are the only 64-bit fields customer has.
//
// This rewrites the text rather than the value tree, so a *string value*
// spelling one of these keys followed by a colon and digits would be unquoted
// too. No case produces one, and the escape-hunting cases cannot: a `"` inside
// a JSON string is written `\"`.
const BIG_KEYS = 'customer_id|signup_ts|amount_minor';

function unquoteBigints(string $text): string
{
    return preg_replace('/"(' . BIG_KEYS . ')":"(-?\d+)"/', '"$1":$2', $text);
}
