# `jp`: JSON Pointers for Raw JSON Content

<a href="https://godoc.org/github.com/pgavlin/jp"><img src="https://img.shields.io/badge/api-reference-blue.svg?style=flat-square" alt="GoDoc"></a>

`jp` is a Go package that provides access to raw JSON content using [RFC 6901 JSON pointers](https://datatracker.ietf.org/doc/html/rfc6901).

`jp` is derived from [GJSON](https://github.com/tidwall/gjson)


## Installing

To start using `jp`, install Go and run `go get`:

```sh
$ go get -u github.com/pgavlin/jp
```

This will retrieve the library.

## Get a value
Get searches JSON content for the specified pointer. A pointer is in JSON pointer syntax, such as "/name/last" or "/age". When the value is found it's returned immediately. 

```go
package main

import "github.com/pgavlin/jp"

const json = `{"name":{"first":"Janet","last":"Prichard"},"age":47}`

func main() {
	value := jp.Get(json, "/name/last")
	println(value.String())
}
```

This will print:

```
Prichard
```
*There's also the [GetMany](#get-multiple-values-at-once) function to get multiple values at once, and [GetBytes](#working-with-bytes) for working with JSON byte slices.*

## Pointer Syntax

Below is a quick overview of the pointer syntax, for more complete information please see [RFC 6901](https://datatracker.ietf.org/doc/html/rfc6901).

- A pointer is a series of keys separated by `/`
- An array elements is accessed using its base-10 index as its key
- If they appear in aa key, the `~` and `/` characters must be escaped as `~0` and `~1`, respectively

```json
{
  "name": {"first": "Tom", "last": "Anderson"},
  "age":37,
  "children": ["Sara","Alex","Jack"],
  "fav.movie": "Deer Hunter",
  "friends": [
    {"first": "Dale", "last": "Murphy", "age": 44, "nets": ["ig", "fb", "tw"]},
    {"first": "Roger", "last": "Craig", "age": 68, "nets": ["fb", "tw"]},
    {"first": "Jane", "last": "Murphy", "age": 47, "nets": ["ig", "tw"]}
  ]
}
```
```
"/name/last"          >> "Anderson"
"/age"                >> 37
"/children"           >> ["Sara","Alex","Jack"]
"/children/1"         >> "Alex"
"/fav.movie"         >> "Deer Hunter"
"/friends/1/last"     >> "Craig"
```


## Result Type

`jp` supports the json types `string`, `number`, `bool`, and `null`. 
Arrays and Objects are returned as their raw json types. 

The `Result` type holds one of these:

```
bool, for JSON booleans
float64, for JSON numbers
string, for JSON string literals
nil, for JSON null
```

To directly access the value:

```go
result.Type           // can be String, Number, True, False, Null, or JSON
result.Str            // holds the string
result.Num            // holds the float64 number
result.Raw            // holds the raw json
result.Index          // index of raw value in original json, zero means index unknown
```

There are a variety of handy functions that work on a result:

```go
result.Exists() bool
result.Value() interface{}
result.Int() int64
result.Uint() uint64
result.Float() float64
result.String() string
result.Bool() bool
result.Time() time.Time
result.Array() []jp.Result
result.Map() map[string]jp.Result
result.Get(pointer string) jp.Result
result.Range() *jp.Iterator
result.ForEach(iterator func(key, value jp.Result) bool)
result.Less(token jp.Result, caseSensitive bool) bool
```

The `result.Value()` function returns an `interface{}` which requires type assertion and is one of the following Go types:

```go
boolean >> bool
number  >> float64
string  >> string
null    >> nil
array   >> []interface{}
object  >> map[string]interface{}
```

The `result.Array()` function returns back an array of values.
If the result represents a non-existent value, then an empty array will be returned.
If the result is not a JSON array, the return value will be an array containing one result.

### 64-bit integers

The `result.Int()` and `result.Uint()` calls are capable of reading all 64 bits, allowing for large JSON integers.

```go
result.Int() int64    // -9223372036854775808 to 9223372036854775807
result.Uint() int64   // 0 to 18446744073709551615
```

## Iterate through an object or array

The `ForEach` function allows for quickly iterating through an object or array. 
The key and value are passed to the iterator function for objects.
The element index and value aare passed for arrays.
Returning `false` from an iterator will stop iteration.

```go
result := jp.Get(json, "/programmers")
result.ForEach(func(key, value jp.Result) bool {
	println(value.String()) 
	return true // keep iterating
})
```

Alternatively, the `Range` function allows the caller to drive iteration:

```go
result := jp.Get(json, "/programmers")
for it := result.Range(); it.Next(); {
	println(value.String())
}
```

## Simple Parse and Get

There's a `Parse(json)` function that will do a simple parse, and `result.Get(pointer)` that will search a result.

For example, all of these will return the same result:

```go
jp.Parse(json).Get("/name").Get("/last")
jp.Get(json, "/name").Get("/last")
jp.Get(json, "/name/last")
```

## Check for the existence of a value

Sometimes you just want to know if a value exists. 

```go
value := jp.Get(json, "/name/last")
if !value.Exists() {
	println("no last name")
} else {
	println(value.String())
}

// Or as one step
if jp.Get(json, "/name/last").Exists() {
	println("has a last name")
}
```

## Validate JSON

The `Get*` and `Parse*` functions expects that the json is well-formed. Bad json will not panic, but it may return back unexpected results.

If you are consuming JSON from an unpredictable source then you may want to validate prior to using GJSON.

```go
if !jp.Valid(json) {
	return errors.New("invalid json")
}
value := jp.Get(json, "name.last")
```

## Unmarshal to a map

To unmarshal to a `map[string]interface{}`:

```go
m, ok := jp.Parse(json).Value().(map[string]interface{})
if !ok {
	// not a map
}
```

## Working with Bytes

If your JSON is contained in a `[]byte` slice, there's the [GetBytes](https://godoc.org/github.com/tidwall/jp#GetBytes) function. This is preferred over `Get(string(data), pointer)`.

```go
var json []byte = ...
result := jp.GetBytes(json, pointer)
```

If you are using the `jp.GetBytes(json, pointer)` function and you want to avoid converting `result.Raw` to a `[]byte`, then you can use this pattern:

```go
var json []byte = ...
result := jp.GetBytes(json, pointer)
var raw []byte
if result.Index > 0 {
    raw = json[result.Index:result.Index+len(result.Raw)]
} else {
    raw = []byte(result.Raw)
}
```

This is a best-effort no allocation sub slice of the original json. This method utilizes the `result.Index` field, which is the position of the raw data in the original json. It's possible that the value of `result.Index` equals zero, in which case the `result.Raw` is converted to a `[]byte`.

## Get multiple values at once

The `GetMany` function can be used to get multiple values at the same time.

```go
results := jp.GetMany(json, "/name/first", "/name/last", "/age")
```

The return value is a `[]Result`, which will always contain exactly the same number of items as the input pointers.
