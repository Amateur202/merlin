# Loops

Merlin uses a unified `for` keyword for all looping constructs (no `while` keyword).

## For Range

Iterates over a numeric range:

```merlin
for i in range(0, 10):
    print(i, " ")
```

This compiles to `for (int i = 0; i < 10; i++)`.

## For Condition (Replaces While)

```merlin
int x = 5
for x > 0:
    print("tick\n")
    x = x - 1
```

Compiles to `while (x > 0)`.

## For Infinite (Replaces While True)

```merlin
for:
    print("loop\n")
    break
```

Compiles to `while (1)`.

## Break and Continue

```merlin
for i in range(0, 5):
    if i == 2:
        continue
    if i == 4:
        break
    print(i, "\n")
```

## Referenced Files

- `examples/control_flow.mrl` — all loop types with break/continue
