package expr_test

import (
	"math"
	"strings"
	"testing"

	"github.com/sacloud/sakumock/workflows/expr"
)

func TestLiterals(t *testing.T) {
	env := expr.NewEnv()

	tests := []struct {
		input string
		want  expr.Value
	}{
		{"42", expr.Number(42)},
		{"3.14", expr.Number(3.14)},
		{`"hello"`, expr.String("hello")},
		{`'world'`, expr.String("world")},
		{"true", expr.Bool(true)},
		{"false", expr.Bool(false)},
		{"null", expr.Null},
	}
	for _, tt := range tests {
		got, err := expr.Eval(tt.input, env)
		if err != nil {
			t.Errorf("Eval(%q) error: %v", tt.input, err)
			continue
		}
		if !got.StrictEqual(tt.want) {
			t.Errorf("Eval(%q) = %v, want %v", tt.input, got.ToString(), tt.want.ToString())
		}
	}
}

func TestArithmetic(t *testing.T) {
	env := expr.NewEnv()

	tests := []struct {
		input string
		want  float64
	}{
		{"1 + 2", 3},
		{"10 - 3", 7},
		{"4 * 5", 20},
		{"15 / 4", 3.75},
		{"10 % 3", 1},
		{"2 ** 10", 1024},
		{"(1 + 2) * 3", 9},
		{"-5", -5},
		{"+3", 3},
		{"2 + 3 * 4", 14},
		{"(2 + 3) * 4", 20},
	}
	for _, tt := range tests {
		got, err := expr.Eval(tt.input, env)
		if err != nil {
			t.Errorf("Eval(%q) error: %v", tt.input, err)
			continue
		}
		if got.AsNumber() != tt.want {
			t.Errorf("Eval(%q) = %v, want %v", tt.input, got.AsNumber(), tt.want)
		}
	}
}

func TestComparison(t *testing.T) {
	env := expr.NewEnv()

	tests := []struct {
		input string
		want  bool
	}{
		{"1 == 1", true},
		{"1 == 2", false},
		{"1 != 2", true},
		{"1 < 2", true},
		{"2 > 1", true},
		{"1 <= 1", true},
		{"1 >= 2", false},
		{`"a" == "a"`, true},
		{`"a" < "b"`, true},
		{"1 === 1", true},
		{"1 !== 2", true},
	}
	for _, tt := range tests {
		got, err := expr.Eval(tt.input, env)
		if err != nil {
			t.Errorf("Eval(%q) error: %v", tt.input, err)
			continue
		}
		if got.AsBool() != tt.want {
			t.Errorf("Eval(%q) = %v, want %v", tt.input, got.AsBool(), tt.want)
		}
	}
}

func TestLogical(t *testing.T) {
	env := expr.NewEnv()

	tests := []struct {
		input string
		want  bool
	}{
		{"true && true", true},
		{"true && false", false},
		{"false || true", true},
		{"false || false", false},
		{"!true", false},
		{"!false", true},
		{"!0", true},
		{"!1", false},
		{`!""`, true},
		{`!"hello"`, false},
	}
	for _, tt := range tests {
		got, err := expr.Eval(tt.input, env)
		if err != nil {
			t.Errorf("Eval(%q) error: %v", tt.input, err)
			continue
		}
		if got.Truthy() != tt.want {
			t.Errorf("Eval(%q).Truthy() = %v, want %v", tt.input, got.Truthy(), tt.want)
		}
	}
}

func TestStringConcat(t *testing.T) {
	env := expr.NewEnv()
	env.Set("name", expr.String("World"))

	got, err := expr.Eval(`"Hello " + name + "!"`, env)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.AsString() != "Hello World!" {
		t.Errorf("got %q, want %q", got.AsString(), "Hello World!")
	}
}

func TestTernary(t *testing.T) {
	env := expr.NewEnv()

	tests := []struct {
		input string
		want  string
	}{
		{`true ? "yes" : "no"`, "yes"},
		{`false ? "yes" : "no"`, "no"},
		{`1 > 0 ? "positive" : "non-positive"`, "positive"},
	}
	for _, tt := range tests {
		got, err := expr.Eval(tt.input, env)
		if err != nil {
			t.Errorf("Eval(%q) error: %v", tt.input, err)
			continue
		}
		if got.AsString() != tt.want {
			t.Errorf("Eval(%q) = %q, want %q", tt.input, got.AsString(), tt.want)
		}
	}
}

