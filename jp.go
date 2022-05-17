// Package gjson provides searching for json strings.
package gjson

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

// Type is Result type
type Type int

const (
	// Null is a null json value
	Null Type = iota
	// False is a json false boolean
	False
	// Number is json number
	Number
	// String is a json string
	String
	// True is a json true boolean
	True
	// JSON is a raw block of JSON
	JSON
)

// String returns a string representation of the type.
func (t Type) String() string {
	switch t {
	default:
		return ""
	case Null:
		return "Null"
	case False:
		return "False"
	case Number:
		return "Number"
	case String:
		return "String"
	case True:
		return "True"
	case JSON:
		return "JSON"
	}
}

// Result represents a json value that is returned from Get().
type Result struct {
	// Type is the json type
	Type Type
	// Raw is the raw json
	Raw string
	// Str is the json string
	Str string
	// Num is the json number
	Num float64
	// Index of raw value in original json, zero means index unknown
	Index int
}

// String returns a string representation of the value.
func (t Result) String() string {
	switch t.Type {
	default:
		return ""
	case False:
		return "false"
	case Number:
		if len(t.Raw) == 0 {
			// calculated result
			return strconv.FormatFloat(t.Num, 'f', -1, 64)
		}
		var i int
		if t.Raw[0] == '-' {
			i++
		}
		for ; i < len(t.Raw); i++ {
			if t.Raw[i] < '0' || t.Raw[i] > '9' {
				return strconv.FormatFloat(t.Num, 'f', -1, 64)
			}
		}
		return t.Raw
	case String:
		return t.Str
	case JSON:
		return t.Raw
	case True:
		return "true"
	}
}

// Bool returns an boolean representation.
func (t Result) Bool() bool {
	switch t.Type {
	default:
		return false
	case True:
		return true
	case String:
		b, _ := strconv.ParseBool(strings.ToLower(t.Str))
		return b
	case Number:
		return t.Num != 0
	}
}

// Int returns an integer representation.
func (t Result) Int() int64 {
	switch t.Type {
	default:
		return 0
	case True:
		return 1
	case String:
		n, _ := parseInt(t.Str)
		return n
	case Number:
		// try to directly convert the float64 to int64
		i, ok := safeInt(t.Num)
		if ok {
			return i
		}
		// now try to parse the raw string
		i, ok = parseInt(t.Raw)
		if ok {
			return i
		}
		// fallback to a standard conversion
		return int64(t.Num)
	}
}

// Uint returns an unsigned integer representation.
func (t Result) Uint() uint64 {
	switch t.Type {
	default:
		return 0
	case True:
		return 1
	case String:
		n, _ := parseUint(t.Str)
		return n
	case Number:
		// try to directly convert the float64 to uint64
		i, ok := safeInt(t.Num)
		if ok && i >= 0 {
			return uint64(i)
		}
		// now try to parse the raw string
		u, ok := parseUint(t.Raw)
		if ok {
			return u
		}
		// fallback to a standard conversion
		return uint64(t.Num)
	}
}

// Float returns an float64 representation.
func (t Result) Float() float64 {
	switch t.Type {
	default:
		return 0
	case True:
		return 1
	case String:
		n, _ := strconv.ParseFloat(t.Str, 64)
		return n
	case Number:
		return t.Num
	}
}

// Time returns a time.Time representation.
func (t Result) Time() time.Time {
	res, _ := time.Parse(time.RFC3339, t.String())
	return res
}

// Array returns back an array of values.
// If the result represents a null value or is non-existent, then an empty
// array will be returned.
// If the result is not a JSON array, the return value will be an
// array containing one result.
func (t Result) Array() []Result {
	if t.Type == Null {
		return []Result{}
	}
	if !t.IsArray() {
		return []Result{t}
	}
	r := t.arrayOrMap('[', false)
	return r.a
}

// IsObject returns true if the result value is a JSON object.
func (t Result) IsObject() bool {
	return t.Type == JSON && len(t.Raw) > 0 && t.Raw[0] == '{'
}

// IsArray returns true if the result value is a JSON array.
func (t Result) IsArray() bool {
	return t.Type == JSON && len(t.Raw) > 0 && t.Raw[0] == '['
}

// IsBool returns true if the result value is a JSON boolean.
func (t Result) IsBool() bool {
	return t.Type == True || t.Type == False
}

type Iterator struct {
	root Result

	obj bool
	i   int

	key   Result
	value Result
}

func (it *Iterator) init() bool {
	if it.i != -1 {
		return true
	}

	if !it.root.Exists() || it.root.Type != JSON {
		return false
	}

	json := it.root.Raw
	for it.i = 0; it.i < len(json); it.i++ {
		if json[it.i] == '{' {
			it.i, it.key.Type, it.obj = it.i+1, String, true
			return true
		} else if json[it.i] == '[' {
			it.i, it.key.Type, it.key.Num = it.i+1, Number, -1
			return true
		} else if json[it.i] > ' ' {
			return false
		}
	}
	return false
}

