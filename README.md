# Merlin

A compiled systems programming language with Python-style syntax and C-level performance.

Merlin transpiles to C and compiles with GCC, combining readable indentation-based syntax with full low-level control.

## Features

- **Python-style syntax**: indentation-based blocks, English keywords (`and`, `or`, `not`)
- **Statically typed**: full type system with integers, floats, booleans, chars, strings
- **Generics**: template structs and functions
- **Error handling**: `throws`/`raise`/`try`/`catch`
- **Pattern matching**: `match`/`case`
- **Low-level access**: pointers, inline C (`cblock`), inline assembly (`asm`), `volatile`
- **Standard library**: math, strings, list (dynamic array), hashmap, file I/O, memory
- **Dual import system**: `@import` for global symbols, `import` for namespaced access

## Quick Start

```
make
./src/merlin examples/hello.mrl
./hello
```

## Building

Requires Go 1.26+ and GCC.

```
make build          # builds the compiler
make clean          # cleans build artifacts
```

## Examples

The `examples/` directory contains ready-to-run `.mrl` files:

| Example | Description |
|---|---|
| `hello.mrl` | Minimal demo with all print types |
| `primitives.mrl` | All types, operators, casting |
| `control_flow.mrl` | if/elif/else, for loops, break/continue |
| `structs_methods.mrl` | Structs, methods, self |
| `templates.mrl` | Generic structs and functions |
| `pointers.mrl` | Pointers, dereference, auto-deref, volatile |
| `strings.mrl` | String conversions and type aliases |
| `error_handling.mrl` | throws/raise/try/catch |
| `match.mrl` | Pattern matching |
| `advanced.mrl` | Combined advanced features |
| `lib_math.mrl` | Math library demo (`--link m`) |
| `lib_str.mrl` | String utilities demo |
| `lib_list.mrl` | Dynamic array (list) demo |
| `hashmap_test.mrl` | Hash map demo |
| `std_combined.mrl` | Combined std library demo |

## Documentation

See `MerlinDocs/` for detailed documentation on each feature.

## License

MIT — see [LICENSE](LICENSE).
