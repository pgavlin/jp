package jp

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"
)

// TestRandomData is a fuzzing test that throws random data at the Parse
// function looking for panics.
func TestRandomData(t *testing.T) {
	var lstr string
	defer func() {
		if v := recover(); v != nil {
			println("'" + hex.EncodeToString([]byte(lstr)) + "'")
			println("'" + lstr + "'")
			panic(v)
		}
	}()
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, 200)
	for i := 0; i < 2000000; i++ {
		n, err := rand.Read(b[:rand.Int()%len(b)])
		if err != nil {
			t.Fatal(err)
		}
		lstr = string(b[:n])
		GetBytes([]byte(lstr), "/zzzz")
		Parse(lstr)
	}
}

func TestRandomValidStrings(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, 200)
	for i := 0; i < 100000; i++ {
		n, err := rand.Read(b[:rand.Int()%len(b)])
		if err != nil {
			t.Fatal(err)
		}
		sm, err := json.Marshal(string(b[:n]))
		if err != nil {
			t.Fatal(err)
		}
		var su string
		if err := json.Unmarshal([]byte(sm), &su); err != nil {
			t.Fatal(err)
		}
		token := Get(`{"str":`+string(sm)+`}`, "/str")
		if token.Type != String || token.Str != su {
			println("["+token.Raw+"]", "["+token.Str+"]", "["+su+"]",
				"["+string(sm)+"]")
			t.Fatal("string mismatch")
		}
	}
}

func TestEmoji(t *testing.T) {
	const input = `{"utf8":"Example emoji, KO: \ud83d\udd13, \ud83c\udfc3 ` +
		`OK: \u2764\ufe0f "}`
	value := Get(input, "/utf8")
	var s string
	json.Unmarshal([]byte(value.Raw), &s)
	if value.String() != s {
		t.Fatalf("expected '%v', got '%v'", s, value.String())
	}
}

func testEscapePointer(t *testing.T, json, pointer, expect string) {
	if Get(json, pointer).String() != expect {
		t.Fatalf("expected '%v', got '%v'", expect, Get(json, pointer).String())
	}
}

func TestEscapePointer(t *testing.T) {
	json := `{
		"test":{
			"*":"valZ",
			"*v":"val0",
			"keyv*":"val1",
			"key*v":"val2",
			"keyv?":"val3",
			"key?v":"val4",
			"keyv.":"val5",
			"key.v":"val6",
			"keyk*":{"key?":"val7"}
			"/": "val8",
			"~": "val9",
		}
	}`

	testEscapePointer(t, json, "/test/*", "valZ")
	testEscapePointer(t, json, "/test/*v", "val0")
	testEscapePointer(t, json, "/test/keyv*", "val1")
	testEscapePointer(t, json, "/test/key*v", "val2")
	testEscapePointer(t, json, "/test/keyv?", "val3")
	testEscapePointer(t, json, "/test/key?v", "val4")
	testEscapePointer(t, json, "/test/keyv.", "val5")
	testEscapePointer(t, json, "/test/key.v", "val6")
	testEscapePointer(t, json, "/test/keyk*/key?", "val7")
	testEscapePointer(t, json, "/test/~1", "val8")
	testEscapePointer(t, json, "/test/~0", "val9")
}

// this json block is poorly formed on purpose.
var basicJSON = `  {"age":100, "name":{"here":"B\\\"R"},
	"noop":{"what is a wren?":"a bird"},
	"happy":true,"immortal":false,
	"items":[1,2,3,{"tags":[1,2,3],"points":[[1,2],[3,4]]},4,5,6,7],
	"arr":["1",2,"3",{"hello":"world"},"4",5],
	"vals":[1,2,3,{"sadf":sdf"asdf"}],"name":{"first":"tom","last":null},
	"created":"2014-05-16T08:28:06.989Z",
	"loggy":{
		"programmers": [
    	    {
    	        "firstName": "Brett",
    	        "lastName": "McLaughlin",
    	        "email": "aaaa",
				"tag": "good"
    	    },
    	    {
    	        "firstName": "Jason",
    	        "lastName": "Hunter",
    	        "email": "bbbb",
				"tag": "bad"
    	    },
    	    {
    	        "firstName": "Elliotte",
    	        "lastName": "Harold",
    	        "email": "cccc",
				"tag":, "good"
    	    },
			{
				"firstName": 1002.3,
				"age": 101
			}
    	]
	},
	"lastly":{"end...ing":"soon","yay":"final"}
}`

func TestTimeResult(t *testing.T) {
	assert(t, Get(basicJSON, "/created").String() ==
		Get(basicJSON, "/created").Time().Format(time.RFC3339Nano))
}

func TestParseAny(t *testing.T) {
	assert(t, Parse("100").Float() == 100)
	assert(t, Parse("true").Bool())
	assert(t, Parse("false").Bool() == false)
	assert(t, Parse("yikes").Exists() == false)
}

func TestManyVariousPathCounts(t *testing.T) {
	json := `{"a":"a","b":"b","c":"c"}`
	counts := []int{3, 4, 7, 8, 9, 15, 16, 17, 31, 32, 33, 63, 64, 65, 127,
		128, 129, 255, 256, 257, 511, 512, 513}
	pointers := []string{"/a", "/b", "/c"}
	expects := []string{"a", "b", "c"}
	for _, count := range counts {
		var gpointers []string
		for i := 0; i < count; i++ {
			if i < len(pointers) {
				gpointers = append(gpointers, pointers[i])
			} else {
				gpointers = append(gpointers, fmt.Sprintf("/not%d", i))
			}
		}
		results := GetMany(json, gpointers...)
		for i := 0; i < len(pointers); i++ {
			if results[i].String() != expects[i] {
				t.Fatalf("expected '%v', got '%v'", expects[i],
					results[i].String())
			}
		}
	}
}
func TestManyRecursion(t *testing.T) {
	var json string
	var pointer string
	for i := 0; i < 100; i++ {
		json += `{"a":`
		pointer += "/a"
	}
	json += `"b"`
	for i := 0; i < 100; i++ {
		json += `}`
	}
	pointer = pointer[1:]
	assert(t, GetMany(json, pointer)[0].String() == "b")
}
func TestByteSafety(t *testing.T) {
	jsonb := []byte(`{"name":"Janet","age":38}`)
	mtok := GetBytes(jsonb, "/name")
	if mtok.String() != "Janet" {
		t.Fatalf("expected %v, got %v", "Jason", mtok.String())
	}
	mtok2 := GetBytes(jsonb, "/age")
	if mtok2.Raw != "38" {
		t.Fatalf("expected %v, got %v", "Jason", mtok2.Raw)
	}
	jsonb[9] = 'T'
	jsonb[12] = 'd'
	jsonb[13] = 'y'
	if mtok.String() != "Janet" {
		t.Fatalf("expected %v, got %v", "Jason", mtok.String())
	}
}

