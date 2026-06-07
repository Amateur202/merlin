# Primitive Types

Merlin provides a range of integer, floating-point, boolean, and character types. See `examples/primitives.mrl` for a complete demonstration.

## Integer Types

| Type | Size | Range | C Equivalent |
|---|---|---|---|
| `int` | word | platform-dependent | `intptr_t` |
| `int8` | 1 byte | -128 to 127 | `int8_t` |
| `int16` | 2 bytes | -32768 to 32767 | `int16_t` |
| `int32` | 4 bytes | -2^31 to 2^31-1 | `int32_t` |
| `int64` | 8 bytes | -2^63 to 2^63-1 | `int64_t` |
| `uint8` | 1 byte | 0 to 255 | `uint8_t` |
| `uint16` | 2 bytes | 0 to 65535 | `uint16_t` |
| `uint32` | 4 bytes | 0 to 4294967295 | `uint32_t` |
| `uint64` | 8 bytes | 0 to 2^64-1 | `uint64_t` |

`int` is distinct from `int32`/`int64` — it maps to `intptr_t` in C.

## Floating-Point Types

| Type | Size | Precision | C Equivalent |
|---|---|---|---|
| `float` | 8 bytes | double | `double` |
| `float32` | 4 bytes | single | `float` |
| `float64` | 8 bytes | double | `double` |

Note: `float` in Merlin maps to C `double`, not `float`.

## Boolean

| Type | Values | C Equivalent |
|---|---|---|
| `bool` | `true`, `false` | `uint8_t` |

## Character

| Type | Size | C Equivalent |
|---|---|---|
| `char` | 1 byte | `char` |

Characters can be arithmetic operands: `'A' + 1` equals `'B'`.

## Literals

```merlin
int   a = 42
int   b = 0xFF        # Hex
int   c = 0b1010      # Binary (from parser's parseInt)
float d = 3.14159
bool  e = true
char  f = 'A'
char  g = '\n'        # Escape sequences
char  h = '\x00'      # Hex escape
```

## Referenced Files

- `examples/primitives.mrl` — full demonstration of all types, operators, casting, and conversion
