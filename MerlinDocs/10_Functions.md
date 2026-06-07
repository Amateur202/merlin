# Functions

## Declaration

```
<return_type> <name>(<params>):
    <body>
```

```merlin
void say_hello():
    print("Hello!\n")

int add(int a, int b):
    return a + b
```

## Return Types

- `void` — no return value
- Single type — returns one value
- Multiple types — comma-separated return types

```merlin
void log(string msg):
    print(msg, "\n")

int square(int x):
    return x * x

int, string find(int id):
    if id == 0:
        return 0, "not found"
    return id, "ok"
```

Multi-return functions compile to out-parameters in C.

## Parameters

Parameters are typed:

```merlin
void configure(uint16 port, bool active, string label):
    # ...
```

Parameters can be pointers:

```merlin
void swap(&int a, &int b):
    int tmp = *a
    *a = *b
    *b = tmp
```

## Pointers in Return Types

```merlin
&int get_ptr(&int p):
    return p
```

## Private Functions

```merlin
private void helper():
    pass
```

Private functions are not visible to importing modules.

## Short Function Syntax

Functions can be called as part of expressions or at the top level:

```merlin
void demo():
    int result = add(3, 4)
    print("result = ", result, "\n")

demo()  # Top-level call
```

## Referenced Files

- `examples/hello.mrl` — top-level code calling `print`
- `examples/primitives.mrl` — function with all primitive types
- `examples/templates.mrl` — template functions
- `examples/error_handling.mrl` — functions with `throws`