func get(json, path string) Result {
	return GetBytes([]byte(json), path)
}

func TestIsArrayIsObject(t *testing.T) {
	mtok := get(basicJSON, "/loggy")
	assert(t, mtok.IsObject())
	assert(t, !mtok.IsArray())

	mtok = get(basicJSON, "/loggy/programmers")
	assert(t, !mtok.IsObject())
	assert(t, mtok.IsArray())

	mtok = get(basicJSON, `/loggy/programmers/0/firstName`)
	assert(t, !mtok.IsObject())
	assert(t, !mtok.IsArray())
}

func TestPlus53BitInts(t *testing.T) {
	json := `{"IdentityData":{"GameInstanceId":634866135153775564}}`
	value := Get(json, "/IdentityData/GameInstanceId")
	assert(t, value.Uint() == 634866135153775564)
	assert(t, value.Int() == 634866135153775564)
	assert(t, value.Float() == 634866135153775616)

	json = `{"IdentityData":{"GameInstanceId":634866135153775564.88172}}`
	value = Get(json, "/IdentityData/GameInstanceId")
	assert(t, value.Uint() == 634866135153775616)
	assert(t, value.Int() == 634866135153775616)
	assert(t, value.Float() == 634866135153775616.88172)

	json = `{
		"min_uint64": 0,
		"max_uint64": 18446744073709551615,
		"overflow_uint64": 18446744073709551616,
		"min_int64": -9223372036854775808,
		"max_int64": 9223372036854775807,
		"overflow_int64": 9223372036854775808,
		"min_uint53":  0,
		"max_uint53":  4503599627370495,
		"overflow_uint53": 4503599627370496,
		"min_int53": -2251799813685248,
		"max_int53": 2251799813685247,
		"overflow_int53": 2251799813685248
	}`

	assert(t, Get(json, "/min_uint53").Uint() == 0)
	assert(t, Get(json, "/max_uint53").Uint() == 4503599627370495)
	assert(t, Get(json, "/overflow_uint53").Int() == 4503599627370496)
	assert(t, Get(json, "/min_int53").Int() == -2251799813685248)
	assert(t, Get(json, "/max_int53").Int() == 2251799813685247)
	assert(t, Get(json, "/overflow_int53").Int() == 2251799813685248)
	assert(t, Get(json, "/min_uint64").Uint() == 0)
	assert(t, Get(json, "/max_uint64").Uint() == 18446744073709551615)
	// this next value overflows the max uint64 by one which will just
	// flip the number to zero
	assert(t, Get(json, "/overflow_uint64").Int() == 0)
	assert(t, Get(json, "/min_int64").Int() == -9223372036854775808)
	assert(t, Get(json, "/max_int64").Int() == 9223372036854775807)
	// this next value overflows the max int64 by one which will just
	// flip the number to the negative sign.
	assert(t, Get(json, "/overflow_int64").Int() == -9223372036854775808)
}
func TestIssue38(t *testing.T) {
	// These should not fail, even though the unicode is invalid.
	Get(`["S3O PEDRO DO BUTI\udf93"]`, "/0")
	Get(`["S3O PEDRO DO BUTI\udf93asdf"]`, "/0")
	Get(`["S3O PEDRO DO BUTI\udf93\u"]`, "/0")
	Get(`["S3O PEDRO DO BUTI\udf93\u1"]`, "/0")
	Get(`["S3O PEDRO DO BUTI\udf93\u13"]`, "/0")
	Get(`["S3O PEDRO DO BUTI\udf93\u134"]`, "/0")
	Get(`["S3O PEDRO DO BUTI\udf93\u1345"]`, "/0")
	Get(`["S3O PEDRO DO BUTI\udf93\u1345asd"]`, "/0")
}
func TestTypes(t *testing.T) {
	assert(t, (Result{Type: String}).Type.String() == "String")
	assert(t, (Result{Type: Number}).Type.String() == "Number")
	assert(t, (Result{Type: Null}).Type.String() == "Null")
	assert(t, (Result{Type: False}).Type.String() == "False")
	assert(t, (Result{Type: True}).Type.String() == "True")
	assert(t, (Result{Type: JSON}).Type.String() == "JSON")
	assert(t, (Result{Type: 100}).Type.String() == "")
	// bool
	assert(t, (Result{Type: True}).Bool() == true)
	assert(t, (Result{Type: False}).Bool() == false)
	assert(t, (Result{Type: Number, Num: 1}).Bool() == true)
	assert(t, (Result{Type: Number, Num: 0}).Bool() == false)
	assert(t, (Result{Type: String, Str: "1"}).Bool() == true)
	assert(t, (Result{Type: String, Str: "T"}).Bool() == true)
	assert(t, (Result{Type: String, Str: "t"}).Bool() == true)
	assert(t, (Result{Type: String, Str: "true"}).Bool() == true)
	assert(t, (Result{Type: String, Str: "True"}).Bool() == true)
	assert(t, (Result{Type: String, Str: "TRUE"}).Bool() == true)
	assert(t, (Result{Type: String, Str: "tRuE"}).Bool() == true)
	assert(t, (Result{Type: String, Str: "0"}).Bool() == false)
	assert(t, (Result{Type: String, Str: "f"}).Bool() == false)
	assert(t, (Result{Type: String, Str: "F"}).Bool() == false)
	assert(t, (Result{Type: String, Str: "false"}).Bool() == false)
	assert(t, (Result{Type: String, Str: "False"}).Bool() == false)
	assert(t, (Result{Type: String, Str: "FALSE"}).Bool() == false)
	assert(t, (Result{Type: String, Str: "fAlSe"}).Bool() == false)
	assert(t, (Result{Type: String, Str: "random"}).Bool() == false)

	// int
	assert(t, (Result{Type: String, Str: "1"}).Int() == 1)
	assert(t, (Result{Type: True}).Int() == 1)
	assert(t, (Result{Type: False}).Int() == 0)
	assert(t, (Result{Type: Number, Num: 1}).Int() == 1)
	// uint
	assert(t, (Result{Type: String, Str: "1"}).Uint() == 1)
	assert(t, (Result{Type: True}).Uint() == 1)
	assert(t, (Result{Type: False}).Uint() == 0)
	assert(t, (Result{Type: Number, Num: 1}).Uint() == 1)
	// float
	assert(t, (Result{Type: String, Str: "1"}).Float() == 1)
	assert(t, (Result{Type: True}).Float() == 1)
	assert(t, (Result{Type: False}).Float() == 0)
	assert(t, (Result{Type: Number, Num: 1}).Float() == 1)
}

