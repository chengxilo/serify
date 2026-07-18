// Copyright 2026 Chengxi Luo
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
)

// Address, Money and CustomerRecord stand in for the types an application
// already has. Their binary layout follows the conventions documented in
// wire.go.

type Address struct {
	Recipient  string `json:"recipient"   serify:"recipient"`
	Street     string `json:"street"      serify:"street"`
	City       string `json:"city"        serify:"city"`
	Country    string `json:"country"     serify:"country"`
	PostalCode string `json:"postal_code" serify:"postal_code"`
}

type Money struct {
	Currency    string `json:"currency"     serify:"currency"`
	AmountMinor int64  `json:"amount_minor" serify:"amount_minor"`
}

// CustomerRecord mirrors examples/cases/customer.yaml. Every field carries an
// explicit serify tag rather than relying on the snake_case fallback, so the
// mapping stays obvious for names like AvatarSHA256 that do not round-trip
// through a naming convention cleanly.
type CustomerRecord struct {
	CustomerID        uint64             `json:"customer_id"        serify:"customer_id"`
	Email             string             `json:"email"              serify:"email"`
	DisplayName       string             `json:"display_name"       serify:"display_name"`
	Age               uint8              `json:"age"                serify:"age"`
	EmailVerified     bool               `json:"email_verified"     serify:"email_verified"`
	FraudScore        float32            `json:"fraud_score"        serify:"fraud_score"`
	LoyaltyPoints     uint32             `json:"loyalty_points"     serify:"loyalty_points"`
	SignupTS          int64              `json:"signup_ts"          serify:"signup_ts"`
	AvatarSHA256      []byte             `json:"avatar_sha256"      serify:"avatar_sha256"`
	Pin               [4]uint8           `json:"pin"                serify:"pin"`
	ReferralCode      *string            `json:"referral_code"      serify:"referral_code"`
	StoreCredit       Money              `json:"store_credit"       serify:"store_credit"`
	ShippingAddresses []Address          `json:"shipping_addresses" serify:"shipping_addresses"`
	AddressBook       map[string]Address `json:"address_book"       serify:"address_book"`
	WishlistSKUs      []string           `json:"wishlist_skus"      serify:"wishlist_skus"`
	Preferences       map[string]string  `json:"preferences"        serify:"preferences"`
}

func appendAddress(buf []byte, a Address) []byte {
	buf = appendLenStr(buf, a.Recipient)
	buf = appendLenStr(buf, a.Street)
	buf = appendLenStr(buf, a.City)
	buf = appendLenStr(buf, a.Country)
	return appendLenStr(buf, a.PostalCode)
}

func readAddress(b []byte) (Address, []byte, error) {
	var a Address
	var err error
	if a.Recipient, b, err = readLenStr(b); err != nil {
		return a, b, err
	}
	if a.Street, b, err = readLenStr(b); err != nil {
		return a, b, err
	}
	if a.City, b, err = readLenStr(b); err != nil {
		return a, b, err
	}
	if a.Country, b, err = readLenStr(b); err != nil {
		return a, b, err
	}
	a.PostalCode, b, err = readLenStr(b)
	return a, b, err
}

func (c *CustomerRecord) MarshalBinary() ([]byte, error) {
	var buf []byte

	buf = binary.LittleEndian.AppendUint64(buf, c.CustomerID)
	buf = appendLenStr(buf, c.Email)
	buf = appendLenStr(buf, c.DisplayName)
	buf = append(buf, c.Age)
	if c.EmailVerified {
		buf = append(buf, 1)
	} else {
		buf = append(buf, 0)
	}
	buf = binary.LittleEndian.AppendUint32(buf, math.Float32bits(c.FraudScore))
	buf = binary.LittleEndian.AppendUint32(buf, c.LoyaltyPoints)
	buf = binary.LittleEndian.AppendUint64(buf, uint64(c.SignupTS))

	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(c.AvatarSHA256)))
	buf = append(buf, c.AvatarSHA256...)

	buf = append(buf, c.Pin[:]...)

	if c.ReferralCode == nil {
		buf = append(buf, 0)
	} else {
		buf = append(buf, 1)
		buf = appendLenStr(buf, *c.ReferralCode)
	}

	buf = appendLenStr(buf, c.StoreCredit.Currency)
	buf = binary.LittleEndian.AppendUint64(buf, uint64(c.StoreCredit.AmountMinor))

	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(c.ShippingAddresses)))
	for _, a := range c.ShippingAddresses {
		buf = appendAddress(buf, a)
	}

	bookKeys := sortedKeys(c.AddressBook)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(bookKeys)))
	for _, k := range bookKeys {
		buf = appendLenStr(buf, k)
		buf = appendAddress(buf, c.AddressBook[k])
	}

	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(c.WishlistSKUs)))
	for _, s := range c.WishlistSKUs {
		buf = appendLenStr(buf, s)
	}

	prefKeys := sortedKeys(c.Preferences)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(prefKeys)))
	for _, k := range prefKeys {
		buf = appendLenStr(buf, k)
		buf = appendLenStr(buf, c.Preferences[k])
	}

	return buf, nil
}

