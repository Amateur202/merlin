# Compiler Flags

```
Merlin v1.0.0
Usage: merlin <source.mrl> [flags]
```

## `--ast-only`
Parse the source file and print the AST as JSON. Skips code generation and GCC compilation.

```
merlin examples/hello.mrl --ast-only
```

Useful for debugging the parser output.

## `--emit-c`
Generate the C file (e.g., `hello.c`) but skip GCC compilation. Inspect the C output to see how Merlin translates your code.

```
merlin examples/hello.mrl --emit-c
cat hello.c
```

## `--output <bin>`
Specify the output binary name. Defaults to the source file's basename without extension.

```
merlin examples/hello.mrl --output my_program
./my_program
```

## `--verbose`
Print pipeline stage progress to stderr:

```
[merlin] reading source file: examples/hello.mrl
[merlin] lexing...
[merlin] lexed 27 tokens
[merlin] parsing...
[merlin] parsed 3 top-level nodes (0 imports)
[merlin] semantic analysis...
[merlin] generating C code...
[merlin] compiling with GCC...
compiled: hello
```

## `--version`
Print the compiler version and exit.

```
merlin --version
```

## `--link <lib>`
Link against a system library (e.g., `--link m` for `libm.so`). Used when importing the math module or any standard library that requires C linkage.

```
merlin examples/lib_math.mrl --link m
```

You can also use `external` function declarations with `link` in your source code (see the math module for examples).

## `--opt <level>`
Optimization level passed to GCC. One of: `O`, `O1`, `O2`, `O3`. Default is `O`.

```
merlin examples/hello.mrl --opt O3
```

The GCC flags used are: `-<level> -fno-strict-aliasing -march=native`.