func TestForEach(t *testing.T) {
	Result{}.ForEach(nil)
	Result{Type: String, Str: "Hello"}.ForEach(func(_, value Result) bool {
		assert(t, value.String() == "Hello")
		return false
	})
	Result{Type: JSON, Raw: "*invalid*"}.ForEach(nil)

	json := ` {"name": {"first": "Janet","last": "Prichard"},
	"asd\nf":"\ud83d\udd13","age": 47}`
	var count int
	ParseBytes([]byte(json)).ForEach(func(key, value Result) bool {
		count++
		return true
	})
	assert(t, count == 3)
	ParseBytes([]byte(`{"bad`)).ForEach(nil)
	ParseBytes([]byte(`{"ok":"bad`)).ForEach(nil)
}

func TestMap(t *testing.T) {
	assert(t, len(ParseBytes([]byte(`"asdf"`)).Map()) == 0)
	assert(t, ParseBytes([]byte(`{"asdf":"ghjk"`)).Map()["asdf"].String() ==
		"ghjk")
	assert(t, len(Result{Type: JSON, Raw: "**invalid**"}.Map()) == 0)
	assert(t, Result{Type: JSON, Raw: "**invalid**"}.Value() == nil)
	assert(t, Result{Type: JSON, Raw: "{"}.Map() != nil)
}

func TestBasic1(t *testing.T) {
	mtok := get(basicJSON, `/loggy/programmers`)
	var count int
	mtok.ForEach(func(key, value Result) bool {
		assert(t, key.Exists())
		assert(t, key.String() == fmt.Sprint(count))
		assert(t, key.Int() == int64(count))
		count++
		if count == 3 {
			return false
		}
		if count == 1 {
			i := 0
			value.ForEach(func(key, value Result) bool {
				switch i {
				case 0:
					if key.String() != "firstName" ||
						value.String() != "Brett" {
						t.Fatalf("expected %v/%v got %v/%v", "firstName",
							"Brett", key.String(), value.String())
					}
				case 1:
					if key.String() != "lastName" ||
						value.String() != "McLaughlin" {
						t.Fatalf("expected %v/%v got %v/%v", "lastName",
							"McLaughlin", key.String(), value.String())
					}
				case 2:
					if key.String() != "email" || value.String() != "aaaa" {
						t.Fatalf("expected %v/%v got %v/%v", "email", "aaaa",
							key.String(), value.String())
					}
				}
				i++
				return true
			})
		}
		return true
	})
	if count != 3 {
		t.Fatalf("expected %v, got %v", 3, count)
	}
}

func TestBasic2(t *testing.T) {
	mtok := get(basicJSON, "/loggy")
	if mtok.Type != JSON {
		t.Fatalf("expected %v, got %v", JSON, mtok.Type)
	}
	if len(mtok.Map()) != 1 {
		t.Fatalf("expected %v, got %v", 1, len(mtok.Map()))
	}
	programmers := mtok.Map()["programmers"]
	if programmers.Array()[1].Map()["firstName"].Str != "Jason" {
		t.Fatalf("expected %v, got %v", "Jason",
			mtok.Map()["programmers"].Array()[1].Map()["firstName"].Str)
	}
}
func TestBasic3(t *testing.T) {
	if Parse(basicJSON).Get("/loggy/programmers").Get("/1").
		Get("firstName").Str != "Jason" {
		t.Fatalf("expected %v, got %v", "Jason", Parse(basicJSON).
			Get("loggy.programmers").Get("1").Get("firstName").Str)
	}
	var token Result
	if token = Parse("-102"); token.Num != -102 {
		t.Fatalf("expected %v, got %v", -102, token.Num)
	}
	if token = Parse("102"); token.Num != 102 {
		t.Fatalf("expected %v, got %v", 102, token.Num)
	}
	if token = Parse("102.2"); token.Num != 102.2 {
		t.Fatalf("expected %v, got %v", 102.2, token.Num)
	}
	if token = Parse(`"hello"`); token.Str != "hello" {
		t.Fatalf("expected %v, got %v", "hello", token.Str)
	}
	if token = Parse(`"\"he\nllo\""`); token.Str != "\"he\nllo\"" {
		t.Fatalf("expected %v, got %v", "\"he\nllo\"", token.Str)
	}
}
func TestBasic4(t *testing.T) {
	if !get(basicJSON, "/name/last").Exists() {
		t.Fatal("expected true, got false")
	}
	token := get(basicJSON, "/name/here")
	if token.String() != "B\\\"R" {
		t.Fatal("expecting 'B\\\"R'", "got", token.String())
	}
	token = get(basicJSON, "/arr/3/hello")
	if token.String() != "world" {
		t.Fatal("expecting 'world'", "got", token.String())
	}
	_ = token.Value().(string)
	token = get(basicJSON, "/name/first")
	if token.String() != "tom" {
		t.Fatal("expecting 'tom'", "got", token.String())
	}
	_ = token.Value().(string)
	token = get(basicJSON, "/name/last")
	if token.String() != "" {
		t.Fatal("expecting ''", "got", token.String())
	}
	if token.Value() != nil {
		t.Fatal("should be nil")
	}
}
func TestBasic5(t *testing.T) {
	token := get(basicJSON, "/age")
	if token.String() != "100" {
		t.Fatal("expecting '100'", "got", token.String())
	}
	_ = token.Value().(float64)
	token = get(basicJSON, "/happy")
	if token.String() != "true" {
		t.Fatal("expecting 'true'", "got", token.String())
	}
	_ = token.Value().(bool)
	token = get(basicJSON, "/immortal")
	if token.String() != "false" {
		t.Fatal("expecting 'false'", "got", token.String())
	}
	_ = token.Value().(bool)
	token = get(basicJSON, "/noop")
	if token.String() != `{"what is a wren?":"a bird"}` {
		t.Fatal("expecting '"+`{"what is a wren?":"a bird"}`+"'", "got",
			token.String())
	}
	_ = token.Value().(map[string]interface{})

	get(basicJSON, "/vals/hello")

	type msi = map[string]interface{}
	type fi = []interface{}
	mm := Parse(basicJSON).Value().(msi)
	fn := mm["loggy"].(msi)["programmers"].(fi)[1].(msi)["firstName"].(string)
	if fn != "Jason" {
		t.Fatalf("expecting %v, got %v", "Jason", fn)
	}
}
func TestUnicode(t *testing.T) {
	var json = `{"key":0,"的情况下解":{"key":1,"的情况":2}}`
	if Get(json, "/的情况下解/key").Num != 1 {
		t.Fatal("fail")
	}
	if Get(json, "/的情况下解/的情况").Num != 2 {
		t.Fatal("fail")
	}
}

