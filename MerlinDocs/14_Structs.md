# Structs

## Declaration

```
struct <Name>:
    <field_type> <field_name>
    ...
```

```merlin
struct NetworkInterface:
    uint16 io_port
    uint32 capacity
    bool   active
```

## Instantiation

### Full Initialization

```merlin
NetworkInterface eth0 = NetworkInterface{io_port: 0x300, capacity: 0, active: false}
```

### Partial Initialization

Unspecified fields default to zero:

```merlin
NetworkInterface eth1 = NetworkInterface{io_port: 0x300}
```

## Field Access

```merlin
eth0.io_port = 0x300
bool up = eth0.active
```

## Structs as Types

Struct names can be used in type annotations:

```merlin
NetworkInterface eth0
```

## Private Fields

Fields can be marked `private`:

```merlin
struct Account:
    private int balance
    public string name   # No prefix means public
```

Private fields are not accessible from importing modules.

## Referenced Files

- `examples/structs_methods.mrl` — full struct with fields and methods
- `examples/pointers.mrl` — struct with pointer
- `examples/templates.mrl` — template struct