func TestVariables(t *testing.T) {
	env := expr.NewEnv()
	env.Set("x", expr.Number(10))
	env.Set("args", expr.Object(map[string]expr.Value{
		"maxNumber": expr.Number(100),
		"nested": expr.Object(map[string]expr.Value{
			"value": expr.String("deep"),
		}),
	}))

	tests := []struct {
		input string
		want  string
	}{
		{"x", "10"},
		{"args.maxNumber", "100"},
		{"args.nested.value", "deep"},
		{"undefined_var", "null"},
	}
	for _, tt := range tests {
		got, err := expr.Eval(tt.input, env)
		if err != nil {
			t.Errorf("Eval(%q) error: %v", tt.input, err)
			continue
		}
		if got.ToString() != tt.want {
			t.Errorf("Eval(%q) = %q, want %q", tt.input, got.ToString(), tt.want)
		}
	}
}

func TestArrayAccess(t *testing.T) {
	env := expr.NewEnv()
	env.Set("arr", expr.Array(expr.Number(10), expr.Number(20), expr.Number(30)))

	tests := []struct {
		input string
		want  float64
	}{
		{"arr[0]", 10},
		{"arr[1]", 20},
		{"arr[2]", 30},
	}
	for _, tt := range tests {
		got, err := expr.Eval(tt.input, env)
		if err != nil {
			t.Errorf("Eval(%q) error: %v", tt.input, err)
			continue
		}
		if got.AsNumber() != tt.want {
			t.Errorf("Eval(%q) = %v, want %v", tt.input, got.AsNumber(), tt.want)
		}
	}
}

func TestArrayLiteral(t *testing.T) {
	env := expr.NewEnv()

	got, err := expr.Eval("[1, 2, 3]", env)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.Type() != expr.TypeArray {
		t.Fatalf("type = %v, want array", got.Type())
	}
	arr := got.AsArray()
	if len(arr) != 3 {
		t.Fatalf("len = %d, want 3", len(arr))
	}
	if arr[0].AsNumber() != 1 || arr[1].AsNumber() != 2 || arr[2].AsNumber() != 3 {
		t.Errorf("values = %v", arr)
	}
}

func TestObjectLiteral(t *testing.T) {
	env := expr.NewEnv()

	got, err := expr.Eval(`{name: "test", value: 42}`, env)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.Type() != expr.TypeObject {
		t.Fatalf("type = %v, want object", got.Type())
	}
	obj := got.AsObject()
	if obj["name"].AsString() != "test" {
		t.Errorf("name = %q, want test", obj["name"].AsString())
	}
	if obj["value"].AsNumber() != 42 {
		t.Errorf("value = %v, want 42", obj["value"].AsNumber())
	}
}

