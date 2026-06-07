# Standard Library: List

**File:** `std/list/list.mrl`

A dynamic array (vector) template. Import with `@import "list/list"`.

## Type

```merlin
template<T>
struct lists:
    &T data
    int count
    int capacity
```

## Constructor

```merlin
template<T>
lists<T> make():
    return lists<T>{data: (&T)0, count: 0, capacity: 0}
```

## Methods

| Method | Description |
|---|---|
| `void push(T val)` | Append element (auto-grows) |
| `T pop()` | Remove and return last element |
| `T get(int idx) throws` | Get element at index (bounds-checked, raises 1) |
| `T uget(int idx)` | Unchecked get (no bounds check) |
| `void set(int idx, T val)` | Set element at index |
| `int len()` | Return number of elements |
| `bool contains(T val)` | Check if value exists |
| `void remove(int idx) throws` | Remove element (bounds-checked, raises 1) |
| `void uremove(int idx)` | Unchecked remove |
| `void insert(int idx, T val) throws` | Insert element (bounds-checked, raises 1) |
| `void uinsert(int idx, T val)` | Unchecked insert |
| `void sort()` | Bubble sort (in-place) |
| `void destroy()` | Free memory |

## Example

```merlin
@import "list/list"

lists<int> nums = make<int>()
nums.push(10)
nums.push(20)
nums.push(30)

try:
    print("nums[1] = ", nums.get(1), "\n")
catch 1:
    print("out of bounds\n")

nums.destroy()
```

## Referenced Files

- `examples/lib_list.mrl` — full demonstration of list operations
- `examples/advanced.mrl` — list with throws handling
- `examples/std_combined.mrl` — list used with other std modules
- `std/list/list.mrl` — module source