func (it *Iterator) Next() bool {
	if !it.init() {
		return false
	}

	json := it.root.Raw

	var str string
	var vesc bool
	var ok bool
	for ; it.i < len(json); it.i++ {
		if it.obj {
			if json[it.i] != '"' {
				continue
			}
			s := it.i
			it.i, str, vesc, ok = parseString(json, it.i+1)
			if !ok {
				return false
			}
			if vesc {
				it.key.Str = unescape(str[1 : len(str)-1])
			} else {
				it.key.Str = str[1 : len(str)-1]
			}
			it.key.Raw = str
			it.key.Index = s + it.root.Index
		} else {
			it.key.Num += 1
		}
		for ; it.i < len(json); it.i++ {
			if json[it.i] <= ' ' || json[it.i] == ',' || json[it.i] == ':' {
				continue
			}
			break
		}
		s := it.i
		it.i, it.value, ok = parseAny(json, it.i, true)
		if !ok {
			return false
		}
		it.value.Index = s + it.root.Index
		return true
	}
	return false
}

func (it *Iterator) Key() Result {
	return it.key
}

func (it *Iterator) Value() Result {
	return it.value
}

func (t Result) Range() *Iterator {
	return &Iterator{root: t, i: -1}
}

// ForEach iterates through values.
// If the result represents a non-existent value, then no values will be
// iterated. If the result is an Object, the iterator will pass the key and
// value of each item. If the result is an Array, the iterator will pass the
// index and value of each item.
func (t Result) ForEach(iterator func(key, value Result) bool) {
	it := t.Range()
	for it.Next() {
		if !iterator(it.Key(), it.Value()) {
			return
		}
	}
}

// Map returns back a map of values. The result should be a JSON object.
// If the result is not a JSON object, the return value will be an empty map.
func (t Result) Map() map[string]Result {
	if t.Type != JSON {
		return map[string]Result{}
	}
	r := t.arrayOrMap('{', false)
	return r.o
}

// Get searches result for the specified path.
// The result should be a JSON array or object.
func (t Result) Get(path string) Result {
	r := Get(t.Raw, path)
	r.Index += t.Index
	return r
}

type arrayOrMapResult struct {
	a  []Result
	ai []interface{}
	o  map[string]Result
	oi map[string]interface{}
	vc byte
}

func (t Result) arrayOrMap(vc byte, valueize bool) (r arrayOrMapResult) {
	var json = t.Raw
	var i int
	var value Result
	var count int
	var key Result
	if vc == 0 {
		for ; i < len(json); i++ {
			if json[i] == '{' || json[i] == '[' {
				r.vc = json[i]
				i++
				break
			}
			if json[i] > ' ' {
				return
			}
		}
	} else {
		for ; i < len(json); i++ {
			if json[i] == vc {
				i++
				break
			}
			if json[i] > ' ' {
				return
			}
		}
		r.vc = vc
	}
	if r.vc == '{' {
		if valueize {
			r.oi = make(map[string]interface{})
		} else {
			r.o = make(map[string]Result)
		}
	} else {
		if valueize {
			r.ai = make([]interface{}, 0)
		} else {
			r.a = make([]Result, 0)
		}
	}
	for ; i < len(json); i++ {
		if json[i] <= ' ' {
			continue
		}
		// get next value
		if json[i] == ']' || json[i] == '}' {
			break
		}
		switch json[i] {
		default:
			if (json[i] >= '0' && json[i] <= '9') || json[i] == '-' {
				value.Type = Number
				value.Raw, value.Num = tonum(json[i:])
				value.Str = ""
			} else {
				continue
			}
		case '{', '[':
			value.Type = JSON
			value.Raw = squash(json[i:])
			value.Str, value.Num = "", 0
		case 'n':
			value.Type = Null
			value.Raw = tolit(json[i:])
			value.Str, value.Num = "", 0
		case 't':
			value.Type = True
			value.Raw = tolit(json[i:])
			value.Str, value.Num = "", 0
		case 'f':
			value.Type = False
			value.Raw = tolit(json[i:])
			value.Str, value.Num = "", 0
		case '"':
			value.Type = String
			value.Raw, value.Str = tostr(json[i:])
			value.Num = 0
		}
		value.Index = i + t.Index

		i += len(value.Raw) - 1

		if r.vc == '{' {
			if count%2 == 0 {
				key = value
			} else {
				if valueize {
					if _, ok := r.oi[key.Str]; !ok {
						r.oi[key.Str] = value.Value()
					}
				} else {
					if _, ok := r.o[key.Str]; !ok {
						r.o[key.Str] = value
					}
				}
			}
			count++
		} else {
			if valueize {
				r.ai = append(r.ai, value.Value())
			} else {
				r.a = append(r.a, value)
			}
		}
	}
	return
}

// Parse parses the json and returns a result.
//
// This function expects that the json is well-formed, and does not validate.
// Invalid json will not panic, but it may return back unexpected results.
// If you are consuming JSON from an unpredictable source then you may want to
// use the Valid function first.
func Parse(json string) Result {
	var value Result
	i := 0
	for ; i < len(json); i++ {
		if json[i] == '{' || json[i] == '[' {
			value.Type = JSON
			value.Raw = json[i:] // just take the entire raw
			break
		}
		if json[i] <= ' ' {
			continue
		}
		switch json[i] {
		case '+', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
			'i', 'I', 'N':
			value.Type = Number
			value.Raw, value.Num = tonum(json[i:])
		case 'n':
			if i+1 < len(json) && json[i+1] != 'u' {
				// nan
				value.Type = Number
				value.Raw, value.Num = tonum(json[i:])
			} else {
				// null
				value.Type = Null
				value.Raw = tolit(json[i:])
			}
		case 't':
			value.Type = True
			value.Raw = tolit(json[i:])
		case 'f':
			value.Type = False
			value.Raw = tolit(json[i:])
		case '"':
			value.Type = String
			value.Raw, value.Str = tostr(json[i:])
		default:
			return Result{}
		}
		break
	}
	if value.Exists() {
		value.Index = i
	}
	return value
}

