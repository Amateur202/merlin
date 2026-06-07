# Operators

## Arithmetic

| Operator | Description |
|---|---|
| `+` | Addition, string concatenation |
| `-` | Subtraction |
| `*` | Multiplication |
| `/` | Division |
| `%` | Modulo |

## Comparison

| Operator | Description |
|---|---|
| `==` | Equal |
| `!=` | Not equal |
| `<` | Less than |
| `>` | Greater than |
| `<=` | Less than or equal |
| `>=` | Greater than or equal |

String comparison (`==`, `!=`) uses `strcmp` under the hood.

## Logical

Merlin uses English keywords instead of symbols:

| Operator | Description | C Equivalent |
|---|---|---|
| `and` | Logical AND | `&&` |
| `or` | Logical OR | `||` |
| `not` | Logical NOT | `!` |

## Bitwise

| Operator | Description |
|---|---|
| `&` | AND (also address-of in type context) |
| `|` | OR |
| `^` | XOR |
| `<<` | Left shift |
| `>>` | Right shift |
| `~` | NOT (C emited as `~`) |

## Membership (`in`)

Tests if a value is in a collection:

```merlin
if 'A' in "HELLO":          # char in string (uses strchr)
if "wor" in "hello world":  # substring in string (uses strstr)
if x in my_array:           # value in array (linear search)
```

## String Concatenation

The `+` operator concatenates strings:

```merlin
string greeting = "Hello, " + "World!"
```

## Type Cast

```
(type)expression
```

```merlin
int    truncd = (int)3.14159
int8   forced = (int8)large
Feet   ceiling = (Feet)distance
int    raw   = (int)distance
```

## Type Conversion Functions

```merlin
string buf  = "8192"
int    port = int(buf)    # string to int

int    sid  = 65
string s1   = string(sid) # int to string
string s2   = string('M') # char to string
```

## Referenced Files

- `examples/primitives.mrl` — all operators demonstrated
- `examples/strings.mrl` — casting and conversion
