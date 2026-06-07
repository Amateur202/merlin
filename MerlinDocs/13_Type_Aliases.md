# Type Aliases

Create new named types from existing types:

```
type <Name> <BaseType>
```

```merlin
type Meter int
type Feet  int
type ResultCode int
```

Type aliases create distinct types. Casting is required between them:

```merlin
Meter distance = (Meter)1500
Feet  ceiling  = (Feet)distance
int   raw      = (int)distance
```

Type aliases must be declared at the top level. They can be marked `private` to prevent export.

## Referenced Files

- `examples/strings.mrl` — `Meter` and `Feet` type aliases
- `examples/advanced.mrl` — `ResultCode` type alias
