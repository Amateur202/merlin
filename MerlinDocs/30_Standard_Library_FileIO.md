# Standard Library: File I/O

**File:** `std/file/file.mrl`

Simple file reading functions. Import with `@import "file/file"`.

## Types

```merlin
struct StringList:
    &string data
    int count
    int capacity
```

## Functions

| Function | Description |
|---|---|
| `string read(string path)` | Read entire file as a string |
| `StringList readl(string path)` | Read file into lines |
| `int line_len(&StringList lines, int idx)` | Get length of a specific line |
| `string line_at(&StringList lines, int idx)` | Copy a specific line |
| `void lines_free(&StringList lines)` | Free the StringList |

## Example

```merlin
@import "file/file"

string content = read("data.txt")
print("file length: ", content.length, "\n")
print(content)

StringList lines = readl("data.txt")
print("line 1: ", line_at(&lines, 0), "\n")
lines_free(&lines)
```

## Referenced Files

- `std/file/file.mrl` — module source