// ParseBytes parses the json and returns a result.
// If working with bytes, this method preferred over Parse(string(data))
func ParseBytes(json []byte) Result {
	return Parse(string(json))
}

func squash(json string) string {
	// expects that the lead character is a '[' or '{' or '(' or '"'
	// squash the value, ignoring all nested arrays and objects.
	var i, depth int
	if json[0] != '"' {
		i, depth = 1, 1
	}
	for ; i < len(json); i++ {
		if json[i] >= '"' && json[i] <= '}' {
			switch json[i] {
			case '"':
				i++
				s2 := i
				for ; i < len(json); i++ {
					if json[i] > '\\' {
						continue
					}
					if json[i] == '"' {
						// look for an escaped slash
						if json[i-1] == '\\' {
							n := 0
							for j := i - 2; j > s2-1; j-- {
								if json[j] != '\\' {
									break
								}
								n++
							}
							if n%2 == 0 {
								continue
							}
						}
						break
					}
				}
				if depth == 0 {
					if i >= len(json) {
						return json
					}
					return json[:i+1]
				}
			case '{', '[', '(':
				depth++
			case '}', ']', ')':
				depth--
				if depth == 0 {
					return json[:i+1]
				}
			}
		}
	}
	return json
}

func tonum(json string) (raw string, num float64) {
	for i := 1; i < len(json); i++ {
		// less than dash might have valid characters
		if json[i] <= '-' {
			if json[i] <= ' ' || json[i] == ',' {
				// break on whitespace and comma
				raw = json[:i]
				num, _ = strconv.ParseFloat(raw, 64)
				return
			}
			// could be a '+' or '-'. let's assume so.
		} else if json[i] == ']' || json[i] == '}' {
			// break on ']' or '}'
			raw = json[:i]
			num, _ = strconv.ParseFloat(raw, 64)
			return
		}
	}
	raw = json
	num, _ = strconv.ParseFloat(raw, 64)
	return
}

func tolit(json string) (raw string) {
	for i := 1; i < len(json); i++ {
		if json[i] < 'a' || json[i] > 'z' {
			return json[:i]
		}
	}
	return json
}

func tostr(json string) (raw string, str string) {
	// expects that the lead character is a '"'
	for i := 1; i < len(json); i++ {
		if json[i] > '\\' {
			continue
		}
		if json[i] == '"' {
			return json[:i+1], json[1:i]
		}
		if json[i] == '\\' {
			i++
			for ; i < len(json); i++ {
				if json[i] > '\\' {
					continue
				}
				if json[i] == '"' {
					// look for an escaped slash
					if json[i-1] == '\\' {
						n := 0
						for j := i - 2; j > 0; j-- {
							if json[j] != '\\' {
								break
							}
							n++
						}
						if n%2 == 0 {
							continue
						}
					}
					return json[:i+1], unescape(json[1:i])
				}
			}
			var ret string
			if i+1 < len(json) {
				ret = json[:i+1]
			} else {
				ret = json[:i]
			}
			return ret, unescape(json[1:i])
		}
	}
	return json, json[1:]
}

// Exists returns true if value exists.
//
//  if gjson.Get(json, "name.last").Exists(){
//		println("value exists")
//  }
func (t Result) Exists() bool {
	return t.Type != Null || len(t.Raw) != 0
}

// Value returns one of these types:
//
//	bool, for JSON booleans
//	float64, for JSON numbers
//	Number, for JSON numbers
//	string, for JSON string literals
//	nil, for JSON null
//	map[string]interface{}, for JSON objects
//	[]interface{}, for JSON arrays
//
func (t Result) Value() interface{} {
	if t.Type == String {
		return t.Str
	}
	switch t.Type {
	default:
		return nil
	case False:
		return false
	case Number:
		return t.Num
	case JSON:
		r := t.arrayOrMap(0, true)
		if r.vc == '{' {
			return r.oi
		} else if r.vc == '[' {
			return r.ai
		}
		return nil
	case True:
		return true
	}
}

func parseString(json string, i int) (int, string, bool, bool) {
	var s = i
	for ; i < len(json); i++ {
		if json[i] > '\\' {
			continue
		}
		if json[i] == '"' {
			return i + 1, json[s-1 : i+1], false, true
		}
		if json[i] == '\\' {
			i++
			for ; i < len(json); i++ {
				if json[i] > '\\' {
					continue
				}
				if json[i] == '"' {
					// look for an escaped slash
					if json[i-1] == '\\' {
						n := 0
						for j := i - 2; j > 0; j-- {
							if json[j] != '\\' {
								break
							}
							n++
						}
						if n%2 == 0 {
							continue
						}
					}
					return i + 1, json[s-1 : i+1], true, true
				}
			}
			break
		}
	}
	return i, json[s-1:], false, false
}

