# Variables

## Explicit Variable Declaration

```
<type> <name> = <value>
<type> <name>
```

```merlin
int    answer = 42
float  pi     = 3.14159
bool   ok     = true
char   nl     = '\n'
string msg    = "Hello"
```

Variables can be declared without an initializer. They will have a default zero value.

## Short Variable Declaration (`:=`)

Type is inferred from the expression:

```merlin
x := 42        # int
y := 3.14      # float
z := true      # bool
s := "hello"   # string
```

## Multiple Return Values

```merlin
int, string lookup(int id):
    return id, "found"

# Assigning multiple return values
val, status := lookup(5)
```

## Multi-Variable Declaration

```merlin
a, b := 1, 2
x, y = 10, 20  # Assignment (already declared)
```

## Compound Assignment

```merlin
int acc = 10
acc += 2
acc -= 1
acc *= 5
acc /= 2
acc %= 3
```

## Const Variables

```merlin
const int MAX = 100   # Immutable after declaration
```

## Arrays

```merlin
int[5] arr           # Fixed-size array of 5 ints
int[3] first = ...   # Array from slice operation
```

Arrays are value types (copied on assignment).

## Private Variables

Top-level variables can be marked `private`:

```merlin
private int internal = 0
```

Private variables are not exported when their module is imported.

## Volatile Variables

Used for memory-mapped I/O:

```merlin
volatile &uint8 uart = (volatile &uint8)0x3F8
```

## Referenced Files

- `examples/hello.mrl` — basic variable usage
- `examples/primitives.mrl` — variable declarations with all types
- `examples/pointers.mrl` — pointer variables
