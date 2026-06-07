# External Functions

Call C functions from Merlin with `external`.

## Syntax

```
external <return_type> <name>(<params>)
external <return_type> <name>(<params>) link "<lib>"
```

## Basic External Declaration

```merlin
external void free(&void ptr)
```

## External with Link Library

```merlin
external float sqrt(float x) link "m"
external float sin(float x) link "m"
```

The `link` string specifies the C library to link against (e.g., `"m"` for `libm.so`).

## Pointer Return Types

```merlin
external &void malloc(uint64 size)
external &void calloc(uint64 count, uint64 size)
external &void realloc(&void ptr, uint64 size)
```

## Private External Functions

```merlin
private external void internal_helper()
```

## Usage

Once declared, external functions can be called like normal Merlin functions:

```merlin
float x = sqrt(9.0)
float y = sin(PI / 2.0)
```

## Standard Library Pattern

The `std/math/math.mrl` module uses `external` declarations to wrap C's `libm`:

```merlin
external float sqrt(float x) link "m"
external float sin(float x) link "m"
external float cos(float x) link "m"
```

The `std/mem/mem.mrl` module wraps `stdlib.h` functions:

```merlin
external &void malloc(uint64 size)
external void free(&void ptr)
```

## Referenced Files

- `std/math/math.mrl` — external math functions with `link "m"`
- `std/mem/mem.mrl` — external memory functions