func TestUnescape(t *testing.T) {
	unescape(string([]byte{'\\', '\\', 0}))
	unescape(string([]byte{'\\', '/', '\\', 'b', '\\', 'f'}))
}
func assert(t testing.TB, cond bool) {
	if !cond {
		panic("assert failed")
	}
}
func TestLess(t *testing.T) {
	assert(t, !Result{Type: Null}.Less(Result{Type: Null}, true))
	assert(t, Result{Type: Null}.Less(Result{Type: False}, true))
	assert(t, Result{Type: Null}.Less(Result{Type: True}, true))
	assert(t, Result{Type: Null}.Less(Result{Type: JSON}, true))
	assert(t, Result{Type: Null}.Less(Result{Type: Number}, true))
	assert(t, Result{Type: Null}.Less(Result{Type: String}, true))
	assert(t, !Result{Type: False}.Less(Result{Type: Null}, true))
	assert(t, Result{Type: False}.Less(Result{Type: True}, true))
	assert(t, Result{Type: String, Str: "abc"}.Less(Result{Type: String,
		Str: "bcd"}, true))
	assert(t, Result{Type: String, Str: "ABC"}.Less(Result{Type: String,
		Str: "abc"}, true))
	assert(t, !Result{Type: String, Str: "ABC"}.Less(Result{Type: String,
		Str: "abc"}, false))
	assert(t, Result{Type: Number, Num: 123}.Less(Result{Type: Number,
		Num: 456}, true))
	assert(t, !Result{Type: Number, Num: 456}.Less(Result{Type: Number,
		Num: 123}, true))
	assert(t, !Result{Type: Number, Num: 456}.Less(Result{Type: Number,
		Num: 456}, true))
	assert(t, stringLessInsensitive("abcde", "BBCDE"))
	assert(t, stringLessInsensitive("abcde", "bBCDE"))
	assert(t, stringLessInsensitive("Abcde", "BBCDE"))
	assert(t, stringLessInsensitive("Abcde", "bBCDE"))
	assert(t, !stringLessInsensitive("bbcde", "aBCDE"))
	assert(t, !stringLessInsensitive("bbcde", "ABCDE"))
	assert(t, !stringLessInsensitive("Bbcde", "aBCDE"))
	assert(t, !stringLessInsensitive("Bbcde", "ABCDE"))
	assert(t, !stringLessInsensitive("abcde", "ABCDE"))
	assert(t, !stringLessInsensitive("Abcde", "ABCDE"))
	assert(t, !stringLessInsensitive("abcde", "ABCDE"))
	assert(t, !stringLessInsensitive("ABCDE", "ABCDE"))
	assert(t, !stringLessInsensitive("abcde", "abcde"))
	assert(t, !stringLessInsensitive("123abcde", "123Abcde"))
	assert(t, !stringLessInsensitive("123Abcde", "123Abcde"))
	assert(t, !stringLessInsensitive("123Abcde", "123abcde"))
	assert(t, !stringLessInsensitive("123abcde", "123abcde"))
	assert(t, !stringLessInsensitive("124abcde", "123abcde"))
	assert(t, !stringLessInsensitive("124Abcde", "123Abcde"))
	assert(t, !stringLessInsensitive("124Abcde", "123abcde"))
	assert(t, !stringLessInsensitive("124abcde", "123abcde"))
	assert(t, stringLessInsensitive("124abcde", "125abcde"))
	assert(t, stringLessInsensitive("124Abcde", "125Abcde"))
	assert(t, stringLessInsensitive("124Abcde", "125abcde"))
	assert(t, stringLessInsensitive("124abcde", "125abcde"))
}

func TestIssue6(t *testing.T) {
	data := `{
      "code": 0,
      "msg": "",
      "data": {
        "sz002024": {
          "qfqday": [
            [
              "2014-01-02",
              "8.93",
              "9.03",
              "9.17",
              "8.88",
              "621143.00"
            ],
            [
              "2014-01-03",
              "9.03",
              "9.30",
              "9.47",
              "8.98",
              "1624438.00"
            ]
          ]
        }
      }
    }`

	var num []string
	for _, v := range Get(data, "/data/sz002024/qfqday/0").Array() {
		num = append(num, v.String())
	}
	if fmt.Sprintf("%v", num) != "[2014-01-02 8.93 9.03 9.17 8.88 621143.00]" {
		t.Fatalf("invalid result")
	}
}

