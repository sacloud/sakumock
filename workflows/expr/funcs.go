package expr

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"math/rand/v2"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

func builtinFuncs() map[string]Func {
	return map[string]Func{
		// array
		"array.fill":   arrayFill,
		"array.range":  arrayRange,
		"array.push":   arrayPush,
		"array.set":    arraySet,
		"array.length": arrayLength,

		// json
		"json.decode": jsonDecode,
		"json.encode": jsonEncode,

		// list
		"list.concat":  listConcat,
		"list.prepend": listPrepend,

		// map
		"map.delete":      mapDelete,
		"map.get":         mapGet,
		"map.put":         mapPut,
		"map.merge":       mapMerge,
		"map.mergeNested": mapMergeNested,

		// math
		"math.ceil":    mathCeil,
		"math.sqrt":    mathSqrt,
		"math.abs":     mathAbs,
		"math.max":     mathMax,
		"math.min":     mathMin,
		"math.randint": mathRandint,

		// text
		"text.decode":          textDecode,
		"text.encode":          textEncode,
		"text.findAll":         textFindAll,
		"text.findAllRegex":    textFindAllRegex,
		"text.matchRegex":      textMatchRegex,
		"text.replaceAll":      textReplaceAll,
		"text.replaceAllRegex": textReplaceAllRegex,
		"text.split":           textSplit,
		"text.substring":       textSubstring,
		"text.toLower":         textToLower,
		"text.toUpper":         textToUpper,
		"text.urlDecode":       textURLDecode,
		"text.urlEncode":       textURLEncode,
		"text.urlEncodePlus":   textURLEncodePlus,

		// time
		"time.format": timeFormat,
		"time.parse":  timeParse,
		"time.now":    timeNow,

		// uuid
		"uuid.v7":  uuidV7,
		"uuid.nil": uuidNil,
	}
}

func requireArgs(name string, args []Value, min, max int) error {
	if len(args) < min || len(args) > max {
		if min == max {
			return fmt.Errorf("%s: expected %d arguments, got %d", name, min, len(args))
		}
		return fmt.Errorf("%s: expected %d-%d arguments, got %d", name, min, max, len(args))
	}
	return nil
}

// array functions

func arrayFill(env *Env, args []Value) (Value, error) {
	if err := requireArgs("array.fill", args, 2, 2); err != nil {
		return Null, err
	}
	arr := args[0]
	if arr.typ != TypeArray {
		return Null, fmt.Errorf("array.fill: first argument must be an array")
	}
	result := make([]Value, len(arr.aval))
	for i := range result {
		result[i] = args[1]
	}
	return Array(result...), nil
}

func arrayRange(env *Env, args []Value) (Value, error) {
	if err := requireArgs("array.range", args, 1, 3); err != nil {
		return Null, err
	}
	var start, stop, step float64
	switch len(args) {
	case 1:
		start, stop, step = 0, args[0].ToNumber(), 1
	case 2:
		start, stop, step = args[0].ToNumber(), args[1].ToNumber(), 1
	case 3:
		start, stop, step = args[0].ToNumber(), args[1].ToNumber(), args[2].ToNumber()
	}
	if step == 0 {
		return Null, fmt.Errorf("array.range: step cannot be zero")
	}
	var count int
	if step > 0 {
		count = int((stop - start + step - 1) / step)
	} else {
		count = int((start - stop - step - 1) / -step)
	}
	if count < 0 {
		count = 0
	}
	if count > env.MaxArrayLen {
		return Null, fmt.Errorf("array.range: result length %d exceeds limit %d", count, env.MaxArrayLen)
	}
	result := make([]Value, 0, count)
	if step > 0 {
		for i := start; i < stop; i += step {
			result = append(result, Number(i))
		}
	} else {
		for i := start; i > stop; i += step {
			result = append(result, Number(i))
		}
	}
	return Array(result...), nil
}

func arrayPush(env *Env, args []Value) (Value, error) {
	if err := requireArgs("array.push", args, 2, 2); err != nil {
		return Null, err
	}
	arr := args[0]
	if arr.typ != TypeArray {
		return Null, fmt.Errorf("array.push: first argument must be an array")
	}
	result := make([]Value, len(arr.aval)+1)
	copy(result, arr.aval)
	result[len(arr.aval)] = args[1]
	return Array(result...), nil
}

