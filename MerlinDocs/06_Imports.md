# Imports

Merlin has a dual import system: `@import` makes all symbols globally available, while `import` requires a `module.` prefix.

## Global Import (`@import`)

All symbols (types, functions, variables, templates) from the imported module become directly accessible without any prefix:

```merlin
@import "math/math"

void demo():
    print("sqrt(9) = ", sqrt(9.0), "\n")  # Direct access, no prefix
```

Used by all example files that need standard library access.

## Namespaced Import (`import`)

All symbols require a `module.` prefix:

```merlin
import "list/list"

void demo():
    lists<int> items = make<int>()  # Error: global lookup fails
    # Would need prefix, but list symbols are template-based
```

The module name is derived from the file path (everything after the last `/` and without `.mrl`).

## Import Resolution

The compiler searches for imported files in this order:

1. Relative to the importing file's directory: `<file_dir>/<path>.mrl`
2. In the `std/` directory: `std/<path>.mrl`
3. In the `packages/` directory: `packages/<path>.mrl`
4. As a package directory: `<search_path>/` containing the module

## Private Symbols

Declarations marked `private` in an imported module are not visible to the importer:

```merlin
# In a library module
private int internal_counter = 0
private void helper():
    pass
```

## The `std/mem` Module

The memory module (`@import "mem"`) is imported by the list and hashmap standard libraries to provide access to C memory functions.

## Module Path Examples

| Import Path | Module Name | Search Location |
|---|---|---|
| `"math/math"` | `math` | `std/math/math.mrl` |
| `"list/list"` | `list` | `std/list/list.mrl` |
| `"strings/strings"` | `strings` | `std/strings/strings.mrl` |
| `"mem"` | `mem` | `std/mem.mrl` |

## Referenced Files

- `examples/advanced.mrl`, `examples/lib_list.mrl`, `examples/lib_math.mrl`, `examples/lib_str.mrl`, `examples/std_combined.mrl` — `@import` usage
- `examples/hashmap_test.mrl` — imports the hashmap module
- `std/list/list.mrl` — imports `mem`
- `std/hashmap/hashmap.mrl` — imports `mem`
