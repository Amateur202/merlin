# Standard Library: Memory

**File:** `std/mem/mem.mrl`

Raw memory allocation functions, wrapping C's `<stdlib.h>`. This module is automatically imported by the list and hashmap standard libraries.

## External Functions

| Function | Description |
|---|---|
| `&void malloc(uint64 size)` | Allocate uninitialized memory |
| `&void calloc(uint64 count, uint64 size)` | Allocate zero-initialized memory |
| `&void realloc(&void ptr, uint64 size)` | Resize allocation |
| `void free(&void ptr)` | Free allocation |

## Typed Wrappers

```merlin
template<T>
&T alloc(int n):
    return (&T)malloc(n * sizeof(T))

template<T>
&T alloc_zero(int n):
    return (&T)calloc(n, sizeof(T))
```

## Usage

```merlin
@import "mem"

# Allocate 10 ints
&int arr = alloc<int>(10)

# Allocate 5 zero-initialized floats
&float arr2 = alloc_zero<float>(5)

free(arr)
free(arr2)
```

## Referenced Files

- `std/mem/mem.mrl` — module source
- `std/list/list.mrl` — imports `mem` for dynamic allocation
- `std/hashmap/hashmap.mrl` — imports `mem` for hash map allocation