var exampleJSON = `{
	"widget": {
		"debug": "on",
		"window": {
			"title": "Sample Konfabulator Widget",
			"name": "main_window",
			"width": 500,
			"height": 500
		},
		"image": {
			"src": "Images/Sun.png",
			"hOffset": 250,
			"vOffset": 250,
			"alignment": "center"
		},
		"text": {
			"data": "Click Here",
			"size": 36,
			"style": "bold",
			"vOffset": 100,
			"alignment": "center",
			"onMouseUp": "sun1.opacity = (sun1.opacity / 100) * 90;"
		}
	}
}`

func TestUnmarshalMap(t *testing.T) {
	var m1 = Parse(exampleJSON).Value().(map[string]interface{})
	var m2 map[string]interface{}
	if err := json.Unmarshal([]byte(exampleJSON), &m2); err != nil {
		t.Fatal(err)
	}
	b1, err := json.Marshal(m1)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := json.Marshal(m2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1, b2) {
		t.Fatal("b1 != b2")
	}
}

func TestSingleArrayValue(t *testing.T) {
	var json = `{"key": "value","key2":[1,2,3,4,"A"]}`
	var result = Get(json, "/key")
	var array = result.Array()
	if len(array) != 1 {
		t.Fatal("array is empty")
	}
	if array[0].String() != "value" {
		t.Fatalf("got %s, should be %s", array[0].String(), "value")
	}

	array = Get(json, "/key3").Array()
	if len(array) != 0 {
		t.Fatalf("got '%v', expected '%v'", len(array), 0)
	}

}

var manyJSON = `  {
	"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{
	"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{
	"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{
	"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{
	"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{
	"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{
	"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{"a":{"hello":"world"
	}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}
	"position":{"type":"Point","coordinates":[-115.24,33.09]},
	"loves":["world peace"],
	"name":{"last":"Anderson","first":"Nancy"},
	"age":31
	"":{"a":"emptya","b":"emptyb"},
	"name.last":"Yellow",
	"name.first":"Cat",
}`

var testWatchForFallback bool

func TestManyBasic(t *testing.T) {
	testWatchForFallback = true
	defer func() {
		testWatchForFallback = false
	}()
	testMany := func(shouldFallback bool, expect string, pointers ...string) {
		results := GetManyBytes(
			[]byte(manyJSON),
			pointers...,
		)
		if len(results) != len(pointers) {
			t.Fatalf("expected %v, got %v", len(pointers), len(results))
		}
		if fmt.Sprintf("%v", results) != expect {
			fmt.Printf("%v\n", pointers)
			t.Fatalf("expected %v, got %v", expect, results)
		}
	}
	testMany(false, "[Point]", "/position/type")
	testMany(false, `[["world peace"] 31]`, "/loves", "/age")
	testMany(false, `[["world peace"]]`, "/loves")
	testMany(false, `[{"last":"Anderson","first":"Nancy"} Nancy]`, "/name",
		"/name/first")
	testMany(true, `[]`, strings.Repeat("/a", 40)+"/hello")
	res := Get(manyJSON, strings.Repeat("/a", 48)+"/a")
	testMany(true, `[`+res.String()+`]`, strings.Repeat("/a", 48)+"/a")
	// these should fallback
	testMany(true, `[Cat Nancy]`, "/name.first", "/name/first")
	testMany(true, `[world]`, strings.Repeat("/a", 70)+"/hello")
}
func testMany(t *testing.T, json string, pointers, expected []string) {
	testManyAny(t, json, pointers, expected, true)
	testManyAny(t, json, pointers, expected, false)
}
func testManyAny(t *testing.T, json string, pointers, expected []string,
	bytes bool) {
	var result []Result
	for i := 0; i < 2; i++ {
		var which string
		if i == 0 {
			which = "Get"
			result = nil
			for j := 0; j < len(expected); j++ {
				if bytes {
					result = append(result, GetBytes([]byte(json), pointers[j]))
				} else {
					result = append(result, Get(json, pointers[j]))
				}
			}
		} else if i == 1 {
			which = "GetMany"
			if bytes {
				result = GetManyBytes([]byte(json), pointers...)
			} else {
				result = GetMany(json, pointers...)
			}
		}
		for j := 0; j < len(expected); j++ {
			if result[j].String() != expected[j] {
				t.Fatalf("Using key '%s' for '%s'\nexpected '%v', got '%v'",
					pointers[j], which, expected[j], result[j].String())
			}
		}
	}
}
func TestIssue20(t *testing.T) {
	json := `{ "name": "FirstName", "name1": "FirstName1", ` +
		`"address": "address1", "addressDetails": "address2", }`
	pointers := []string{"/name", "/name1", "/address", "/addressDetails"}
	expected := []string{"FirstName", "FirstName1", "address1", "address2"}
	t.Run("SingleMany", func(t *testing.T) {
		testMany(t, json, pointers,
			expected)
	})
}

func TestIssue21(t *testing.T) {
	json := `{ "Level1Field1":3,
	           "Level1Field4":4,
			   "Level1Field2":{ "Level2Field1":[ "value1", "value2" ],
			   "Level2Field2":{ "Level3Field1":[ { "key1":"value1" } ] } } }`
	pointers := []string{"/Level1Field1", "/Level1Field2/Level2Field1",
		"/Level1Field2/Level2Field2/Level3Field1", "/Level1Field4"}
	expected := []string{"3", `[ "value1", "value2" ]`,
		`[ { "key1":"value1" } ]`, "4"}
	t.Run("SingleMany", func(t *testing.T) {
		testMany(t, json, pointers,
			expected)
	})
}

