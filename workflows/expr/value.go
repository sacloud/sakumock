package expr

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type ValueType int

const (
	TypeNull ValueType = iota
	TypeBool
	TypeNumber
	TypeString
	TypeArray
	TypeObject
)

func (t ValueType) String() string {
	switch t {
	case TypeNull:
		return "null"
	case TypeBool:
		return "boolean"
	case TypeNumber:
		return "number"
	case TypeString:
		return "string"
	case TypeArray:
		return "array"
	case TypeObject:
		return "object"
	default:
		return "unknown"
	}
}

type Value struct {
	typ  ValueType
	bval bool
	nval float64
	sval string
	aval []Value
	oval map[string]Value
}

var Null = Value{typ: TypeNull}

func Bool(b bool) Value      { return Value{typ: TypeBool, bval: b} }
func Number(n float64) Value { return Value{typ: TypeNumber, nval: n} }
func String(s string) Value  { return Value{typ: TypeString, sval: s} }

func Array(items ...Value) Value {
	if items == nil {
		items = []Value{}
	}
	return Value{typ: TypeArray, aval: items}
}

func Object(m map[string]Value) Value {
	if m == nil {
		m = map[string]Value{}
	}
	return Value{typ: TypeObject, oval: m}
}

func (v Value) Type() ValueType            { return v.typ }
func (v Value) IsNull() bool               { return v.typ == TypeNull }
func (v Value) AsBool() bool               { return v.bval }
func (v Value) AsNumber() float64          { return v.nval }
func (v Value) AsString() string           { return v.sval }
func (v Value) AsArray() []Value           { return v.aval }
func (v Value) AsObject() map[string]Value { return v.oval }

func (v Value) Truthy() bool {
	switch v.typ {
	case TypeNull:
		return false
	case TypeBool:
		return v.bval
	case TypeNumber:
		return v.nval != 0 && !math.IsNaN(v.nval)
	case TypeString:
		return v.sval != ""
	case TypeArray, TypeObject:
		return true
	default:
		return false
	}
}

func (v Value) ToNumber() float64 {
	switch v.typ {
	case TypeNull:
		return 0
	case TypeBool:
		if v.bval {
			return 1
		}
		return 0
	case TypeNumber:
		return v.nval
	case TypeString:
		n, err := strconv.ParseFloat(v.sval, 64)
		if err != nil {
			return math.NaN()
		}
		return n
	default:
		return math.NaN()
	}
}

func (v Value) ToString() string {
	switch v.typ {
	case TypeNull:
		return "null"
	case TypeBool:
		if v.bval {
			return "true"
		}
		return "false"
	case TypeNumber:
		if v.nval == math.Trunc(v.nval) && !math.IsInf(v.nval, 0) && !math.IsNaN(v.nval) {
			return strconv.FormatInt(int64(v.nval), 10)
		}
		return strconv.FormatFloat(v.nval, 'f', -1, 64)
	case TypeString:
		return v.sval
	case TypeArray:
		parts := make([]string, len(v.aval))
		for i, item := range v.aval {
			parts[i] = item.ToString()
		}
		return strings.Join(parts, ",")
	case TypeObject:
		b, _ := json.Marshal(v.ToInterface())
		return string(b)
	default:
		return ""
	}
}

func (v Value) ToInterface() any {
	switch v.typ {
	case TypeNull:
		return nil
	case TypeBool:
		return v.bval
	case TypeNumber:
		if v.nval == math.Trunc(v.nval) && !math.IsInf(v.nval, 0) && !math.IsNaN(v.nval) && math.Abs(v.nval) < 1e15 {
			return int64(v.nval)
		}
		return v.nval
	case TypeString:
		return v.sval
	case TypeArray:
		result := make([]any, len(v.aval))
		for i, item := range v.aval {
			result[i] = item.ToInterface()
		}
		return result
	case TypeObject:
		result := make(map[string]any, len(v.oval))
		for k, item := range v.oval {
			result[k] = item.ToInterface()
		}
		return result
	default:
		return nil
	}
}

