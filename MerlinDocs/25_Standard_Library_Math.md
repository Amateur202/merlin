# Standard Library: Math

**File:** `std/math/math.mrl`

The math module wraps C's `libm` library. Import with `@import "math/math"` and compile with `--link m`.

## Constants

| Name | Value |
|---|---|
| `PI` | 3.14159265358979323846 |
| `E` | 2.71828182845904523536 |

## Basic Arithmetic

| Function | Description |
|---|---|
| `float fabs(float x)` | Absolute value |
| `float sqrt(float x)` | Square root |
| `float pow(float x, float y)` | Power (x^y) |
| `float exp(float x)` | Exponential (e^x) |
| `float log(float x)` | Natural logarithm |
| `float log10(float x)` | Base-10 logarithm |

## Trigonometry

| Function | Description |
|---|---|
| `float sin(float x)` | Sine |
| `float cos(float x)` | Cosine |
| `float tan(float x)` | Tangent |
| `float asin(float x)` | Arc sine |
| `float acos(float x)` | Arc cosine |
| `float atan(float x)` | Arc tangent |
| `float atan2(float y, float x)` | Arc tangent of y/x |

## Hyperbolic

| Function | Description |
|---|---|
| `float sinh(float x)` | Hyperbolic sine |
| `float cosh(float x)` | Hyperbolic cosine |
| `float tanh(float x)` | Hyperbolic tangent |

## Rounding

| Function | Description |
|---|---|
| `float ceil(float x)` | Round up |
| `float floor(float x)` | Round down |
| `float round(float x)` | Round to nearest |

## Example

```merlin
@import "math/math"

print("PI = ", PI, "\n")
print("sqrt(9) = ", sqrt(9.0), "\n")
print("sin(PI/2) = ", sin(PI / 2.0), "\n")
```

Compile with: `merlin myfile.mrl --link m`

## Referenced Files

- `examples/lib_math.mrl` — full demonstration of all math functions
- `examples/std_combined.mrl` — math used with other std modules
- `std/math/math.mrl` — module source
