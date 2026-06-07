# Comments

Merlin uses `#` for single-line comments. Everything from `#` to the end of the line is ignored.

```merlin
# This is a comment
int x = 5  # Inline comment
```

There are no multi-line comment delimiters (`/* */`). Use multiple `#` lines instead.

```merlin
# This is a
# multi-line comment
# in Merlin
```

Comments are also used in the standard library source files (`std/`) to document struct layouts and module purposes:
```merlin
# Merlin Standard Math Library
# Implementation: Links to C's libm.so
```