//func TestRandomMany(t *testing.T) {
//	var lstr string
//	defer func() {
//		if v := recover(); v != nil {
//			println("'" + hex.EncodeToString([]byte(lstr)) + "'")
//			println("'" + lstr + "'")
//			panic(v)
//		}
//	}()
//	rand.Seed(time.Now().UnixNano())
//	b := make([]byte, 512)
//	for i := 0; i < 50000; i++ {
//		n, err := rand.Read(b[:rand.Int()%len(b)])
//		if err != nil {
//			t.Fatal(err)
//		}
//		lstr = string(b[:n])
//		pointers := make([]string, rand.Int()%64)
//		for i := range pointers {
//			var b []byte
//			n := rand.Int() % 5
//			for j := 0; j < n; j++ {
//				if j > 0 {
//					b = append(b, '.')
//				}
//				nn := rand.Int() % 10
//				for k := 0; k < nn; k++ {
//					b = append(b, 'a'+byte(rand.Int()%26))
//				}
//			}
//			pointers[i] = string(b)
//		}
//		GetMany(lstr, pointers...)
//	}
//}

var complicatedJSON = `
{
	"tagged": "OK",
	"Tagged": "KO",
	"NotTagged": true,
	"unsettable": 101,
	"Nested": {
		"Yellow": "Green",
		"yellow": "yellow"
	},
	"nestedTagged": {
		"Green": "Green",
		"Map": {
			"this": "that",
			"and": "the other thing"
		},
		"Ints": {
			"Uint": 99,
			"Uint16": 16,
			"Uint32": 32,
			"Uint64": 65
		},
		"Uints": {
			"int": -99,
			"Int": -98,
			"Int16": -16,
			"Int32": -32,
			"int64": -64,
			"Int64": -65
		},
		"Uints": {
			"Float32": 32.32,
			"Float64": 64.64
		},
		"Byte": 254,
		"Bool": true
	},
	"LeftOut": "you shouldn't be here",
	"SelfPtr": {"tagged":"OK","nestedTagged":{"Ints":{"Uint32":32}}},
	"SelfSlice": [{"tagged":"OK","nestedTagged":{"Ints":{"Uint32":32}}}],
	"SelfSlicePtr": [{"tagged":"OK","nestedTagged":{"Ints":{"Uint32":32}}}],
	"SelfPtrSlice": [{"tagged":"OK","nestedTagged":{"Ints":{"Uint32":32}}}],
	"interface": "Tile38 Rocks!",
	"Interface": "Please Download",
	"Array": [0,2,3,4,5],
	"time": "2017-05-07T13:24:43-07:00",
	"Binary": "R0lGODlhPQBEAPeo",
	"NonBinary": [9,3,100,115]
}
`

func TestLen(t *testing.T) {
	basic := Parse(basicJSON)
	complicated := Parse(complicatedJSON)

	cases := []struct {
		value    Result
		pointer  string
		expected int
	}{
		{basic, "/", 12},
		{basic, "/noop", 1},
		{basic, "/loggy", 1},
		{basic, "/loggy/programmers", 4},
		{basic, "/loggy/programmers/0", 4},
		{complicated, "/", 17},
		{complicated, "/Nested", 2},
		{complicated, "/nestedTagged/Ints", 4},
		{complicated, "/SelfSlice", 1},
		{complicated, "/Array", 5},
		{complicated, "/NonBinary", 4},
	}
	for _, c := range cases {
		t.Run(c.pointer, func(t *testing.T) {
			actual := c.value.Get(c.pointer).Len()
			if actual != c.expected {
				t.Errorf("unexpected len: %v != %v", actual, c.expected)
			}
		})
	}
}

func testvalid(t *testing.T, json string, expect bool) {
	t.Helper()
	_, ok := validpayload([]byte(json), 0)
	if ok != expect {
		t.Fatal("mismatch")
	}
}

func TestValidBasic(t *testing.T) {
	testvalid(t, "0", true)
	testvalid(t, "00", false)
	testvalid(t, "-00", false)
	testvalid(t, "-.", false)
	testvalid(t, "-.123", false)
	testvalid(t, "0.0", true)
	testvalid(t, "10.0", true)
	testvalid(t, "10e1", true)
	testvalid(t, "10EE", false)
	testvalid(t, "10E-", false)
	testvalid(t, "10E+", false)
	testvalid(t, "10E123", true)
	testvalid(t, "10E-123", true)
	testvalid(t, "10E-0123", true)
	testvalid(t, "", false)
	testvalid(t, " ", false)
	testvalid(t, "{}", true)
	testvalid(t, "{", false)
	testvalid(t, "-", false)
	testvalid(t, "-1", true)
	testvalid(t, "-1.", false)
	testvalid(t, "-1.0", true)
	testvalid(t, " -1.0", true)
	testvalid(t, " -1.0 ", true)
	testvalid(t, "-1.0 ", true)
	testvalid(t, "-1.0 i", false)
	testvalid(t, "-1.0 i", false)
	testvalid(t, "true", true)
	testvalid(t, " true", true)
	testvalid(t, " true ", true)
	testvalid(t, " True ", false)
	testvalid(t, " tru", false)
	testvalid(t, "false", true)
	testvalid(t, " false", true)
	testvalid(t, " false ", true)
	testvalid(t, " False ", false)
	testvalid(t, " fals", false)
	testvalid(t, "null", true)
	testvalid(t, " null", true)
	testvalid(t, " null ", true)
	testvalid(t, " Null ", false)
	testvalid(t, " nul", false)
	testvalid(t, " []", true)
	testvalid(t, " [true]", true)
	testvalid(t, " [ true, null ]", true)
	testvalid(t, " [ true,]", false)
	testvalid(t, `{"hello":"world"}`, true)
	testvalid(t, `{ "hello": "world" }`, true)
	testvalid(t, `{ "hello": "world", }`, false)
	testvalid(t, `{"a":"b",}`, false)
	testvalid(t, `{"a":"b","a"}`, false)
	testvalid(t, `{"a":"b","a":}`, false)
	testvalid(t, `{"a":"b","a":1}`, true)
	testvalid(t, `{"a":"b",2"1":2}`, false)
	testvalid(t, `{"a":"b","a": 1, "c":{"hi":"there"} }`, true)
	testvalid(t, `{"a":"b","a": 1, "c":{"hi":"there", "easy":["going",`+
		`{"mixed":"bag"}]} }`, true)
	testvalid(t, `""`, true)
	testvalid(t, `"`, false)
	testvalid(t, `"\n"`, true)
	testvalid(t, `"\"`, false)
	testvalid(t, `"\\"`, true)
	testvalid(t, `"a\\b"`, true)
	testvalid(t, `"a\\b\\\"a"`, true)
	testvalid(t, `"a\\b\\\uFFAAa"`, true)
	testvalid(t, `"a\\b\\\uFFAZa"`, false)
	testvalid(t, `"a\\b\\\uFFA"`, false)
	testvalid(t, string(complicatedJSON), true)
	testvalid(t, string(exampleJSON), true)
	testvalid(t, "[-]", false)
	testvalid(t, "[-.123]", false)
}

