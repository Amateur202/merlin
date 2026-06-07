# Error Handling

## Throws Functions

Functions marked with `throws` can return error codes via `raise`:

```merlin
int read_byte() throws:
    if fault_flag == 1:
        raise 1
    elif fault_flag == 2:
        raise 2
    return 0xAA
```

## Raise

`raise <int_value>` exits the function immediately with the given error code:

```merlin
void process() throws:
    if fault_flag != 0:
        raise -1
```

## Try / Catch

```merlin
try:
    int byte = read_byte()
    print("byte = ", byte, "\n")
catch 1:
    print("not found\n")
catch 2:
    print("timeout\n")
catch:
    print("unknown error\n")
```

- `catch <value>:` catches a specific error code
- `catch:` (catch-all) catches any remaining error

## Error Code Types

Error values must be of type `int`. In practice, positive and negative integers are used as error codes.

## Nested Try

Try blocks can be nested:

```merlin
try:
    try:
        process()
    catch -1:
        print("inner failed\n")
catch:
    print("outer catch\n")
```

## Checked Exceptions

The `throws` annotation is part of the function's type signature. Calling a `throws` function outside of a `try` block or another `throws` function will cause the program to abort on error.

## Referenced Files

- `examples/error_handling.mrl` — complete throws/raise/try/catch demo
- `examples/advanced.mrl` — template struct with throws method
- `std/list/list.mrl` — list methods with throws
