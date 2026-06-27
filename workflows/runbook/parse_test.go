package runbook_test

import (
	"context"
	"testing"

	"github.com/sacloud/sakumock/workflows/expr"
	"github.com/sacloud/sakumock/workflows/runbook"
)

func TestParseBasic(t *testing.T) {
	yaml := `
meta:
  description: basic test
args:
  x:
    type: number
    description: input number
steps:
  calc:
    assign:
      result: ${args.x * 2}
  done:
    return: ${result}
`
	rb, err := runbook.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if rb.Meta.Description != "basic test" {
		t.Errorf("meta.description = %q", rb.Meta.Description)
	}
	if rb.Args["x"].Type != "number" {
		t.Errorf("args.x.type = %q", rb.Args["x"].Type)
	}
	if len(rb.Steps) != 2 {
		t.Fatalf("steps count = %d, want 2", len(rb.Steps))
	}
	if rb.Steps[0].Name != "calc" || rb.Steps[1].Name != "done" {
		t.Errorf("step names = %q, %q", rb.Steps[0].Name, rb.Steps[1].Name)
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, map[string]expr.Value{
		"x": expr.Number(21),
	})
	if result.Err != nil {
		t.Fatalf("run: %v", result.Err)
	}
	if result.Value.AsNumber() != 42 {
		t.Errorf("result = %v, want 42", result.Value.AsNumber())
	}
}

func TestParseSwitch(t *testing.T) {
	yaml := `
steps:
  check:
    switch:
      - condition: ${args.x > 0}
        return: ${"positive"}
      - condition: ${true}
        return: ${"non-positive"}
`
	rb, err := runbook.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, map[string]expr.Value{
		"x": expr.Number(5),
	})
	if result.Err != nil {
		t.Fatalf("run: %v", result.Err)
	}
	if result.Value.AsString() != "positive" {
		t.Errorf("got %q, want positive", result.Value.AsString())
	}
}

func TestParseFor(t *testing.T) {
	yaml := `
steps:
  init:
    assign:
      sum: ${0}
  loop:
    for:
      in: ${[1, 2, 3, 4, 5]}
      as: i
      steps:
        add:
          assign:
            sum: ${sum + i}
  done:
    return: ${sum}
`
	rb, err := runbook.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, nil)
	if result.Err != nil {
		t.Fatalf("run: %v", result.Err)
	}
	if result.Value.AsNumber() != 15 {
		t.Errorf("got %v, want 15", result.Value.AsNumber())
	}
}

func TestParseTryExcept(t *testing.T) {
	yaml := `
steps:
  attempt:
    try:
      assign:
        x: ${json.decode("invalid")}
    except:
      as: err
      return: ${err.message}
`
	rb, err := runbook.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, nil)
	if result.Err != nil {
		t.Fatalf("run: %v", result.Err)
	}
	if result.Value.Type() != expr.TypeString || result.Value.AsString() == "" {
		t.Errorf("expected error message, got %v", result.Value)
	}
}

func TestParseCall(t *testing.T) {
	yaml := `
steps:
  greet:
    call: http.get
    args:
      url: ${args.url}
    result: resp
  done:
    return: ${resp.status}
`
	rb, err := runbook.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if rb.Steps[0].Step.Call == nil {
		t.Fatal("expected call step")
	}
	if rb.Steps[0].Step.Call.Func != "http.get" {
		t.Errorf("call func = %q", rb.Steps[0].Step.Call.Func)
	}
	if rb.Steps[0].Step.Call.Result != "resp" {
		t.Errorf("call result = %q", rb.Steps[0].Step.Call.Result)
	}
}

func TestParseNext(t *testing.T) {
	yaml := `
steps:
  start:
    assign:
      x: ${1}
    next: done
  skipped:
    assign:
      x: ${999}
  done:
    return: ${x}
`
	rb, err := runbook.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, nil)
	if result.Err != nil {
		t.Fatalf("run: %v", result.Err)
	}
	if result.Value.AsNumber() != 1 {
		t.Errorf("got %v, want 1", result.Value.AsNumber())
	}
}

func TestParseEmptyStep(t *testing.T) {
	yaml := `
steps:
  noop:
  done:
    return: ${"ok"}
`
	rb, err := runbook.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, nil)
	if result.Err != nil {
		t.Fatalf("run: %v", result.Err)
	}
	if result.Value.AsString() != "ok" {
		t.Errorf("got %q, want ok", result.Value.AsString())
	}
}

func TestParseSieveEndToEnd(t *testing.T) {
	yaml := `
meta:
  description: エラトステネスの篩
args:
  maxNumber:
    type: number
    description: 素数を求める最大の数
steps:
  setup:
    assign:
      sieve: ${array.fill(array.range(args.maxNumber), true)}
      primes: []
  initial:
    assign:
      _a: ${array.set(sieve, 0, false)}
      _b: ${array.set(sieve, 1, false)}
  loop:
    for:
      in: ${array.range(2, math.ceil(math.sqrt(args.maxNumber)))}
      as: index
      steps:
        if:
          switch:
            - condition: ${sieve[index] == false}
              next: continue
            - condition: ${sieve[index] != false}
              steps:
                updateSieve:
                  for:
                    in: ${array.range(index * 2, args.maxNumber, index)}
                    as: n
                    steps:
                      set:
                        assign:
                          sieve: ${array.set(sieve, n, false)}
        continue:
  printPrimes:
    for:
      in: ${array.range(2, args.maxNumber)}
      as: index
      steps:
        if:
          switch:
            - condition: ${sieve[index] == true}
              steps:
                push:
                  assign:
                    primes: ${array.push(primes, index)}
  done:
    return: ${primes}
`
	rb, err := runbook.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, map[string]expr.Value{
		"maxNumber": expr.Number(30),
	})
	if result.Err != nil {
		t.Fatalf("run: %v", result.Err)
	}

	primes := result.Value.AsArray()
	expected := []float64{2, 3, 5, 7, 11, 13, 17, 19, 23, 29}
	if len(primes) != len(expected) {
		t.Fatalf("got %d primes, want %d", len(primes), len(expected))
	}
	for i, p := range primes {
		if p.AsNumber() != expected[i] {
			t.Errorf("prime[%d] = %v, want %v", i, p.AsNumber(), expected[i])
		}
	}
}

func TestParseAddressLookup(t *testing.T) {
	yaml := `
meta:
  description: 住所検索
args:
  address:
    type: string
    description: 住所を取得するための郵便番号
steps:
  set:
    switch:
      - condition: ${args.address}
        steps:
          x:
            assign:
              address: ${args.address}
      - condition: ${!args.address}
        steps:
          x:
            assign:
              address: "1000001"
  done:
    return: ${address}
`
	rb, err := runbook.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, map[string]expr.Value{
		"address": expr.String("1500001"),
	})
	if result.Err != nil {
		t.Fatalf("run: %v", result.Err)
	}
	if result.Value.AsString() != "1500001" {
		t.Errorf("got %q, want 1500001", result.Value.AsString())
	}

	result = r.Run(context.Background(), rb, map[string]expr.Value{
		"address": expr.String(""),
	})
	if result.Err != nil {
		t.Fatalf("run default: %v", result.Err)
	}
	if result.Value.AsString() != "1000001" {
		t.Errorf("got %q, want 1000001", result.Value.AsString())
	}
}