//nolint:gocognit,funlen // one flat read per schema field, mirroring MarshalBinary
func (c *CustomerRecord) UnmarshalBinary(data []byte) error {
	b := data
	var err error

	if len(b) < 8 {
		return errTruncated
	}
	c.CustomerID = binary.LittleEndian.Uint64(b[:8])
	b = b[8:]

	if c.Email, b, err = readLenStr(b); err != nil {
		return err
	}
	if c.DisplayName, b, err = readLenStr(b); err != nil {
		return err
	}

	if len(b) < 2 {
		return errTruncated
	}
	c.Age = b[0]
	c.EmailVerified = b[1] != 0
	b = b[2:]

	if len(b) < 16 {
		return errTruncated
	}
	c.FraudScore = math.Float32frombits(binary.LittleEndian.Uint32(b[:4]))
	c.LoyaltyPoints = binary.LittleEndian.Uint32(b[4:8])
	c.SignupTS = int64(binary.LittleEndian.Uint64(b[8:16]))
	b = b[16:]

	if len(b) < 4 {
		return errTruncated
	}
	avatarLen := int(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	if len(b) < avatarLen {
		return errTruncated
	}
	c.AvatarSHA256 = make([]byte, avatarLen)
	copy(c.AvatarSHA256, b[:avatarLen])
	b = b[avatarLen:]

	if len(b) < len(c.Pin) {
		return errTruncated
	}
	copy(c.Pin[:], b[:len(c.Pin)])
	b = b[len(c.Pin):]

	if len(b) < 1 {
		return errTruncated
	}
	hasReferral := b[0] != 0
	b = b[1:]
	c.ReferralCode = nil
	if hasReferral {
		var s string
		if s, b, err = readLenStr(b); err != nil {
			return err
		}
		c.ReferralCode = &s
	}

	if c.StoreCredit.Currency, b, err = readLenStr(b); err != nil {
		return err
	}
	if len(b) < 8 {
		return errTruncated
	}
	c.StoreCredit.AmountMinor = int64(binary.LittleEndian.Uint64(b[:8]))
	b = b[8:]

	if len(b) < 4 {
		return errTruncated
	}
	shipLen := int(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	c.ShippingAddresses = make([]Address, shipLen)
	for i := range c.ShippingAddresses {
		if c.ShippingAddresses[i], b, err = readAddress(b); err != nil {
			return err
		}
	}

	if len(b) < 4 {
		return errTruncated
	}
	bookLen := int(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	c.AddressBook = make(map[string]Address, bookLen)
	for range bookLen {
		var k string
		if k, b, err = readLenStr(b); err != nil {
			return err
		}
		var a Address
		if a, b, err = readAddress(b); err != nil {
			return err
		}
		c.AddressBook[k] = a
	}

	if len(b) < 4 {
		return errTruncated
	}
	skuLen := int(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	c.WishlistSKUs = make([]string, skuLen)
	for i := range c.WishlistSKUs {
		if c.WishlistSKUs[i], b, err = readLenStr(b); err != nil {
			return err
		}
	}

	if len(b) < 4 {
		return errTruncated
	}
	prefLen := int(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	c.Preferences = make(map[string]string, prefLen)
	for range prefLen {
		var k, v string
		if k, b, err = readLenStr(b); err != nil {
			return err
		}
		if v, b, err = readLenStr(b); err != nil {
			return err
		}
		c.Preferences[k] = v
	}

	return nil
}

// marshalJSON deliberately avoids json.Marshal: it would HTML-escape <, >, &
// and U+2028/U+2029, a Go-only quirk that no other language's JSON encoder
// reproduces, so every other worker would have to replicate it to stay
// byte-identical. Encoder.Encode appends a newline, so trim it.
func marshalJSON(c *CustomerRecord) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(c); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

func unmarshalJSON(c *CustomerRecord, b []byte) error { return json.Unmarshal(b, c) }