func arraySet(env *Env, args []Value) (Value, error) {
	if err := requireArgs("array.set", args, 3, 3); err != nil {
		return Null, err
	}
	arr := args[0]
	if arr.typ != TypeArray {
		return Null, fmt.Errorf("array.set: first argument must be an array")
	}
	idx := int(args[1].ToNumber())
	if idx < 0 || idx >= len(arr.aval) {
		return Null, fmt.Errorf("array.set: index %d out of range", idx)
	}
	result := make([]Value, len(arr.aval))
	copy(result, arr.aval)
	result[idx] = args[2]
	return Array(result...), nil
}

func arrayLength(env *Env, args []Value) (Value, error) {
	if err := requireArgs("array.length", args, 1, 1); err != nil {
		return Null, err
	}
	if args[0].typ != TypeArray {
		return Null, fmt.Errorf("array.length: argument must be an array")
	}
	return Number(float64(len(args[0].aval))), nil
}

// json functions

func jsonDecode(env *Env, args []Value) (Value, error) {
	if err := requireArgs("json.decode", args, 1, 1); err != nil {
		return Null, err
	}
	var raw any
	if err := json.Unmarshal([]byte(args[0].ToString()), &raw); err != nil {
		return Null, fmt.Errorf("json.decode: %w", err)
	}
	return FromInterface(raw), nil
}

func jsonEncode(env *Env, args []Value) (Value, error) {
	if err := requireArgs("json.encode", args, 1, 1); err != nil {
		return Null, err
	}
	b, err := json.Marshal(args[0].ToInterface())
	if err != nil {
		return Null, fmt.Errorf("json.encode: %w", err)
	}
	return String(string(b)), nil
}

// list functions

func listConcat(env *Env, args []Value) (Value, error) {
	if err := requireArgs("list.concat", args, 2, 2); err != nil {
		return Null, err
	}
	return arrayPush(env, args)
}

func listPrepend(env *Env, args []Value) (Value, error) {
	if err := requireArgs("list.prepend", args, 2, 2); err != nil {
		return Null, err
	}
	arr := args[0]
	if arr.typ != TypeArray {
		return Null, fmt.Errorf("list.prepend: first argument must be an array")
	}
	result := make([]Value, len(arr.aval)+1)
	result[0] = args[1]
	copy(result[1:], arr.aval)
	return Array(result...), nil
}

// map functions

func mapDelete(env *Env, args []Value) (Value, error) {
	if err := requireArgs("map.delete", args, 2, 2); err != nil {
		return Null, err
	}
	obj := args[0]
	if obj.typ != TypeObject {
		return Null, fmt.Errorf("map.delete: first argument must be an object")
	}
	key := args[1].ToString()
	result := make(map[string]Value, len(obj.oval))
	for k, v := range obj.oval {
		if k != key {
			result[k] = v
		}
	}
	return Object(result), nil
}

func mapGet(env *Env, args []Value) (Value, error) {
	if err := requireArgs("map.get", args, 2, 2); err != nil {
		return Null, err
	}
	obj := args[0]
	if obj.typ != TypeObject {
		return Null, fmt.Errorf("map.get: first argument must be an object")
	}
	v, ok := obj.oval[args[1].ToString()]
	if !ok {
		return Null, nil
	}
	return v, nil
}

func mapPut(env *Env, args []Value) (Value, error) {
	if err := requireArgs("map.put", args, 3, 3); err != nil {
		return Null, err
	}
	obj := args[0]
	if obj.typ != TypeObject {
		return Null, fmt.Errorf("map.put: first argument must be an object")
	}
	result := make(map[string]Value, len(obj.oval)+1)
	maps.Copy(result, obj.oval)
	result[args[1].ToString()] = args[2]
	return Object(result), nil
}

func mapMerge(env *Env, args []Value) (Value, error) {
	if err := requireArgs("map.merge", args, 2, 2); err != nil {
		return Null, err
	}
	if args[0].typ != TypeObject || args[1].typ != TypeObject {
		return Null, fmt.Errorf("map.merge: both arguments must be objects")
	}
	result := make(map[string]Value, len(args[0].oval)+len(args[1].oval))
	maps.Copy(result, args[0].oval)
	maps.Copy(result, args[1].oval)
	return Object(result), nil
}

