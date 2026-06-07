# Standard Library: Strings

**File:** `std/strings/strings.mrl`

String utility functions. Import with `@import "strings/strings"`.

## Functions

| Function | Description |
|---|---|
| `bool is_empty(string s)` | Check if string length is 0 |
| `int index_of(string s, string sub)` | Find first index of substring (-1 if not found) |
| `bool contains(string s, string sub)` | Check if substring exists |
| `bool starts_with(string s, string sub)` | Check prefix |
| `bool ends_with(string s, string sub)` | Check suffix |
| `string upper(string s)` | Convert to uppercase |
| `string lower(string s)` | Convert to lowercase |
| `string substring(string s, int start, int end)` | Extract substring |
| `string reverse(string s)` | Reverse the string |
| `string repeat(string s, int n)` | Repeat string n times |
| `int compare(string a, string b)` | Lexicographic comparison (-1, 0, 1) |
| `string capitalize(string s)` | First character to uppercase |
| `string uncapitalize(string s)` | First character to lowercase |
| `string trim_left(string s)` | Remove leading whitespace |
| `string trim_right(string s)` | Remove trailing whitespace |
| `string trim(string s)` | Remove both leading and trailing whitespace |
| `string replace(string s, string old, string new)` | Replace all occurrences |

## Example

```merlin
@import "strings/strings"

string msg = "  Hello, Merlin!  "
print("upper: [", upper(msg), "]\n")
print("trim:  [", trim(msg), "]\n")
print("contains 'rlin': ", contains(msg, "rlin"), "\n")
```

## Referenced Files

- `examples/lib_str.mrl` — full demonstration of all string functions
- `examples/std_combined.mrl` — strings used with other std modules
- `std/strings/strings.mrl` — module source