func TestFunctionCalls(t *testing.T) {
	env := expr.NewEnv()

	tests := []struct {
		name  string
		input string
		check func(t *testing.T, v expr.Value)
	}{
		{
			"array.range(5)",
			"array.range(5)",
			func(t *testing.T, v expr.Value) {
				arr := v.AsArray()
				if len(arr) != 5 {
					t.Errorf("len = %d, want 5", len(arr))
				}
			},
		},
		{
			"array.range(2, 5)",
			"array.range(2, 5)",
			func(t *testing.T, v expr.Value) {
				arr := v.AsArray()
				if len(arr) != 3 || arr[0].AsNumber() != 2 {
					t.Errorf("got %v", arr)
				}
			},
		},
		{
			"array.fill",
			"array.fill(array.range(3), true)",
			func(t *testing.T, v expr.Value) {
				arr := v.AsArray()
				if len(arr) != 3 {
					t.Errorf("len = %d, want 3", len(arr))
				}
				for i, item := range arr {
					if !item.AsBool() {
						t.Errorf("arr[%d] = %v, want true", i, item)
					}
				}
			},
		},
		{
			"array.push",
			"array.push([1, 2], 3)",
			func(t *testing.T, v expr.Value) {
				arr := v.AsArray()
				if len(arr) != 3 || arr[2].AsNumber() != 3 {
					t.Errorf("got %v", arr)
				}
			},
		},
		{
			"array.set",
			"array.set([1, 2, 3], 1, 99)",
			func(t *testing.T, v expr.Value) {
				arr := v.AsArray()
				if arr[1].AsNumber() != 99 {
					t.Errorf("arr[1] = %v, want 99", arr[1])
				}
			},
		},
		{
			"array.length",
			"array.length([1, 2, 3])",
			func(t *testing.T, v expr.Value) {
				if v.AsNumber() != 3 {
					t.Errorf("got %v, want 3", v)
				}
			},
		},
		{
			"math.ceil",
			"math.ceil(4.2)",
			func(t *testing.T, v expr.Value) {
				if v.AsNumber() != 5 {
					t.Errorf("got %v, want 5", v)
				}
			},
		},
		{
			"math.sqrt",
			"math.sqrt(9)",
			func(t *testing.T, v expr.Value) {
				if v.AsNumber() != 3 {
					t.Errorf("got %v, want 3", v)
				}
			},
		},
		{
			"math.abs",
			"math.abs(-7)",
			func(t *testing.T, v expr.Value) {
				if v.AsNumber() != 7 {
					t.Errorf("got %v, want 7", v)
				}
			},
		},
		{
			"math.max",
			"math.max(3, 5)",
			func(t *testing.T, v expr.Value) {
				if v.AsNumber() != 5 {
					t.Errorf("got %v, want 5", v)
				}
			},
		},
		{
			"json.decode + json.encode",
			`json.encode(json.decode('{"a":1}'))`,
			func(t *testing.T, v expr.Value) {
				if v.AsString() != `{"a":1}` {
					t.Errorf("got %q", v.AsString())
				}
			},
		},
		{
			"text.split",
			`text.split("a,b,c", ",")`,
			func(t *testing.T, v expr.Value) {
				arr := v.AsArray()
				if len(arr) != 3 || arr[0].AsString() != "a" {
					t.Errorf("got %v", arr)
				}
			},
		},
		{
			"text.toUpper",
			`text.toUpper("hello")`,
			func(t *testing.T, v expr.Value) {
				if v.AsString() != "HELLO" {
					t.Errorf("got %q", v.AsString())
				}
			},
		},
		{
			"text.replaceAll",
			`text.replaceAll("aabaa", "a", "x")`,
			func(t *testing.T, v expr.Value) {
				if v.AsString() != "xxbxx" {
					t.Errorf("got %q", v.AsString())
				}
			},
		},
		{
			"text.substring",
			`text.substring("hello world", 0, 5)`,
			func(t *testing.T, v expr.Value) {
				if v.AsString() != "hello" {
					t.Errorf("got %q", v.AsString())
				}
			},
		},
		{
			"text.matchRegex",
			`text.matchRegex("hello123", "[0-9]+")`,
			func(t *testing.T, v expr.Value) {
				if !v.AsBool() {
					t.Error("expected true")
				}
			},
		},
		{
			"map.put + map.get",
			`map.get(map.put({}, "key", "val"), "key")`,
			func(t *testing.T, v expr.Value) {
				if v.AsString() != "val" {
					t.Errorf("got %q", v.AsString())
				}
			},
		},
		{
			"map.merge",
			`map.get(map.merge({"a": 1}, {"b": 2}), "b")`,
			func(t *testing.T, v expr.Value) {
				if v.AsNumber() != 2 {
					t.Errorf("got %v", v)
				}
			},
		},
		{
			"uuid.nil",
			`uuid.nil()`,
			func(t *testing.T, v expr.Value) {
				if v.AsString() != "00000000-0000-0000-0000-000000000000" {
					t.Errorf("got %q", v.AsString())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expr.Eval(tt.input, env)
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			tt.check(t, got)
		})
	}
}

func TestEvalInterpolated(t *testing.T) {
	env := expr.NewEnv()
	env.Set("name", expr.String("World"))
	env.Set("x", expr.Number(42))

	tests := []struct {
		input string
		want  string
	}{
		{"no interpolation", "no interpolation"},
		{"${x}", "42"},
		{`${"Hello " + name}`, "Hello World"},
		{"value is ${x}!", "value is 42!"},
		{"${x} + ${x} = ${x + x}", "42 + 42 = 84"},
	}
	for _, tt := range tests {
		got, err := expr.EvalInterpolated(tt.input, env)
		if err != nil {
			t.Errorf("EvalInterpolated(%q) error: %v", tt.input, err)
			continue
		}
		if got.ToString() != tt.want {
			t.Errorf("EvalInterpolated(%q) = %q, want %q", tt.input, got.ToString(), tt.want)
		}
	}
}

func TestSieveExpression(t *testing.T) {
	env := expr.NewEnv()
	env.Set("args", expr.Object(map[string]expr.Value{
		"maxNumber": expr.Number(20),
	}))

	rangeResult, err := expr.Eval("array.range(args.maxNumber)", env)
	if err != nil {
		t.Fatalf("range: %v", err)
	}
	if len(rangeResult.AsArray()) != 20 {
		t.Fatalf("range len = %d, want 20", len(rangeResult.AsArray()))
	}

	fillResult, err := expr.Eval("array.fill(array.range(args.maxNumber), true)", env)
	if err != nil {
		t.Fatalf("fill: %v", err)
	}
	if len(fillResult.AsArray()) != 20 {
		t.Fatalf("fill len = %d, want 20", len(fillResult.AsArray()))
	}

	ceilSqrt, err := expr.Eval("math.ceil(math.sqrt(args.maxNumber))", env)
	if err != nil {
		t.Fatalf("ceil(sqrt): %v", err)
	}
	expected := math.Ceil(math.Sqrt(20))
	if ceilSqrt.AsNumber() != expected {
		t.Errorf("ceil(sqrt(20)) = %v, want %v", ceilSqrt.AsNumber(), expected)
	}
}

func TestLooseEquality(t *testing.T) {
	env := expr.NewEnv()

	tests := []struct {
		input string
		want  bool
	}{
		{"null == null", true},
		{"1 == true", true},
		{"0 == false", true},
		{`1 == "1"`, true},
		{"null == false", false},
		{"1 === true", false},
		{`1 === "1"`, false},
	}
	for _, tt := range tests {
		got, err := expr.Eval(tt.input, env)
		if err != nil {
			t.Errorf("Eval(%q) error: %v", tt.input, err)
			continue
		}
		if got.AsBool() != tt.want {
			t.Errorf("Eval(%q) = %v, want %v", tt.input, got.AsBool(), tt.want)
		}
	}
}

func TestStringEscape(t *testing.T) {
	env := expr.NewEnv()

	got, err := expr.Eval(`"line1\nline2\ttab"`, env)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	want := "line1\nline2\ttab"
	if got.AsString() != want {
		t.Errorf("got %q, want %q", got.AsString(), want)
	}
}

func TestNestedFunctionCalls(t *testing.T) {
	env := expr.NewEnv()

	got, err := expr.Eval("array.length(array.push(array.push([1], 2), 3))", env)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.AsNumber() != 3 {
		t.Errorf("got %v, want 3", got.AsNumber())
	}
}

func TestDivisionByZero(t *testing.T) {
	env := expr.NewEnv()

	got, err := expr.Eval("1 / 0", env)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !math.IsInf(got.AsNumber(), 1) {
		t.Errorf("1/0 = %v, want +Inf", got.AsNumber())
	}

	got, err = expr.Eval("0 / 0", env)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !math.IsNaN(got.AsNumber()) {
		t.Errorf("0/0 = %v, want NaN", got.AsNumber())
	}
}

func TestSafetyLimits(t *testing.T) {
	t.Run("step limit", func(t *testing.T) {
		env := expr.NewEnv()
		env.SetMaxSteps(10)
		_, err := expr.Eval("1+1+1+1+1+1+1+1+1+1+1+1+1+1+1+1+1+1+1+1", env)
		if err == nil {
			t.Fatal("expected step limit error")
		}
		if !strings.Contains(err.Error(), "exceeded") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("array size limit", func(t *testing.T) {
		env := expr.NewEnv()
		env.MaxArrayLen = 100
		_, err := expr.Eval("array.range(1000)", env)
		if err == nil {
			t.Fatal("expected array size limit error")
		}
		if !strings.Contains(err.Error(), "exceeds limit") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("parse depth limit", func(t *testing.T) {
		env := expr.NewEnv()
		deep := strings.Repeat("(", 200) + "1" + strings.Repeat(")", 200)
		_, err := expr.Eval(deep, env)
		if err == nil {
			t.Fatal("expected parse depth error")
		}
		if !strings.Contains(err.Error(), "deeply nested") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestEnvClone(t *testing.T) {
	env := expr.NewEnv()
	env.Set("x", expr.Number(1))

	cloned := env.Clone()
	cloned.Set("x", expr.Number(2))

	orig, _ := env.Get("x")
	if orig.AsNumber() != 1 {
		t.Errorf("original x = %v, want 1", orig.AsNumber())
	}
	c, _ := cloned.Get("x")
	if c.AsNumber() != 2 {
		t.Errorf("cloned x = %v, want 2", c.AsNumber())
	}
}
