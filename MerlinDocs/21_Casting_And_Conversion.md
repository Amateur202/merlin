# Casting and Conversion

## Type Casting

C-style cast syntax:

```
(type)expression
```

```merlin
# Float to int truncation
float pi     = 3.14159
int   truncd = (int)pi

# Numeric downcast
int8 forced = (int8)large

# Between type aliases
Meter distance = (Meter)1500
Feet  ceiling  = (Feet)distance

# To primitive
int raw = (int)distance
```

## Type Conversion Functions

Built-in conversion functions for string ↔ numeric:

```merlin
# String to int
string buf = "8192"
int port   = int(buf)

# Int to string
int    sid = 65
string s1  = string(sid)

# Char to string
string s2 = string('M')
```

## Pointer Casts

```merlin
volatile &uint8 uart = (volatile &uint8)0x3F8

# Typed allocator pattern
&T ptr = (&T)malloc(n * sizeof(T))
```

## Implicit Numeric Promotion

Smaller integer types promote to larger ones automatically:

```merlin
int8  small  = 24
int32 large  = 400000
int32 result = small + large   # int8 promotes to int32
```

Downcasting requires explicit cast:

```merlin
int8 forced = (int8)large      # Explicit truncation
```

## Referenced Files

- `examples/primitives.mrl` — casting, conversion, numeric promotion
- `examples/strings.mrl` — string/int/char conversions and type alias casting
