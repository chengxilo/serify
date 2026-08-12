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

namespace Serify\Attributes;

/**
 * Maps a property to a serify schema key. Defaults to the property name.
 *
 * Usage:
 *   #[SerifyField('user_id')]
 *   public int $userId;
 *
 * `elem` names the model class inside a list<struct> or map<K,struct>. Only the
 * way back needs it: going out, a nested model is recognised by its
 * #[SerifyModel] attribute, but PHP's `array` type says nothing about what it
 * holds, so the class of the elements has to be declared. A plain struct
 * property needs no declaration -- its own type says which class it is.
 *
 *   #[SerifyField('shipping_addresses', elem: Address::class)]
 *   public array $shippingAddresses = [];
 */
#[\Attribute(\Attribute::TARGET_PROPERTY)]
class SerifyField
{
    public function __construct(
        public readonly ?string $name = null,
        public readonly ?string $elem = null,
    ) {
    }
}
