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

namespace Serify;

// Same reason as Type.php: these are this file's own dependencies — the two
// attributes it reflects on, and the FieldMap it returns — and there is no
// autoloader in a worker to fetch them.
require_once __DIR__ . '/FieldMap.php';
require_once __DIR__ . '/Attributes/SerifyModel.php';
require_once __DIR__ . '/Attributes/SerifyField.php';

use Serify\Attributes\SerifyModel;
use Serify\Attributes\SerifyField;

/**
 * Reflection helpers for #[SerifyModel] / #[SerifyField] classes.
 *
 * Usage:
 *   $fm = SerifyModelHelper::toFieldMap($user);
 *   $user = SerifyModelHelper::fromFieldMap(FieldMap $fm, User::class);
 */
class SerifyModelHelper
{
    /**
     * Default schema key for a property: `entryId` becomes `entry_id`.
     * PHP properties are camelCase and schema keys are snake_case, so bridging
     * the two is the library's job — pass a name to #[SerifyField] to override.
     */
    private static function defaultKey(string $prop): string
    {
        return strtolower(preg_replace('/(?<!^)[A-Z]/', '_$0', $prop));
    }

    /**
     * Convert a #[SerifyModel] instance to a FieldMap.
     */
    public static function toFieldMap(object $obj): FieldMap
    {
        $fm = new FieldMap();
        $ref = new \ReflectionClass($obj);

        foreach ($ref->getProperties(\ReflectionProperty::IS_PUBLIC) as $prop) {
            $attrs = $prop->getAttributes(SerifyField::class);
            if (empty($attrs)) {
                continue;
            }
            /** @var SerifyField $attr */
            $attr = $attrs[0]->newInstance();
            $key = $attr->name ?? self::defaultKey($prop->getName());
            $val = $prop->isInitialized($obj) ? $prop->getValue($obj) : null;
            // A union type is PHP's sum type, so a property declared with one is
            // a sum — see toVariant.
            if ($prop->getType() instanceof \ReflectionUnionType) {
                $fm->setRaw($key, self::toVariant($prop->getType(), $key, $val));
            } else {
                self::setFieldMapValue($fm, $key, $val);
            }
        }

        return $fm;
    }

    /**
     * Populate a #[SerifyModel] from a FieldMap.
     */
    public static function fromFieldMap(FieldMap $fm, string $class): object
    {
        $obj = (new \ReflectionClass($class))->newInstanceWithoutConstructor();
        $ref = new \ReflectionClass($obj);

        foreach ($ref->getProperties(\ReflectionProperty::IS_PUBLIC) as $prop) {
            $attrs = $prop->getAttributes(SerifyField::class);
            if (empty($attrs)) {
                continue;
            }
            /** @var SerifyField $attr */
            $attr = $attrs[0]->newInstance();
            $key = $attr->name ?? self::defaultKey($prop->getName());

            if ($fm->has($key)) {
                $val = $fm->raw()[$key];
                if ($prop->getType() instanceof \ReflectionUnionType && $val instanceof Variant) {
                    $prop->setValue($obj, self::fromVariant($prop->getType(), $val));
                    continue;
                }
                if ($prop->hasType() && !$prop->getType()->allowsNull() && $val === null) {
                    continue;
                }
                $prop->setValue($obj, $val);
            }
        }

        return $obj;
    }

    // ── sum ───────────────────────────────────────────────────────────────
    //
    // A property union type (`public Silent|Sms|Push|Invoice $channel`) is PHP's
    // sum type, and it is all the binding needs: ReflectionUnionType names the
    // arms and each arm's own public properties give its payload. No converter,
    // no registration.
    //
    // The arity rule is the same one every serify binding uses, because a sum
    // is a sum-of-products:
    //
    //     0 properties -> a unit variant, no payload
    //     1 property   -> that property's value is the payload
    //     N properties -> the payload is a struct holding the N properties

    /** Schema tag for an arm: its short class name in snake_case. */
    private static function armTag(string $class): string
    {
        $short = substr(strrchr('\\' . $class, '\\'), 1);
        return self::defaultKey($short);
    }

