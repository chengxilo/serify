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
	"encoding/binary"
	"fmt"
	"slices"
)

// LineItem mirrors examples/cases/line_item.yaml, imported by order.
type LineItem struct {
	SKU         string `serify:"sku"`
	ProductName string `serify:"product_name"`
	Quantity    uint16 `serify:"quantity"`
	UnitPrice   Money  `serify:"unit_price"`
	DiscountPct uint8  `serify:"discount_pct"`
	GiftWrap    bool   `serify:"gift_wrap"`
}

// orderStatuses is the declaration order of the `status` enum in
// examples/cases/order.yaml. An enum travels as its variant *name*; the ordinal
// below is this worker's own byte-layout choice, so the list has to match the
// case file's order — serify.EnumVariants(type) exposes it for a worker that
// would rather derive it than restate it.
var orderStatuses = []string{"pending", "paid", "shipped", "delivered", "cancelled"}

// OrderRecord mirrors examples/cases/order.yaml. Between them its fields cover
// the composite types nothing else in the suite exercises end to end: an enum, a
// list of structs, a map of structs, and an optional struct.
type OrderRecord struct {
	OrderID         uint64           `serify:"order_id"`
	CustomerID      uint64           `serify:"customer_id"`
	CreatedAt       int64            `serify:"created_at"`
	Status          string           `serify:"status"`
	Items           []LineItem       `serify:"items"`
	Subtotal        Money            `serify:"subtotal"`
	Adjustments     map[string]Money `serify:"adjustments"`
	Total           Money            `serify:"total"`
	ShippingAddress Address          `serify:"shipping_address"`
	BillingAddress  *Address         `serify:"billing_address"`
	CouponCodes     []string         `serify:"coupon_codes"`
	TrackingNumber  *string          `serify:"tracking_number"`
}

func appendMoney(buf []byte, m Money) []byte {
	buf = appendLenStr(buf, m.Currency)
	return binary.LittleEndian.AppendUint64(buf, uint64(m.AmountMinor))
}

func readMoney(b []byte) (Money, []byte, error) {
	var m Money
	var err error
	if m.Currency, b, err = readLenStr(b); err != nil {
		return m, b, err
	}
	if len(b) < 8 {
		return m, b, errTruncated
	}
	m.AmountMinor = int64(binary.LittleEndian.Uint64(b))
	return m, b[8:], nil
}

func (o *OrderRecord) MarshalBinary() ([]byte, error) {
	buf := binary.LittleEndian.AppendUint64(nil, o.OrderID)
	buf = binary.LittleEndian.AppendUint64(buf, o.CustomerID)
	buf = binary.LittleEndian.AppendUint64(buf, uint64(o.CreatedAt))

	// enum: a u8 ordinal, the variant's position in the case file.
	ord := slices.Index(orderStatuses, o.Status)
	if ord < 0 {
		return nil, fmt.Errorf("unknown order status %q", o.Status)
	}
	buf = append(buf, byte(ord))

	buf = appendCount(buf, o.Items)
	for _, it := range o.Items {
		buf = appendLenStr(buf, it.SKU)
		buf = appendLenStr(buf, it.ProductName)
		buf = binary.LittleEndian.AppendUint16(buf, it.Quantity)
		buf = appendMoney(buf, it.UnitPrice)
		buf = append(buf, it.DiscountPct)
		if it.GiftWrap {
			buf = append(buf, 1)
		} else {
			buf = append(buf, 0)
		}
	}

	buf = appendMoney(buf, o.Subtotal)

	keys := mapKeys(o.Adjustments)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(keys)))
	for _, k := range keys {
		buf = appendLenStr(buf, k)
		buf = appendMoney(buf, o.Adjustments[k])
	}

	buf = appendMoney(buf, o.Total)
	buf = appendAddress(buf, o.ShippingAddress)

	if o.BillingAddress == nil {
		buf = append(buf, 0)
	} else {
		buf = append(buf, 1)
		buf = appendAddress(buf, *o.BillingAddress)
	}

	buf = appendCount(buf, o.CouponCodes)
	for _, c := range o.CouponCodes {
		buf = appendLenStr(buf, c)
	}

	if o.TrackingNumber == nil {
		buf = append(buf, 0)
	} else {
		buf = append(buf, 1)
		buf = appendLenStr(buf, *o.TrackingNumber)
	}

	return buf, nil
}

//nolint:gocognit,funlen // one read block per field
func (o *OrderRecord) UnmarshalBinary(b []byte) error {
	if len(b) < 25 {
		return errTruncated
	}
	o.OrderID = binary.LittleEndian.Uint64(b)
	o.CustomerID = binary.LittleEndian.Uint64(b[8:])
	o.CreatedAt = int64(binary.LittleEndian.Uint64(b[16:]))
	ord := int(b[24])
	if ord >= len(orderStatuses) {
		return fmt.Errorf("unknown order status ordinal %d", ord)
	}
	o.Status = orderStatuses[ord]
	b = b[25:]

	n, b, err := readCount(b)
	if err != nil {
		return err
	}
	o.Items = make([]LineItem, n)
	for i := range n {
		var it LineItem
		if it.SKU, b, err = readLenStr(b); err != nil {
			return err
		}
		if it.ProductName, b, err = readLenStr(b); err != nil {
			return err
		}
		if len(b) < 2 {
			return errTruncated
		}
		it.Quantity = binary.LittleEndian.Uint16(b)
		b = b[2:]
		if it.UnitPrice, b, err = readMoney(b); err != nil {
			return err
		}
		if len(b) < 2 {
			return errTruncated
		}
		it.DiscountPct = b[0]
		it.GiftWrap = b[1] != 0
		b = b[2:]
		o.Items[i] = it
	}

	if o.Subtotal, b, err = readMoney(b); err != nil {
		return err
	}

	if n, b, err = readCount(b); err != nil {
		return err
	}
	o.Adjustments = make(map[string]Money, n)
	for range n {
		var k string
		var m Money
		if k, b, err = readLenStr(b); err != nil {
			return err
		}
		if m, b, err = readMoney(b); err != nil {
			return err
		}
		o.Adjustments[k] = m
	}

	if o.Total, b, err = readMoney(b); err != nil {
		return err
	}
	if o.ShippingAddress, b, err = readAddress(b); err != nil {
		return err
	}

	if len(b) < 1 {
		return errTruncated
	}
	present := b[0]
	b = b[1:]
	o.BillingAddress = nil
	if present != 0 {
		var a Address
		if a, b, err = readAddress(b); err != nil {
			return err
		}
		o.BillingAddress = &a
	}

	if n, b, err = readCount(b); err != nil {
		return err
	}
	o.CouponCodes = make([]string, n)
	for i := range n {
		if o.CouponCodes[i], b, err = readLenStr(b); err != nil {
			return err
		}
	}

	if len(b) < 1 {
		return errTruncated
	}
	present = b[0]
	b = b[1:]
	o.TrackingNumber = nil
	if present != 0 {
		var s string
		if s, b, err = readLenStr(b); err != nil {
			return err
		}
		o.TrackingNumber = &s
	}

	return nil
}
