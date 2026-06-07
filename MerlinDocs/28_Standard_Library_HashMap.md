# Standard Library: HashMap

**File:** `std/hashmap/hashmap.mrl`

A hash map from `string` keys to `int` values. Import with `@import "hashmap/hashmap"`.

## Type

```merlin
struct HashMap:
    &uint8 data
    int count
    int capacity
```

HashMap uses open addressing with FNV-1a hashing. Slots are 32 bytes each.

## Functions (Not Methods)

HashMap functions take `&HashMap self` as the first parameter (pointer to map).

| Function | Description |
|---|---|
| `HashMap new()` | Create a new empty map |
| `void put(&HashMap self, string key, int val)` | Insert/update (copies key) |
| `void put_move(&HashMap self, string key, int val)` | Insert/update (zero-copy, takes ownership of key) |
| `bool contains(&HashMap self, string key)` | Check if key exists |
| `bool try_get(&HashMap self, string key, &int out_val)` | Get with existence check (true if found) |
| `int get(&HashMap self, string key)` | Get value (returns 0 if missing; use try_get to distinguish) |
| `bool delete(&HashMap self, string key)` | Delete key (returns true if existed) |
| `int size(&HashMap self)` | Number of entries |
| `void clear(&HashMap self)` | Remove all entries |
| `void destroy(&HashMap self)` | Free all memory |

## Example

```merlin
@import "hashmap/hashmap"

HashMap map = new()
put(&map, "alice", 30)
put(&map, "bob", 25)

print("alice = ", get(&map, "alice"), "\n")
print("contains zoe: ", contains(&map, "zoe"), "\n")

destroy(&map)
```

## Referenced Files

- `examples/hashmap_test.mrl` — full demonstration
- `std/hashmap/hashmap.mrl` — module source