    /** @return \ReflectionProperty[] */
    private static function armProps(string $class): array
    {
        return (new \ReflectionClass($class))->getProperties(\ReflectionProperty::IS_PUBLIC);
    }

    /** Render the active arm of a union-typed property as a Variant. */
    private static function toVariant(\ReflectionUnionType $type, string $key, mixed $val): Variant
    {
        $arms = array_map(fn($t) => $t->getName(), $type->getTypes());
        if (!is_object($val) || !in_array(get_class($val), $arms, true)) {
            $got = is_object($val) ? get_class($val) : get_debug_type($val);
            throw new \RuntimeException("$key: $got is not one of " . implode(', ', $arms));
        }

        $arm   = get_class($val);
        $props = self::armProps($arm);

        if (count($props) === 0) {
            return new Variant(self::armTag($arm), null);          // unit variant
        }
        if (count($props) === 1) {
            $payload = $props[0]->getValue($val);
            // A single payload that is itself a model travels as a struct.
            return new Variant(self::armTag($arm),
                is_object($payload) && method_exists($payload, 'toFieldMap')
                    ? $payload->toFieldMap()
                    : $payload);
        }

        $payload = new FieldMap();                                  // N props -> a struct
        foreach ($props as $p) {
            self::setFieldMapValue($payload, self::defaultKey($p->getName()), $p->getValue($val));
        }
        return new Variant(self::armTag($arm), $payload);
    }

    /** Rebuild the active arm of a union-typed property from a Variant. */
    private static function fromVariant(\ReflectionUnionType $type, Variant $v): object
    {
        $arms = array_map(fn($t) => $t->getName(), $type->getTypes());
        foreach ($arms as $arm) {
            if (self::armTag($arm) !== $v->tag) {
                continue;
            }
            $obj   = (new \ReflectionClass($arm))->newInstanceWithoutConstructor();
            $props = self::armProps($arm);

            if (count($props) === 1) {
                $props[0]->setValue($obj, $v->value);
            } elseif (count($props) > 1) {
                if (!$v->value instanceof FieldMap) {
                    throw new \RuntimeException("variant \"{$v->tag}\" needs a struct payload");
                }
                foreach ($props as $p) {
                    $p->setValue($obj, $v->value->raw()[self::defaultKey($p->getName())] ?? null);
                }
            }
            return $obj;
        }

        $known = implode(', ', array_map([self::class, 'armTag'], $arms));
        throw new \RuntimeException("unknown variant \"{$v->tag}\" (declared: $known)");
    }

    private static function setFieldMapValue(FieldMap $fm, string $key, mixed $val): void
    {
        // A null is a field, not an absent one: it is how an optional<T> says
        // "no value", and skipping it would silently omit the key.
        if ($val === null) {
            $fm->setRaw($key, null);
            return;
        }

        if ($val instanceof Variant) {
            $fm->setRaw($key, $val);
            return;
        }

        switch (true) {
            case is_int($val):
                $fm->setI64($key, $val);
                break;
            case is_float($val):
                $fm->setF64($key, $val);
                break;
            case is_bool($val):
                $fm->setBool($key, $val);
                break;
            case is_string($val):
                $fm->setString($key, $val);
                break;
            case is_array($val) && array_is_list($val):
                // Store the list as-is: the schema, not the value, decides the
                // element type on the wire. Guessing from $val[0] meant an empty
                // list — and any list of bools, floats or byte strings — was
                // stored as though its elements were strings.
                $fm->setRaw($key, array_map(
                    fn($x) => is_object($x) && method_exists($x, 'toFieldMap') ? $x->toFieldMap() : $x,
                    $val
                ));
                break;
            case is_array($val):
                $fm->setMap($key, $val);
                break;
            case $val instanceof FieldMap:
                $fm->setStruct($key, $val);
                break;
            case is_object($val) && method_exists($val, 'toFieldMap'):
                $fm->setStruct($key, $val->toFieldMap());
                break;
            default:
                $fm->setString($key, (string) $val);
                break;
        }
    }
}
