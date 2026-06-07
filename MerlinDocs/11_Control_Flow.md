# Control Flow

## if / elif / else

```merlin
if val < 0:
    print("negative\n")
elif val == 0:
    print("zero\n")
else:
    print("positive\n")
```

Conditions can use `and`, `or`, `not`:

```merlin
if val >= 10 and val != 100:
    # ...
```

## pass (No-Op)

A placeholder that does nothing:

```merlin
if val == 999:
    pass
```

## Switch / If-Else Chains

Merlin does not have a C-style `switch` statement. Use `if`/`elif`/`else` chains or `match`/`case` for pattern matching.

## Referenced Files

- `examples/control_flow.mrl` — full demonstration of if/elif/else, pass
- `examples/primitives.mrl` — conditions with `and`, `or`, `not`
