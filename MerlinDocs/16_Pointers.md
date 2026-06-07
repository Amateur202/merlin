# Pointers

## Pointer Type Syntax

Pointers are declared with `&` before the type:

```merlin
&int ptr    # Pointer to int
&T data     # Pointer to T (in template context)
```

## Address-of Operator

```merlin
int  value = 777
&int ptr   = &value
```

## Dereference Operator

```merlin
int read = *ptr    # Read through pointer
*ptr = 999         # Write through pointer
```

## Auto-Dereference with Structs

For struct pointers, use dot notation directly (no `->`):

```merlin
Temp  block  = Temp{level: 45}
&Temp tptr   = &block
tptr.level   = 52  # Auto-dereferenced — no -> needed
```

## Volatile Pointers

For hardware-mapped I/O registers:

```merlin
volatile &uint8 uart = (volatile &uint8)0x3F8
```

This generates `volatile uint8_t*` in C.

## Pointer Parameters

Functions can take pointer parameters for mutation:

```merlin
void set_value(&int target, int val):
    *target = val
```

## Pointer Return Types

```merlin
&int get_ref(&int p):
    return p
```

## Null Pointers

Pointers can be set to zero:

```merlin
&T ptr = (&T)0
```

## Referenced Files

- `examples/pointers.mrl` — pointer read/write, struct auto-deref, volatile
- `examples/advanced.mrl` — pointer demo in advanced context
