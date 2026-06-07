# Methods

Methods are functions defined inside a struct body. They receive a `self` parameter implicitly.

## Declaration

```merlin
struct NetworkInterface:
    uint16 io_port
    uint32 capacity
    bool   active

    void configure(uint16 port):
        self.io_port  = port
        self.active   = true
        self.capacity = 1518

    bool is_active():
        return self.active

    uint16 get_port():
        return self.io_port
```

## Calling Methods

```merlin
NetworkInterface eth0 = NetworkInterface{io_port: 0x300, capacity: 0, active: false}
eth0.configure(0x300)
bool up = eth0.is_active()
uint16 port = eth0.get_port()
```

## Self

`self` refers to the struct instance the method was called on. It accesses fields without any pointer dereference syntax — Merlin handles auto-dereferencing.

## Template Methods

```merlin
template<T>
struct Box:
    T value

    void set(T v):
        self.value = v

    T get():
        return self.value
```

## Referenced Files

- `examples/structs_methods.mrl` — methods with self
- `examples/templates.mrl` — template struct with methods
