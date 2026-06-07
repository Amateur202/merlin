# Introduction to Merlin

Merlin is a compiled systems programming language that transpiles to C and then compiles with GCC. It combines Python-style indentation-based syntax with C-level performance and control.

## Philosophy

- **Readable syntax**: Indentation-based blocks, English keywords (`and`, `or`, `not` instead of `&&`, `||`, `!`)
- **C performance**: Compiles through C, so you get the same machine code GCC produces
- **Modern features**: Generics (templates), pattern matching, error handling with `throws`/`raise`/`try`/`catch`
- **Low-level access**: Pointers, `cblock` for inline C, `volatile` for hardware access, `asm` for inline assembly
- **String support**: Built-in dynamic string type (`string`) with `data`, `length`, and `capacity` fields

## Key Characteristics

- **Indentation-based**: Uses Python-style significant whitespace (no braces)
- **Statically typed**: All types must be known at compile time
- **Type inference**: `:=` short variable declarations infer types
- **No garbage collection**: Manual memory management via `malloc`/`free`
- **Dual import system**: `@import` for global symbols, `import` for namespaced access

## Getting Started

```merlin
# examples/hello.mrl
print("Hello from Merlin!\n")

int    answer = 42
float  pi     = 3.14159
bool   ok     = true
char   nl     = '\n'

print("answer = ", answer, "\n")
print("pi     = ", pi, "\n")
print("bool   = ", ok, "\n")
print("char   = ", 'A', nl)
```

Compile and run:

```
merlin examples/hello.mrl
./hello
```