var jsonchars = []string{"{", "[", ",", ":", "}", "]", "1", "0", "true",
	"false", "null", `""`, `"\""`, `"a"`}

func makeRandomJSONChars(b []byte) {
	var bb []byte
	for len(bb) < len(b) {
		bb = append(bb, jsonchars[rand.Int()%len(jsonchars)]...)
	}
	copy(b, bb[:len(b)])
}

func TestValidRandom(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, 100000)
	start := time.Now()
	for time.Since(start) < time.Second*3 {
		n := rand.Int() % len(b)
		rand.Read(b[:n])
		validpayload(b[:n], 0)
	}

	start = time.Now()
	for time.Since(start) < time.Second*3 {
		n := rand.Int() % len(b)
		makeRandomJSONChars(b[:n])
		validpayload(b[:n], 0)
	}
}

func TestGetMany47(t *testing.T) {
	json := `{"bar": {"id": 99, "mybar": "my mybar" }, "foo": ` +
		`{"myfoo": [605]}}`
	pointers := []string{"/foo/myfoo", "/bar/id", "/bar/mybar", "/bar/mybarx"}
	expected := []string{"[605]", "99", "my mybar", ""}
	results := GetMany(json, pointers...)
	if len(expected) != len(results) {
		t.Fatalf("expected %v, got %v", len(expected), len(results))
	}
	for i, pointer := range pointers {
		if results[i].String() != expected[i] {
			t.Fatalf("expected '%v', got '%v' for pointer '%v'", expected[i],
				results[i].String(), pointer)
		}
	}
}

func TestGetMany48(t *testing.T) {
	json := `{"bar": {"id": 99, "xyz": "my xyz"}, "foo": {"myfoo": [605]}}`
	pointers := []string{"/foo/myfoo", "/bar/id", "/bar/xyz", "/bar/abc"}
	expected := []string{"[605]", "99", "my xyz", ""}
	results := GetMany(json, pointers...)
	if len(expected) != len(results) {
		t.Fatalf("expected %v, got %v", len(expected), len(results))
	}
	for i, pointer := range pointers {
		if results[i].String() != expected[i] {
			t.Fatalf("expected '%v', got '%v' for pointer '%v'", expected[i],
				results[i].String(), pointer)
		}
	}
}

func TestResultRawForLiteral(t *testing.T) {
	for _, lit := range []string{"null", "true", "false"} {
		result := Parse(lit)
		if result.Raw != lit {
			t.Fatalf("expected '%v', got '%v'", lit, result.Raw)
		}
	}
}

func TestNullArray(t *testing.T) {
	n := len(Get(`{"data":null}`, "data").Array())
	if n != 0 {
		t.Fatalf("expected '%v', got '%v'", 0, n)
	}
	n = len(Get(`{}`, "data").Array())
	if n != 0 {
		t.Fatalf("expected '%v', got '%v'", 0, n)
	}
	n = len(Get(`{"data":[]}`, "data").Array())
	if n != 0 {
		t.Fatalf("expected '%v', got '%v'", 0, n)
	}
	n = len(Get(`{"data":[null]}`, "data").Array())
	if n != 1 {
		t.Fatalf("expected '%v', got '%v'", 1, n)
	}
}

func TestIssue54(t *testing.T) {
	var r []Result
	json := `{"MarketName":null,"Nounce":6115}`
	r = GetMany(json, "/Nounce", "/Buys", "/Sells", "/Fills")
	if strings.Replace(fmt.Sprintf("%v", r), " ", "", -1) != "[6115]" {
		t.Fatalf("expected '%v', got '%v'", "[6115]",
			strings.Replace(fmt.Sprintf("%v", r), " ", "", -1))
	}
	r = GetMany(json, "/Nounce", "/Buys", "/Sells")
	if strings.Replace(fmt.Sprintf("%v", r), " ", "", -1) != "[6115]" {
		t.Fatalf("expected '%v', got '%v'", "[6115]",
			strings.Replace(fmt.Sprintf("%v", r), " ", "", -1))
	}
	r = GetMany(json, "/Nounce")
	if strings.Replace(fmt.Sprintf("%v", r), " ", "", -1) != "[6115]" {
		t.Fatalf("expected '%v', got '%v'", "[6115]",
			strings.Replace(fmt.Sprintf("%v", r), " ", "", -1))
	}
}

func TestIssue55(t *testing.T) {
	json := `{"one": {"two": 2, "three": 3}, "four": 4, "five": 5}`
	results := GetMany(json, "/four", "/five", "/one/two", "/one/six")
	expected := []string{"4", "5", "2", ""}
	for i, r := range results {
		if r.String() != expected[i] {
			t.Fatalf("expected %v, got %v", expected[i], r.String())
		}
	}
}

func TestNumUint64String(t *testing.T) {
	var i int64 = 9007199254740993 //2^53 + 1
	j := fmt.Sprintf(`{"data":  [  %d, "hello" ] }`, i)
	res := Get(j, "/data/0")
	if res.String() != "9007199254740993" {
		t.Fatalf("expected '%v', got '%v'", "9007199254740993", res.String())
	}
}

func TestNumInt64String(t *testing.T) {
	var i int64 = -9007199254740993
	j := fmt.Sprintf(`{"data":[ "hello", %d ]}`, i)
	res := Get(j, "/data/1")
	if res.String() != "-9007199254740993" {
		t.Fatalf("expected '%v', got '%v'", "-9007199254740993", res.String())
	}
}