func mapMergeNested(env *Env, args []Value) (Value, error) {
	if err := requireArgs("map.mergeNested", args, 2, 2); err != nil {
		return Null, err
	}
	if args[0].typ != TypeObject || args[1].typ != TypeObject {
		return Null, fmt.Errorf("map.mergeNested: both arguments must be objects")
	}
	return mergeDeep(args[0], args[1]), nil
}

func mergeDeep(base, override Value) Value {
	result := make(map[string]Value, len(base.oval)+len(override.oval))
	maps.Copy(result, base.oval)
	for k, v := range override.oval {
		if existing, ok := result[k]; ok && existing.typ == TypeObject && v.typ == TypeObject {
			result[k] = mergeDeep(existing, v)
		} else {
			result[k] = v
		}
	}
	return Object(result)
}

// math functions

func mathCeil(env *Env, args []Value) (Value, error) {
	if err := requireArgs("math.ceil", args, 1, 1); err != nil {
		return Null, err
	}
	return Number(math.Ceil(args[0].ToNumber())), nil
}

func mathSqrt(env *Env, args []Value) (Value, error) {
	if err := requireArgs("math.sqrt", args, 1, 1); err != nil {
		return Null, err
	}
	return Number(math.Sqrt(args[0].ToNumber())), nil
}

func mathAbs(env *Env, args []Value) (Value, error) {
	if err := requireArgs("math.abs", args, 1, 1); err != nil {
		return Null, err
	}
	return Number(math.Abs(args[0].ToNumber())), nil
}

func mathMax(env *Env, args []Value) (Value, error) {
	if err := requireArgs("math.max", args, 2, 2); err != nil {
		return Null, err
	}
	return Number(math.Max(args[0].ToNumber(), args[1].ToNumber())), nil
}

func mathMin(env *Env, args []Value) (Value, error) {
	if err := requireArgs("math.min", args, 2, 2); err != nil {
		return Null, err
	}
	return Number(math.Min(args[0].ToNumber(), args[1].ToNumber())), nil
}

func mathRandint(env *Env, args []Value) (Value, error) {
	if err := requireArgs("math.randint", args, 2, 2); err != nil {
		return Null, err
	}
	lo := int(args[0].ToNumber())
	hi := int(args[1].ToNumber())
	if lo > hi {
		lo, hi = hi, lo
	}
	return Number(float64(lo + rand.IntN(hi-lo+1))), nil
}

// text functions

func textDecode(env *Env, args []Value) (Value, error) {
	if err := requireArgs("text.decode", args, 1, 2); err != nil {
		return Null, err
	}
	return String(args[0].ToString()), nil
}

func textEncode(env *Env, args []Value) (Value, error) {
	if err := requireArgs("text.encode", args, 1, 2); err != nil {
		return Null, err
	}
	return String(args[0].ToString()), nil
}

func textFindAll(env *Env, args []Value) (Value, error) {
	if err := requireArgs("text.findAll", args, 2, 2); err != nil {
		return Null, err
	}
	source := args[0].ToString()
	sub := args[1].ToString()
	var result []Value
	start := 0
	for {
		idx := strings.Index(source[start:], sub)
		if idx < 0 {
			break
		}
		result = append(result, Number(float64(start+idx)))
		start += idx + len(sub)
	}
	return Array(result...), nil
}

func textFindAllRegex(env *Env, args []Value) (Value, error) {
	if err := requireArgs("text.findAllRegex", args, 2, 2); err != nil {
		return Null, err
	}
	re, err := regexp.Compile(args[1].ToString())
	if err != nil {
		return Null, fmt.Errorf("text.findAllRegex: %w", err)
	}
	matches := re.FindAllString(args[0].ToString(), -1)
	result := make([]Value, len(matches))
	for i, m := range matches {
		result[i] = String(m)
	}
	return Array(result...), nil
}

func textMatchRegex(env *Env, args []Value) (Value, error) {
	if err := requireArgs("text.matchRegex", args, 2, 2); err != nil {
		return Null, err
	}
	re, err := regexp.Compile(args[1].ToString())
	if err != nil {
		return Null, fmt.Errorf("text.matchRegex: %w", err)
	}
	return Bool(re.MatchString(args[0].ToString())), nil
}

func textReplaceAll(env *Env, args []Value) (Value, error) {
	if err := requireArgs("text.replaceAll", args, 3, 3); err != nil {
		return Null, err
	}
	return String(strings.ReplaceAll(args[0].ToString(), args[1].ToString(), args[2].ToString())), nil
}