func parseNumber(json string, i int) (int, string) {
	var s = i
	i++
	for ; i < len(json); i++ {
		if json[i] <= ' ' || json[i] == ',' || json[i] == ']' ||
			json[i] == '}' {
			return i, json[s:i]
		}
	}
	return i, json[s:]
}

func parseLiteral(json string, i int) (int, string) {
	var s = i
	i++
	for ; i < len(json); i++ {
		if json[i] < 'a' || json[i] > 'z' {
			return i, json[s:i]
		}
	}
	return i, json[s:]
}

func getReferenceToken(pointer string) (token, rest string) {
	// find the end of the pointer or the next '/'
	sep, escaped := -1, false
	for i := 0; i < len(pointer); i++ {
		c := pointer[i]
		if c == '/' {
			sep = i
			break
		} else if c == '~' {
			escaped = true
		}
	}

	ref := ""
	if sep == -1 {
		ref, rest = pointer, ""
	} else {
		ref = pointer[:sep]
		for sep < len(pointer) && pointer[sep] == '/' {
			sep++
		}
		rest = pointer[sep:]
	}

	if escaped {
		var b strings.Builder
		b.Grow(len(ref))
		for i := 0; i < len(ref); i++ {
			c := ref[i]
			if c == '~' {
				i++
				if i == len(ref) {
					b.WriteByte(c)
					break
				}

				c = ref[i]
				if c == '0' {
					c = '~'
				} else if c == '1' {
					c = '/'
				}
			}
			b.WriteByte(c)
		}
		ref = b.String()
	}

	return ref, rest
}

func getArrayIndex(pointer string) (index int, rest string) {
	ref, rest := getReferenceToken(pointer)
	if ref == "" {
		return -1, ""
	}

	index = 0
	for i := 0; i < len(ref); i++ {
		c := ref[i]
		if c < '0' || c > '9' {
			return -1, ""
		}
		index = index*10 + int(c) - '0'
	}
	return index, rest
}

func parseSquash(json string, i int) (int, string) {
	// expects that the lead character is a '[' or '{' or '('
	// squash the value, ignoring all nested arrays and objects.
	// the first '[' or '{' or '(' has already been read
	s := i
	i++
	depth := 1
	for ; i < len(json); i++ {
		if json[i] >= '"' && json[i] <= '}' {
			switch json[i] {
			case '"':
				i++
				s2 := i
				for ; i < len(json); i++ {
					if json[i] > '\\' {
						continue
					}
					if json[i] == '"' {
						// look for an escaped slash
						if json[i-1] == '\\' {
							n := 0
							for j := i - 2; j > s2-1; j-- {
								if json[j] != '\\' {
									break
								}
								n++
							}
							if n%2 == 0 {
								continue
							}
						}
						break
					}
				}
			case '{', '[', '(':
				depth++
			case '}', ']', ')':
				depth--
				if depth == 0 {
					i++
					return i, json[s:i]
				}
			}
		}
	}
	return i, json[s:]
}

func parseObject(c *parseContext, i int, pointer string) (int, bool) {
	var pmatch, kesc, vesc, ok, hit bool
	var ref, key, val string
	ref, pointer = getReferenceToken(pointer)
	if ref == "" {
		// return the entire object
		i, val = parseSquash(c.json, i-1)
		c.value.Raw = val
		c.value.Type = JSON
		return i, true
	}
	more := pointer != ""

	for i < len(c.json) {
		for ; i < len(c.json); i++ {
			if c.json[i] == '"' {
				// parse_key_string
				// this is slightly different from getting s string value
				// because we don't need the outer quotes.
				i++
				var s = i
				for ; i < len(c.json); i++ {
					if c.json[i] > '\\' {
						continue
					}
					if c.json[i] == '"' {
						i, key, kesc, ok = i+1, c.json[s:i], false, true
						goto parse_key_string_done
					}
					if c.json[i] == '\\' {
						i++
						for ; i < len(c.json); i++ {
							if c.json[i] > '\\' {
								continue
							}
							if c.json[i] == '"' {
								// look for an escaped slash
								if c.json[i-1] == '\\' {
									n := 0
									for j := i - 2; j > 0; j-- {
										if c.json[j] != '\\' {
											break
										}
										n++
									}
									if n%2 == 0 {
										continue
									}
								}
								i, key, kesc, ok = i+1, c.json[s:i], true, true
								goto parse_key_string_done
							}
						}
						break
					}
				}
				key, kesc, ok = c.json[s:], false, false
			parse_key_string_done:
				break
			}
			if c.json[i] == '}' {
				return i + 1, false
			}
		}
		if !ok {
			return i, false
		}
		if kesc {
			pmatch = ref == unescape(key)
		} else {
			pmatch = ref == key
		}

		hit = pmatch && !more
		for ; i < len(c.json); i++ {
			var num bool
			switch c.json[i] {
			default:
				continue
			case '"':
				i++
				i, val, vesc, ok = parseString(c.json, i)
				if !ok {
					return i, false
				}
				if hit {
					if vesc {
						c.value.Str = unescape(val[1 : len(val)-1])
					} else {
						c.value.Str = val[1 : len(val)-1]
					}
					c.value.Raw = val
					c.value.Type = String
					return i, true
				}
			case '{':
				if pmatch && !hit {
					i, hit = parseObject(c, i+1, pointer)
					if hit {
						return i, true
					}
				} else {
					i, val = parseSquash(c.json, i)
					if hit {
						c.value.Raw = val
						c.value.Type = JSON
						return i, true
					}
				}
			case '[':
				if pmatch && !hit {
					i, hit = parseArray(c, i+1, pointer)
					if hit {
						return i, true
					}
				} else {
					i, val = parseSquash(c.json, i)
					if hit {
						c.value.Raw = val
						c.value.Type = JSON
						return i, true
					}
				}
			case 'n':
				if i+1 < len(c.json) && c.json[i+1] != 'u' {
					num = true
					break
				}
				fallthrough
			case 't', 'f':
				vc := c.json[i]
				i, val = parseLiteral(c.json, i)
				if hit {
					c.value.Raw = val
					switch vc {
					case 't':
						c.value.Type = True
					case 'f':
						c.value.Type = False
					}
					return i, true
				}
			case '+', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
				'i', 'I', 'N':
				num = true
			}
			if num {
				i, val = parseNumber(c.json, i)
				if hit {
					c.value.Raw = val
					c.value.Type = Number
					c.value.Num, _ = strconv.ParseFloat(val, 64)
					return i, true
				}
			}
			break
		}
	}
	return i, false
}

