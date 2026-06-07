# Building and Running Merlin Programs

## Building the Compiler

Merlin is written in Go. To build it:

```
cd src/
go build -o merlin
```

This produces a single `merlin` binary. You can move it anywhere in your `PATH`.

## Compiling a Merlin Program

```
merlin <source.mrl> [flags]
```

The source file must have a `.mrl` extension. By default, Merlin:

1. Reads and lexes the source file
2. Parses it into an AST
3. Runs semantic (type) analysis
4. Generates C code (e.g., `hello.mrl` → `hello.c`)
5. Compiles the C code with GCC into a binary (e.g., `hello`)

## Running the Output

The compiled binary is executable directly:

```
./hello
./my_program
```

## Examples

The `examples/` directory contains ready-to-run `.mrl` files:

```
merlin examples/hello.mrl
./hello
```

For the math examples, link against `libm`:

```
merlin examples/lib_math.mrl --link m
./lib_math
```

## The Pipeline

1. **Lexer** (`Lexer.go`) — tokenizes source with Python-style indentation
2. **Parser** (`Parser.go`, `Expr.go`) — builds AST using Pratt parsing
3. **Checker** (`Checker.go`) — semantic analysis + type checking
4. **Codegen** (`Codegen.go`, `Cg_decl.go`, `Cg_expr.go`, `Cg_util.go`) — emits C
5. **GCC** — compiles the C to a binary using `-O -fno-strict-aliasing -march=native`