func textReplaceAllRegex(env *Env, args []Value) (Value, error) {
	if err := requireArgs("text.replaceAllRegex", args, 3, 3); err != nil {
		return Null, err
	}
	re, err := regexp.Compile(args[1].ToString())
	if err != nil {
		return Null, fmt.Errorf("text.replaceAllRegex: %w", err)
	}
	return String(re.ReplaceAllString(args[0].ToString(), args[2].ToString())), nil
}

func textSplit(env *Env, args []Value) (Value, error) {
	if err := requireArgs("text.split", args, 2, 2); err != nil {
		return Null, err
	}
	parts := strings.Split(args[0].ToString(), args[1].ToString())
	result := make([]Value, len(parts))
	for i, p := range parts {
		result[i] = String(p)
	}
	return Array(result...), nil
}

func textSubstring(env *Env, args []Value) (Value, error) {
	if err := requireArgs("text.substring", args, 3, 3); err != nil {
		return Null, err
	}
	s := args[0].ToString()
	start := int(args[1].ToNumber())
	end := int(args[2].ToNumber())
	if start < 0 {
		start = 0
	}
	if end > len(s) {
		end = len(s)
	}
	if start >= end {
		return String(""), nil
	}
	return String(s[start:end]), nil
}

func textToLower(env *Env, args []Value) (Value, error) {
	if err := requireArgs("text.toLower", args, 1, 1); err != nil {
		return Null, err
	}
	return String(strings.ToLower(args[0].ToString())), nil
}

func textToUpper(env *Env, args []Value) (Value, error) {
	if err := requireArgs("text.toUpper", args, 1, 1); err != nil {
		return Null, err
	}
	return String(strings.ToUpper(args[0].ToString())), nil
}

func textURLDecode(env *Env, args []Value) (Value, error) {
	if err := requireArgs("text.urlDecode", args, 1, 1); err != nil {
		return Null, err
	}
	s, err := url.QueryUnescape(args[0].ToString())
	if err != nil {
		return Null, fmt.Errorf("text.urlDecode: %w", err)
	}
	return String(s), nil
}

func textURLEncode(env *Env, args []Value) (Value, error) {
	if err := requireArgs("text.urlEncode", args, 1, 1); err != nil {
		return Null, err
	}
	return String(url.PathEscape(args[0].ToString())), nil
}

func textURLEncodePlus(env *Env, args []Value) (Value, error) {
	if err := requireArgs("text.urlEncodePlus", args, 1, 1); err != nil {
		return Null, err
	}
	return String(url.QueryEscape(args[0].ToString())), nil
}

// time functions

func timeFormat(env *Env, args []Value) (Value, error) {
	if err := requireArgs("time.format", args, 1, 2); err != nil {
		return Null, err
	}
	sec := int64(args[0].ToNumber())
	loc := time.UTC
	if len(args) == 2 {
		tz := args[1].ToString()
		l, err := time.LoadLocation(tz)
		if err != nil {
			return Null, fmt.Errorf("time.format: unknown timezone %q", tz)
		}
		loc = l
	}
	t := time.Unix(sec, 0).In(loc)
	return String(t.Format(time.RFC3339)), nil
}

func timeParse(env *Env, args []Value) (Value, error) {
	if err := requireArgs("time.parse", args, 1, 1); err != nil {
		return Null, err
	}
	t, err := time.Parse(time.RFC3339, args[0].ToString())
	if err != nil {
		return Null, fmt.Errorf("time.parse: %w", err)
	}
	return Number(float64(t.Unix())), nil
}

func timeNow(env *Env, args []Value) (Value, error) {
	if err := requireArgs("time.now", args, 0, 0); err != nil {
		return Null, err
	}
	return Number(float64(time.Now().Unix())), nil
}

// uuid functions

func uuidV7(env *Env, args []Value) (Value, error) {
	if err := requireArgs("uuid.v7", args, 0, 0); err != nil {
		return Null, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Null, fmt.Errorf("uuid.v7: %w", err)
	}
	return String(id.String()), nil
}

func uuidNil(env *Env, args []Value) (Value, error) {
	if err := requireArgs("uuid.nil", args, 0, 0); err != nil {
		return Null, err
	}
	return String("00000000-0000-0000-0000-000000000000"), nil
}