func parseArray(c *parseContext, i int, pointer string) (int, bool) {
	var pmatch, vesc, ok, hit bool
	var val string
	var h int
	var partidx int
	partidx, pointer = getArrayIndex(pointer)
	if partidx == -1 {
		// return the entire object
		i, val = parseSquash(c.json, i-1)
		c.value.Raw = val
		c.value.Type = JSON
		return i, true
	}
	more := pointer != ""

	for i < len(c.json)+1 {
		pmatch = partidx == h
		hit = pmatch && !more

		h++
		for ; ; i++ {
			var ch byte
			if i > len(c.json) {
				break
			} else if i == len(c.json) {
				ch = ']'
			} else {
				ch = c.json[i]
			}
			var num bool
			switch ch {
			default:
				continue
			case '"':
				i++
				i, val, vesc, ok = parseString(c.json, i)
				if !ok {
					return i, false
				}
				if hit {
					if vesc {
						c.value.Str = unescape(val[1 : len(val)-1])
					} else {
						c.value.Str = val[1 : len(val)-1]
					}
					c.value.Raw = val
					c.value.Type = String
					return i, true
				}
			case '{':
				if pmatch && !hit {
					i, hit = parseObject(c, i+1, pointer)
					if hit {
						return i, true
					}
				} else {
					i, val = parseSquash(c.json, i)
					if hit {
						c.value.Raw = val
						c.value.Type = JSON
						return i, true
					}
				}
			case '[':
				if pmatch && !hit {
					i, hit = parseArray(c, i+1, pointer)
					if hit {
						return i, true
					}
				} else {
					i, val = parseSquash(c.json, i)
					if hit {
						c.value.Raw = val
						c.value.Type = JSON
						return i, true
					}
				}
			case 'n':
				if i+1 < len(c.json) && c.json[i+1] != 'u' {
					num = true
					break
				}
				fallthrough
			case 't', 'f':
				vc := c.json[i]
				i, val = parseLiteral(c.json, i)
				if hit {
					c.value.Raw = val
					switch vc {
					case 't':
						c.value.Type = True
					case 'f':
						c.value.Type = False
					}
					return i, true
				}
			case '+', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
				'i', 'I', 'N':
				num = true
			case ']':
				return i + 1, false
			}
			if num {
				i, val = parseNumber(c.json, i)
				if hit {
					c.value.Raw = val
					c.value.Type = Number
					c.value.Num, _ = strconv.ParseFloat(val, 64)
					return i, true
				}
			}
			break
		}
	}
	return i, false
}

type parseContext struct {
	json  string
	value Result
}

// Get searches json for the specified RFC 6901 JSON pointer.
//
// This function expects that the json is well-formed, and does not validate.
// Invalid json will not panic, but it may return back unexpected results.
// If you are consuming JSON from an unpredictable source then you may want to
// use the Valid function first.
func Get(json, pointer string) Result {
	for len(pointer) > 0 && pointer[0] == '/' {
		pointer = pointer[1:]
	}
	if pointer == "" {
		_, result, _ := parseAny(json, 0, true)
		return result
	}

	c := parseContext{json: json}
	for i := 0; i < len(c.json); i++ {
		if c.json[i] == '{' {
			i++
			parseObject(&c, i, pointer)
			break
		}
		if c.json[i] == '[' {
			i++
			parseArray(&c, i, pointer)
			break
		}
	}
	fillIndex(json, &c)
	return c.value
}

// GetBytes searches json for the specified path.
// If working with bytes, this method preferred over Get(string(data), path)
func GetBytes(json []byte, path string) Result {
	return getBytes(json, path)
}

// runeit returns the rune from the the \uXXXX
func runeit(json string) rune {
	n, _ := strconv.ParseUint(json[:4], 16, 64)
	return rune(n)
}

