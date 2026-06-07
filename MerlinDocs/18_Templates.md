# Templates (Generics)

## Template Declaration

```
template<T>
struct <Name>:
    ...
```

```
template<T>
<return_type> <name>(<params>):
    ...
```

## Template Struct

```merlin
template<T>
struct Box:
    T value

    void set(T v):
        self.value = v

    T get():
        return self.value
```

## Instantiation

```merlin
Box<int>  ibox = Box<int>{value: 42}
Box<float> fbox = Box<float>{value: 3.14}

int   iv = ibox.get()
float fv = fbox.get()
```

## Template Function

```merlin
template<T>
T identity(T x):
    return x
```

```merlin
int   same_i = identity<int>(99)
float same_f = identity<float>(2.71)
```

## Multiple Type Parameters

```merlin
template<K, V>
struct Pair:
    K key
    V value
```

## Template Method Calls

Template methods can be called with explicit type arguments:

```merlin
obj.method<Type>(args)
```

## Template Instantiation in Imports

Template types from imported modules are accessible:

```merlin
@import "list/list"

lists<int> items = make<int>()
```

## Concrete Declarations

Template instantiations generate concrete declarations (`ConcreteDecls`) that the codegen phase emits as regular C structs and functions with mangled names (e.g., `Box_int`).

## Referenced Files

- `examples/templates.mrl` — template struct, function, methods
- `examples/advanced.mrl` — template with throws
- `std/list/list.mrl` — template struct with `T` parameter
