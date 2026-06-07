# Strings

## String Type

Merlin has a built-in `string` type compiled to a `MerlinString` struct in C:

```c
typedef struct {
    char* data;
    int length;
    int capacity;
} MerlinString;
```

## String Literals

```merlin
string name      = "Merlin"
string path      = "/usr/local/merlin"
string short_str = "ok"
```

## String Operations

```merlin
# Concatenation
string greeting = "Hello, " + "World!"

# Length
int n = len(s)

# Indexing (bounds-checked at runtime)
char c = (char)s.data[0]

# Comparison
if a == b:
    print("equal\n")
```

## Conversion Functions

```merlin
# String to int
string buf = "8192"
int port   = int(buf)

# Int to string
int sid   = 65
string s1 = string(sid)

# Char to string
string s2 = string('M')
```

## String Slicing

```merlin
string sub = s[2:9]   # Substring from index 2 to 9 (exclusive)
string sub = s[:5]    # From start to index 5
string sub = s[3:]    # From index 3 to end
```

## Empty Strings

```merlin
string empty = ""
```

## Data Access

The underlying C data is accessible via `.data`:

```merlin
for i in range(0, len(s)):
    char c = (char)s.data[i]
```

Standard library functions in `std/strings/strings.mrl` use this for string manipulation.

## Referenced Files

- `examples/strings.mrl` — string conversions and casts
- `std/strings/strings.mrl` — full string utility library