// unescape unescapes a string
func unescape(json string) string {
	var str = make([]byte, 0, len(json))
	for i := 0; i < len(json); i++ {
		switch {
		default:
			str = append(str, json[i])
		case json[i] < ' ':
			return string(str)
		case json[i] == '\\':
			i++
			if i >= len(json) {
				return string(str)
			}
			switch json[i] {
			default:
				return string(str)
			case '\\':
				str = append(str, '\\')
			case '/':
				str = append(str, '/')
			case 'b':
				str = append(str, '\b')
			case 'f':
				str = append(str, '\f')
			case 'n':
				str = append(str, '\n')
			case 'r':
				str = append(str, '\r')
			case 't':
				str = append(str, '\t')
			case '"':
				str = append(str, '"')
			case 'u':
				if i+5 > len(json) {
					return string(str)
				}
				r := runeit(json[i+1:])
				i += 5
				if utf16.IsSurrogate(r) {
					// need another code
					if len(json[i:]) >= 6 && json[i] == '\\' &&
						json[i+1] == 'u' {
						// we expect it to be correct so just consume it
						r = utf16.DecodeRune(r, runeit(json[i+2:]))
						i += 6
					}
				}
				// provide enough space to encode the largest utf8 possible
				str = append(str, 0, 0, 0, 0, 0, 0, 0, 0)
				n := utf8.EncodeRune(str[len(str)-8:], r)
				str = str[:len(str)-8+n]
				i-- // backtrack index by one
			}
		}
	}
	return string(str)
}

// Less return true if a token is less than another token.
// The caseSensitive paramater is used when the tokens are Strings.
// The order when comparing two different type is:
//
//  Null < False < Number < String < True < JSON
//
func (t Result) Less(token Result, caseSensitive bool) bool {
	if t.Type < token.Type {
		return true
	}
	if t.Type > token.Type {
		return false
	}
	if t.Type == String {
		if caseSensitive {
			return t.Str < token.Str
		}
		return stringLessInsensitive(t.Str, token.Str)
	}
	if t.Type == Number {
		return t.Num < token.Num
	}
	return t.Raw < token.Raw
}

func stringLessInsensitive(a, b string) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] >= 'A' && a[i] <= 'Z' {
			if b[i] >= 'A' && b[i] <= 'Z' {
				// both are uppercase, do nothing
				if a[i] < b[i] {
					return true
				} else if a[i] > b[i] {
					return false
				}
			} else {
				// a is uppercase, convert a to lowercase
				if a[i]+32 < b[i] {
					return true
				} else if a[i]+32 > b[i] {
					return false
				}
			}
		} else if b[i] >= 'A' && b[i] <= 'Z' {
			// b is uppercase, convert b to lowercase
			if a[i] < b[i]+32 {
				return true
			} else if a[i] > b[i]+32 {
				return false
			}
		} else {
			// neither are uppercase
			if a[i] < b[i] {
				return true
			} else if a[i] > b[i] {
				return false
			}
		}
	}
	return len(a) < len(b)
}

// parseAny parses the next value from a json string.
// A Result is returned when the hit param is set.
// The return values are (i int, res Result, ok bool)
func parseAny(json string, i int, hit bool) (int, Result, bool) {
	var res Result
	var val string
	for ; i < len(json); i++ {
		if json[i] == '{' || json[i] == '[' {
			i, val = parseSquash(json, i)
			if hit {
				res.Raw = val
				res.Type = JSON
			}
			var tmp parseContext
			tmp.value = res
			fillIndex(json, &tmp)
			return i, tmp.value, true
		}
		if json[i] <= ' ' {
			continue
		}
		var num bool
		switch json[i] {
		case '"':
			i++
			var vesc bool
			var ok bool
			i, val, vesc, ok = parseString(json, i)
			if !ok {
				return i, res, false
			}
			if hit {
				res.Type = String
				res.Raw = val
				if vesc {
					res.Str = unescape(val[1 : len(val)-1])
				} else {
					res.Str = val[1 : len(val)-1]
				}
			}
			return i, res, true
		case 'n':
			if i+1 < len(json) && json[i+1] != 'u' {
				num = true
				break
			}
			fallthrough
		case 't', 'f':
			vc := json[i]
			i, val = parseLiteral(json, i)
			if hit {
				res.Raw = val
				switch vc {
				case 't':
					res.Type = True
				case 'f':
					res.Type = False
				}
				return i, res, true
			}
		case '+', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
			'i', 'I', 'N':
			num = true
		}
		if num {
			i, val = parseNumber(json, i)
			if hit {
				res.Raw = val
				res.Type = Number
				res.Num, _ = strconv.ParseFloat(val, 64)
			}
			return i, res, true
		}

	}
	return i, res, false
}

// GetMany searches json for the multiple paths.
// The return value is a Result array where the number of items
// will be equal to the number of input paths.
func GetMany(json string, path ...string) []Result {
	res := make([]Result, len(path))
	for i, path := range path {
		res[i] = Get(json, path)
	}
	return res
}

// GetManyBytes searches json for the multiple paths.
// The return value is a Result array where the number of items
// will be equal to the number of input paths.
func GetManyBytes(json []byte, path ...string) []Result {
	res := make([]Result, len(path))
	for i, path := range path {
		res[i] = GetBytes(json, path)
	}
	return res
}