func TestNumBigString(t *testing.T) {
	i := "900719925474099301239109123101" // very big
	j := fmt.Sprintf(`{"data":[ "hello", "%s" ]}`, i)
	res := Get(j, "/data/1")
	if res.String() != "900719925474099301239109123101" {
		t.Fatalf("expected '%v', got '%v'", "900719925474099301239109123101",
			res.String())
	}
}

func TestNumFloatString(t *testing.T) {
	var i int64 = -9007199254740993
	j := fmt.Sprintf(`{"data":[ "hello", %d ]}`, i) //No quotes around value!!
	res := Get(j, "/data/1")
	if res.String() != "-9007199254740993" {
		t.Fatalf("expected '%v', got '%v'", "-9007199254740993", res.String())
	}
}

func TestDuplicateKeys(t *testing.T) {
	// this is vaild json according to the JSON spec
	var json = `{"name": "Alex","name": "Peter"}`
	if Parse(json).Get("/name").String() !=
		Parse(json).Map()["name"].String() {
		t.Fatalf("expected '%v', got '%v'",
			Parse(json).Get("/name").String(),
			Parse(json).Map()["name"].String(),
		)
	}
	if !Valid(json) {
		t.Fatal("should be valid")
	}
}

func TestArrayValues(t *testing.T) {
	var json = `{"array": ["PERSON1","PERSON2",0],}`
	values := Get(json, "/array").Array()
	var output string
	for i, val := range values {
		if i > 0 {
			output += "\n"
		}
		output += fmt.Sprintf("%#v", val)
	}
	expect := strings.Join([]string{
		`jp.Result{Type:3, Raw:"\"PERSON1\"", Str:"PERSON1", Num:0, ` +
			`Index:11}`,
		`jp.Result{Type:3, Raw:"\"PERSON2\"", Str:"PERSON2", Num:0, ` +
			`Index:21}`,
		`jp.Result{Type:2, Raw:"0", Str:"", Num:0, Index:31}`,
	}, "\n")
	if output != expect {
		t.Fatalf("expected '%v', got '%v'", expect, output)
	}

}

func BenchmarkValid(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Valid(complicatedJSON)
	}
}

func BenchmarkValidBytes(b *testing.B) {
	complicatedJSON := []byte(complicatedJSON)
	for i := 0; i < b.N; i++ {
		ValidBytes(complicatedJSON)
	}
}

func BenchmarkGoStdlibValidBytes(b *testing.B) {
	complicatedJSON := []byte(complicatedJSON)
	for i := 0; i < b.N; i++ {
		json.Valid(complicatedJSON)
	}
}

func TestIssue240(t *testing.T) {
	nonArrayData := `{"jsonrpc":"2.0","method":"subscription","params":
		{"channel":"funny_channel","data":
			{"name":"Jason","company":"good_company","number":12345}
		}
	}`
	parsed := Parse(nonArrayData)
	assert(t, len(parsed.Get("/params/data").Array()) == 1)

	arrayData := `{"jsonrpc":"2.0","method":"subscription","params":
		{"channel":"funny_channel","data":[
			{"name":"Jason","company":"good_company","number":12345}
		]}
	}`
	parsed = Parse(arrayData)
	assert(t, len(parsed.Get("/params/data").Array()) == 1)
}

func TestNaNInf(t *testing.T) {
	json := `[+Inf,-Inf,Inf,iNF,-iNF,+iNF,NaN,nan,nAn,-0,+0]`
	raws := []string{"+Inf", "-Inf", "Inf", "iNF", "-iNF", "+iNF", "NaN", "nan",
		"nAn", "-0", "+0"}
	nums := []float64{math.Inf(+1), math.Inf(-1), math.Inf(0), math.Inf(0),
		math.Inf(-1), math.Inf(+1), math.NaN(), math.NaN(), math.NaN(),
		math.Copysign(0, -1), 0}

	for i := 0; i < len(raws); i++ {
		r := Get(json, fmt.Sprintf("/%d", i))
		assert(t, r.Raw == raws[i])
		assert(t, r.Num == nums[i] || (math.IsNaN(r.Num) && math.IsNaN(nums[i])))
		assert(t, r.Type == Number)
	}

	var i int
	Parse(json).ForEach(func(_, r Result) bool {
		assert(t, r.Raw == raws[i])
		assert(t, r.Num == nums[i] || (math.IsNaN(r.Num) && math.IsNaN(nums[i])))
		assert(t, r.Type == Number)
		i++
		return true
	})

	// Parse should also return valid numbers
	assert(t, math.IsNaN(Parse("nan").Float()))
	assert(t, math.IsNaN(Parse("NaN").Float()))
	assert(t, math.IsNaN(Parse(" NaN").Float()))
	assert(t, math.IsInf(Parse("+inf").Float(), +1))
	assert(t, math.IsInf(Parse("-inf").Float(), -1))
	assert(t, math.IsInf(Parse("+INF").Float(), +1))
	assert(t, math.IsInf(Parse("-INF").Float(), -1))
	assert(t, math.IsInf(Parse(" +INF").Float(), +1))
	assert(t, math.IsInf(Parse(" -INF").Float(), -1))
}

func TestParseIndex(t *testing.T) {
	assert(t, Parse(`{}`).Index == 0)
	assert(t, Parse(` {}`).Index == 1)
	assert(t, Parse(` []`).Index == 1)
	assert(t, Parse(` true`).Index == 1)
	assert(t, Parse(` false`).Index == 1)
	assert(t, Parse(` null`).Index == 1)
	assert(t, Parse(` +inf`).Index == 1)
	assert(t, Parse(` -inf`).Index == 1)
}

const readmeJSON = `
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
`

func TestArrayKeys(t *testing.T) {
	N := 100
	json := "["
	for i := 0; i < N; i++ {
		if i > 0 {
			json += ","
		}
		json += fmt.Sprint(i)
	}
	json += "]"
	var i int
	Parse(json).ForEach(func(key, value Result) bool {
		assert(t, key.String() == fmt.Sprint(i))
		assert(t, key.Int() == int64(i))
		i++
		return true
	})
	assert(t, i == N)
}
