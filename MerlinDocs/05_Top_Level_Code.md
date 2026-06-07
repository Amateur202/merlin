# Top-Level Code

Merlin allows executable statements at the top level (outside of functions). These are automatically wrapped in a `main()` function during code generation.

```merlin
# examples/hello.mrl
print("Hello from Merlin!\n")
int answer = 42
print("answer = ", answer, "\n")
```

The compiler wraps top-level code in `int main() { ... return 0; }` automatically.

Variable declarations with non-constant initializers at the top level are split: the declaration goes to file scope and the assignment goes inside `main()`:

```merlin
# Top-level code statements go inside main()
# Top-level function/struct declarations stay at file scope
void my_func():
    print("inside function\n")

# This call goes inside main()
my_func()
```

You can mix both declarations (functions, structs, enums) and executable code at the top level. Declarations remain at file scope; executable statements go into `main()`.