func validpayload(data []byte, i int) (outi int, ok bool) {
	for ; i < len(data); i++ {
		switch data[i] {
		default:
			i, ok = validany(data, i)
			if !ok {
				return i, false
			}
			for ; i < len(data); i++ {
				switch data[i] {
				default:
					return i, false
				case ' ', '\t', '\n', '\r':
					continue
				}
			}
			return i, true
		case ' ', '\t', '\n', '\r':
			continue
		}
	}
	return i, false
}
func validany(data []byte, i int) (outi int, ok bool) {
	for ; i < len(data); i++ {
		switch data[i] {
		default:
			return i, false
		case ' ', '\t', '\n', '\r':
			continue
		case '{':
			return validobject(data, i+1)
		case '[':
			return validarray(data, i+1)
		case '"':
			return validstring(data, i+1)
		case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return validnumber(data, i+1)
		case 't':
			return validtrue(data, i+1)
		case 'f':
			return validfalse(data, i+1)
		case 'n':
			return validnull(data, i+1)
		}
	}
	return i, false
}
func validobject(data []byte, i int) (outi int, ok bool) {
	for ; i < len(data); i++ {
		switch data[i] {
		default:
			return i, false
		case ' ', '\t', '\n', '\r':
			continue
		case '}':
			return i + 1, true
		case '"':
		key:
			if i, ok = validstring(data, i+1); !ok {
				return i, false
			}
			if i, ok = validcolon(data, i); !ok {
				return i, false
			}
			if i, ok = validany(data, i); !ok {
				return i, false
			}
			if i, ok = validcomma(data, i, '}'); !ok {
				return i, false
			}
			if data[i] == '}' {
				return i + 1, true
			}
			i++
			for ; i < len(data); i++ {
				switch data[i] {
				default:
					return i, false
				case ' ', '\t', '\n', '\r':
					continue
				case '"':
					goto key
				}
			}
			return i, false
		}
	}
	return i, false
}
func validcolon(data []byte, i int) (outi int, ok bool) {
	for ; i < len(data); i++ {
		switch data[i] {
		default:
			return i, false
		case ' ', '\t', '\n', '\r':
			continue
		case ':':
			return i + 1, true
		}
	}
	return i, false
}
func validcomma(data []byte, i int, end byte) (outi int, ok bool) {
	for ; i < len(data); i++ {
		switch data[i] {
		default:
			return i, false
		case ' ', '\t', '\n', '\r':
			continue
		case ',':
			return i, true
		case end:
			return i, true
		}
	}
	return i, false
}
func validarray(data []byte, i int) (outi int, ok bool) {
	for ; i < len(data); i++ {
		switch data[i] {
		default:
			for ; i < len(data); i++ {
				if i, ok = validany(data, i); !ok {
					return i, false
				}
				if i, ok = validcomma(data, i, ']'); !ok {
					return i, false
				}
				if data[i] == ']' {
					return i + 1, true
				}
			}
		case ' ', '\t', '\n', '\r':
			continue
		case ']':
			return i + 1, true
		}
	}
	return i, false
}
func validstring(data []byte, i int) (outi int, ok bool) {
	for ; i < len(data); i++ {
		if data[i] < ' ' {
			return i, false
		} else if data[i] == '\\' {
			i++
			if i == len(data) {
				return i, false
			}
			switch data[i] {
			default:
				return i, false
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
			case 'u':
				for j := 0; j < 4; j++ {
					i++
					if i >= len(data) {
						return i, false
					}
					if !((data[i] >= '0' && data[i] <= '9') ||
						(data[i] >= 'a' && data[i] <= 'f') ||
						(data[i] >= 'A' && data[i] <= 'F')) {
						return i, false
					}
				}
			}
		} else if data[i] == '"' {
			return i + 1, true
		}
	}
	return i, false
}
func validnumber(data []byte, i int) (outi int, ok bool) {
	i--
	// sign
	if data[i] == '-' {
		i++
		if i == len(data) {
			return i, false
		}
		if data[i] < '0' || data[i] > '9' {
			return i, false
		}
	}
	// int
	if i == len(data) {
		return i, false
	}
	if data[i] == '0' {
		i++
	} else {
		for ; i < len(data); i++ {
			if data[i] >= '0' && data[i] <= '9' {
				continue
			}
			break
		}
	}
	// frac
	if i == len(data) {
		return i, true
	}
	if data[i] == '.' {
		i++
		if i == len(data) {
			return i, false
		}
		if data[i] < '0' || data[i] > '9' {
			return i, false
		}
		i++
		for ; i < len(data); i++ {
			if data[i] >= '0' && data[i] <= '9' {
				continue
			}
			break
		}
	}
	// exp
	if i == len(data) {
		return i, true
	}
	if data[i] == 'e' || data[i] == 'E' {
		i++
		if i == len(data) {
			return i, false
		}
		if data[i] == '+' || data[i] == '-' {
			i++
		}
		if i == len(data) {
			return i, false
		}
		if data[i] < '0' || data[i] > '9' {
			return i, false
		}
		i++
		for ; i < len(data); i++ {
			if data[i] >= '0' && data[i] <= '9' {
				continue
			}
			break
		}
	}
	return i, true
}

