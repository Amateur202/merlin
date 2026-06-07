# Match / Case

Pattern matching with `match` / `case`.

## Syntax

```
match <expression>:
    case <value>:
        <body>
    case:
        <catch-all body>
```

## Example

```merlin
void handle_code(uint8 code):
    match code:
        case 0x00:
            print("divide fault\n")
        case 0x0A:
            print("stack overflow\n")
        case 0x0F:
            print("timeout\n")
        case:
            print("generic error\n")
```

## Catch-All Case

The bare `case:` (no value) acts as a catch-all/default case.

## Ordered Matching

Cases are evaluated in order. The first matching case executes, then execution continues after the match block (no implicit fallthrough).

## Referenced Files

- `examples/match.mrl` — full match/case demonstration