func FromInterface(v any) Value {
	if v == nil {
		return Null
	}
	switch val := v.(type) {
	case bool:
		return Bool(val)
	case int:
		return Number(float64(val))
	case int64:
		return Number(float64(val))
	case float64:
		return Number(val)
	case string:
		return String(val)
	case []any:
		items := make([]Value, len(val))
		for i, item := range val {
			items[i] = FromInterface(item)
		}
		return Array(items...)
	case map[string]any:
		m := make(map[string]Value, len(val))
		for k, item := range val {
			m[k] = FromInterface(item)
		}
		return Object(m)
	case []Value:
		return Array(val...)
	case Value:
		return val
	default:
		return String(fmt.Sprintf("%v", val))
	}
}

func (v Value) Equal(other Value) bool {
	if v.typ != other.typ {
		return looseEqual(v, other)
	}
	switch v.typ {
	case TypeNull:
		return true
	case TypeBool:
		return v.bval == other.bval
	case TypeNumber:
		return v.nval == other.nval
	case TypeString:
		return v.sval == other.sval
	case TypeArray:
		if len(v.aval) != len(other.aval) {
			return false
		}
		for i := range v.aval {
			if !v.aval[i].Equal(other.aval[i]) {
				return false
			}
		}
		return true
	case TypeObject:
		if len(v.oval) != len(other.oval) {
			return false
		}
		for k, val := range v.oval {
			oval, ok := other.oval[k]
			if !ok || !val.Equal(oval) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func looseEqual(a, b Value) bool {
	if a.typ == TypeNull && b.typ == TypeNull {
		return true
	}
	if a.typ == TypeNull || b.typ == TypeNull {
		return false
	}
	if a.typ == TypeNumber && b.typ == TypeString {
		return a.nval == b.ToNumber()
	}
	if a.typ == TypeString && b.typ == TypeNumber {
		return a.ToNumber() == b.nval
	}
	if a.typ == TypeBool {
		return Number(a.ToNumber()).Equal(b)
	}
	if b.typ == TypeBool {
		return a.Equal(Number(b.ToNumber()))
	}
	return false
}

func (v Value) StrictEqual(other Value) bool {
	if v.typ != other.typ {
		return false
	}
	return v.Equal(other)
}

func (v Value) Less(other Value) bool {
	if v.typ == TypeNumber && other.typ == TypeNumber {
		return v.nval < other.nval
	}
	if v.typ == TypeString && other.typ == TypeString {
		return v.sval < other.sval
	}
	return v.ToNumber() < other.ToNumber()
}

func (v Value) GetMember(key string) (Value, bool) {
	switch v.typ {
	case TypeObject:
		val, ok := v.oval[key]
		return val, ok
	case TypeArray:
		if key == "length" {
			return Number(float64(len(v.aval))), true
		}
		idx, err := strconv.Atoi(key)
		if err != nil || idx < 0 || idx >= len(v.aval) {
			return Null, false
		}
		return v.aval[idx], true
	case TypeString:
		if key == "length" {
			return Number(float64(len(v.sval))), true
		}
		return Null, false
	default:
		return Null, false
	}
}

func (v Value) GetIndex(idx Value) (Value, bool) {
	switch v.typ {
	case TypeArray:
		if idx.typ != TypeNumber {
			return Null, false
		}
		i := int(idx.nval)
		if i < 0 || i >= len(v.aval) {
			return Null, false
		}
		return v.aval[i], true
	case TypeObject:
		key := idx.ToString()
		val, ok := v.oval[key]
		return val, ok
	case TypeString:
		if idx.typ != TypeNumber {
			return Null, false
		}
		i := int(idx.nval)
		if i < 0 || i >= len(v.sval) {
			return Null, false
		}
		return String(string(v.sval[i])), true
	default:
		return Null, false
	}
}