func validtrue(data []byte, i int) (outi int, ok bool) {
	if i+3 <= len(data) && data[i] == 'r' && data[i+1] == 'u' &&
		data[i+2] == 'e' {
		return i + 3, true
	}
	return i, false
}
func validfalse(data []byte, i int) (outi int, ok bool) {
	if i+4 <= len(data) && data[i] == 'a' && data[i+1] == 'l' &&
		data[i+2] == 's' && data[i+3] == 'e' {
		return i + 4, true
	}
	return i, false
}
func validnull(data []byte, i int) (outi int, ok bool) {
	if i+3 <= len(data) && data[i] == 'u' && data[i+1] == 'l' &&
		data[i+2] == 'l' {
		return i + 3, true
	}
	return i, false
}

// Valid returns true if the input is valid json.
//
//  if !gjson.Valid(json) {
//  	return errors.New("invalid json")
//  }
//  value := gjson.Get(json, "name.last")
//
func Valid(json string) bool {
	_, ok := validpayload(stringBytes(json), 0)
	return ok
}

// ValidBytes returns true if the input is valid json.
//
//  if !gjson.Valid(json) {
//  	return errors.New("invalid json")
//  }
//  value := gjson.Get(json, "name.last")
//
// If working with bytes, this method preferred over ValidBytes(string(data))
//
func ValidBytes(json []byte) bool {
	_, ok := validpayload(json, 0)
	return ok
}

func parseUint(s string) (n uint64, ok bool) {
	var i int
	if i == len(s) {
		return 0, false
	}
	for ; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			n = n*10 + uint64(s[i]-'0')
		} else {
			return 0, false
		}
	}
	return n, true
}

func parseInt(s string) (n int64, ok bool) {
	var i int
	var sign bool
	if len(s) > 0 && s[0] == '-' {
		sign = true
		i++
	}
	if i == len(s) {
		return 0, false
	}
	for ; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			n = n*10 + int64(s[i]-'0')
		} else {
			return 0, false
		}
	}
	if sign {
		return n * -1, true
	}
	return n, true
}

// safeInt validates a given JSON number
// ensures it lies within the minimum and maximum representable JSON numbers
func safeInt(f float64) (n int64, ok bool) {
	// https://tc39.es/ecma262/#sec-number.min_safe_integer
	// https://tc39.es/ecma262/#sec-number.max_safe_integer
	if f < -9007199254740991 || f > 9007199254740991 {
		return 0, false
	}
	return int64(f), true
}

// stringHeader instead of reflect.StringHeader
type stringHeader struct {
	data unsafe.Pointer
	len  int
}

// sliceHeader instead of reflect.SliceHeader
type sliceHeader struct {
	data unsafe.Pointer
	len  int
	cap  int
}

// getBytes casts the input json bytes to a string and safely returns the
// results as uniquely allocated data. This operation is intended to minimize
// copies and allocations for the large json string->[]byte.
func getBytes(json []byte, path string) Result {
	var result Result
	if json != nil {
		// unsafe cast to string
		result = Get(*(*string)(unsafe.Pointer(&json)), path)
		// safely get the string headers
		rawhi := *(*stringHeader)(unsafe.Pointer(&result.Raw))
		strhi := *(*stringHeader)(unsafe.Pointer(&result.Str))
		// create byte slice headers
		rawh := sliceHeader{data: rawhi.data, len: rawhi.len, cap: rawhi.len}
		strh := sliceHeader{data: strhi.data, len: strhi.len, cap: rawhi.len}
		if strh.data == nil {
			// str is nil
			if rawh.data == nil {
				// raw is nil
				result.Raw = ""
			} else {
				// raw has data, safely copy the slice header to a string
				result.Raw = string(*(*[]byte)(unsafe.Pointer(&rawh)))
			}
			result.Str = ""
		} else if rawh.data == nil {
			// raw is nil
			result.Raw = ""
			// str has data, safely copy the slice header to a string
			result.Str = string(*(*[]byte)(unsafe.Pointer(&strh)))
		} else if uintptr(strh.data) >= uintptr(rawh.data) &&
			uintptr(strh.data)+uintptr(strh.len) <=
				uintptr(rawh.data)+uintptr(rawh.len) {
			// Str is a substring of Raw.
			start := uintptr(strh.data) - uintptr(rawh.data)
			// safely copy the raw slice header
			result.Raw = string(*(*[]byte)(unsafe.Pointer(&rawh)))
			// substring the raw
			result.Str = result.Raw[start : start+uintptr(strh.len)]
		} else {
			// safely copy both the raw and str slice headers to strings
			result.Raw = string(*(*[]byte)(unsafe.Pointer(&rawh)))
			result.Str = string(*(*[]byte)(unsafe.Pointer(&strh)))
		}
	}
	return result
}

// fillIndex finds the position of Raw data and assigns it to the Index field
// of the resulting value. If the position cannot be found then Index zero is
// used instead.
func fillIndex(json string, c *parseContext) {
	if len(c.value.Raw) > 0 {
		jhdr := *(*stringHeader)(unsafe.Pointer(&json))
		rhdr := *(*stringHeader)(unsafe.Pointer(&(c.value.Raw)))
		c.value.Index = int(uintptr(rhdr.data) - uintptr(jhdr.data))
		if c.value.Index < 0 || c.value.Index >= len(json) {
			c.value.Index = 0
		}
	}
}

func stringBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&sliceHeader{
		data: (*stringHeader)(unsafe.Pointer(&s)).data,
		len:  len(s),
		cap:  len(s),
	}))
}

func bytesString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
