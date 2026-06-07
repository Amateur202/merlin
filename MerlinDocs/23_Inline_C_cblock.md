# Inline C with cblock

The `cblock:` keyword lets you embed raw C code inside Merlin functions. This is used for low-level operations that can't be expressed in Merlin directly.

## Syntax

```
cblock:
    <raw C code>
```

The indented block after `cblock:` is passed through verbatim to the C output.

## Examples

### Memory Allocation

```merlin
string res = ""
int n = 10
cblock:
    free(res.data);
    res.data = malloc(n + 1);
    res.data[n] = 0;
    res.capacity = n + 1;
res.length = n
```

### Hardware Access

```merlin
cblock:
    uint32_t* reg = (uint32_t*)0x40021000;
    *reg = 0x01;
```

### Hash Function

```merlin
int _hash(string s):
    cblock:
        uint32_t h = 2166136261u;
        int i;
        for (i = 0; i < s.length; i++) {
            h ^= (uint8_t)s.data[i];
            h *= 16777619u;
        }
        return (int)(h & 0x7fffffff);
```

### Inline Assembly

```merlin
asm:
    mov $0x01, %rax
    syscall
```

The `asm` keyword produces `__asm__ volatile(...)` in C.

## Important Notes

- `cblock` content is not parsed as Merlin — it's emitted verbatim
- You can reference Merlin variables by name (they become C variables)
- The lexer switches to raw mode for `cblock` bodies, capturing lines literally

## Referenced Files

- `std/strings/strings.mrl` — cblock used for string allocation and manipulation
- `std/hashmap/hashmap.mrl` — cblock used for hash function and resize operations
- `std/file/file.mrl` — cblock used for file I/O
