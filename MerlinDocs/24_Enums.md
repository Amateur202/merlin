# Enums

## Declaration

```
enum <Name>:
    <value_name>
    <value_name> = <expression>
```

```merlin
enum Color:
    RED
    GREEN
    BLUE
    CUSTOM = 100
```

## Values Without Initializer

Enumerators without `= <value>` start at 0 and increment by 1:

```merlin
enum Status:
    OK          # 0
    ERROR       # 1
    TIMEOUT     # 2
```

## Values With Initializer

```merlin
enum ErrorCode:
    NOT_FOUND = 1
    TIMEOUT   = 2
    UNKNOWN   = 255
```

## Using Enums

Enum values are accessible as top-level constants:

```merlin
Status s = OK
if s == ERROR:
    print("error!\n")
```

## Private Enums

```merlin
private enum Internal:
    STATE_A
    STATE_B
```

Private enum values are not exported when the module is imported.

## Individual Private Values

Individual enum values can be marked private:

```merlin
enum Result:
    SUCCESS
    private INTERNAL_ERROR
    FAILURE
```

## Referenced Files

- See `src/Parser.go` lines 306-337 for enum parsing logic
- Enums are used in the standard library types and error codes
