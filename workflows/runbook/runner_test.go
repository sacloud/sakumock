package runbook_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sacloud/sakumock/workflows/expr"
	"github.com/sacloud/sakumock/workflows/runbook"
)

func ptr(s string) *string { return &s }

func TestAssignAndReturn(t *testing.T) {
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "setup", Step: runbook.Step{
				Assign: []runbook.Assignment{
					{Name: "x", Expression: "${1 + 2}"},
					{Name: "y", Expression: "${x * 10}"},
				},
			}},
			{Name: "done", Step: runbook.Step{
				Return: ptr("${x + y}"),
			}},
		},
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, nil)
	if result.Err != nil {
		t.Fatalf("error: %v", result.Err)
	}
	if result.Value.AsNumber() != 33 {
		t.Errorf("got %v, want 33", result.Value.AsNumber())
	}
}

func TestArgs(t *testing.T) {
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "done", Step: runbook.Step{
				Return: ptr("${args.a + args.b}"),
			}},
		},
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, map[string]expr.Value{
		"a": expr.Number(10),
		"b": expr.Number(20),
	})
	if result.Err != nil {
		t.Fatalf("error: %v", result.Err)
	}
	if result.Value.AsNumber() != 30 {
		t.Errorf("got %v, want 30", result.Value.AsNumber())
	}
}

func TestSwitch(t *testing.T) {
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "check", Step: runbook.Step{
				Switch: []runbook.SwitchCase{
					{Condition: "${args.x > 0}", Return: ptr(`${"positive"}`)},
					{Condition: "${args.x == 0}", Return: ptr(`${"zero"}`)},
					{Condition: "${true}", Return: ptr(`${"negative"}`)},
				},
			}},
		},
	}

	r := runbook.NewRunner()

	tests := []struct {
		x    float64
		want string
	}{
		{5, "positive"},
		{0, "zero"},
		{-3, "negative"},
	}
	for _, tt := range tests {
		result := r.Run(context.Background(), rb, map[string]expr.Value{
			"x": expr.Number(tt.x),
		})
		if result.Err != nil {
			t.Fatalf("x=%v: error: %v", tt.x, result.Err)
		}
		if result.Value.AsString() != tt.want {
			t.Errorf("x=%v: got %q, want %q", tt.x, result.Value.AsString(), tt.want)
		}
	}
}

func TestForLoop(t *testing.T) {
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "init", Step: runbook.Step{
				Assign: []runbook.Assignment{
					{Name: "sum", Expression: "${0}"},
				},
			}},
			{Name: "loop", Step: runbook.Step{
				For: &runbook.ForStep{
					In: "${array.range(1, 6)}",
					As: "i",
					Steps: []runbook.NamedStep{
						{Name: "add", Step: runbook.Step{
							Assign: []runbook.Assignment{
								{Name: "sum", Expression: "${sum + i}"},
							},
						}},
					},
				},
			}},
			{Name: "done", Step: runbook.Step{
				Return: ptr("${sum}"),
			}},
		},
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, nil)
	if result.Err != nil {
		t.Fatalf("error: %v", result.Err)
	}
	if result.Value.AsNumber() != 15 {
		t.Errorf("got %v, want 15 (1+2+3+4+5)", result.Value.AsNumber())
	}
}

func TestNext(t *testing.T) {
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "start", Step: runbook.Step{
				Assign: []runbook.Assignment{
					{Name: "x", Expression: "${1}"},
				},
				Next: "done",
			}},
			{Name: "skipped", Step: runbook.Step{
				Assign: []runbook.Assignment{
					{Name: "x", Expression: "${999}"},
				},
			}},
			{Name: "done", Step: runbook.Step{
				Return: ptr("${x}"),
			}},
		},
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, nil)
	if result.Err != nil {
		t.Fatalf("error: %v", result.Err)
	}
	if result.Value.AsNumber() != 1 {
		t.Errorf("got %v, want 1 (skipped step should not run)", result.Value.AsNumber())
	}
}

func TestSwitchWithNext(t *testing.T) {
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "init", Step: runbook.Step{
				Assign: []runbook.Assignment{
					{Name: "sum", Expression: "${0}"},
				},
			}},
			{Name: "loop", Step: runbook.Step{
				For: &runbook.ForStep{
					In: "${[1, 2, 3, 4, 5]}",
					As: "i",
					Steps: []runbook.NamedStep{
						{Name: "filter", Step: runbook.Step{
							Switch: []runbook.SwitchCase{
								{Condition: "${i % 2 == 0}", Next: "skip"},
							},
						}},
						{Name: "add", Step: runbook.Step{
							Assign: []runbook.Assignment{
								{Name: "sum", Expression: "${sum + i}"},
							},
						}},
						{Name: "skip"},
					},
				},
			}},
			{Name: "done", Step: runbook.Step{
				Return: ptr("${sum}"),
			}},
		},
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, nil)
	if result.Err != nil {
		t.Fatalf("error: %v", result.Err)
	}
	if result.Value.AsNumber() != 9 {
		t.Errorf("got %v, want 9 (1+3+5)", result.Value.AsNumber())
	}
}

func TestTryExcept(t *testing.T) {
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "attempt", Step: runbook.Step{
				Try: &runbook.TryStep{
					Steps: []runbook.NamedStep{
						{Name: "fail", Step: runbook.Step{
							Assign: []runbook.Assignment{
								{Name: "x", Expression: `${json.decode("not json")}`},
							},
						}},
					},
					ExceptAs:     "err",
					ExceptReturn: ptr("${err.message}"),
				},
			}},
		},
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, nil)
	if result.Err != nil {
		t.Fatalf("error: %v", result.Err)
	}
	if result.Value.Type() != expr.TypeString || result.Value.AsString() == "" {
		t.Errorf("expected error message, got %v", result.Value)
	}
}

func TestTryExceptWithSteps(t *testing.T) {
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "attempt", Step: runbook.Step{
				Try: &runbook.TryStep{
					Steps: []runbook.NamedStep{
						{Name: "fail", Step: runbook.Step{
							Assign: []runbook.Assignment{
								{Name: "x", Expression: `${json.decode("bad")}`},
							},
						}},
					},
					ExceptAs: "err",
					ExceptSteps: []runbook.NamedStep{
						{Name: "handle", Step: runbook.Step{
							Assign: []runbook.Assignment{
								{Name: "recovered", Expression: "${true}"},
							},
						}},
					},
				},
			}},
			{Name: "done", Step: runbook.Step{
				Return: ptr("${recovered}"),
			}},
		},
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, nil)
	if result.Err != nil {
		t.Fatalf("error: %v", result.Err)
	}
	if !result.Value.AsBool() {
		t.Errorf("expected true, got %v", result.Value)
	}
}

func TestParallelBranches(t *testing.T) {
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "par", Step: runbook.Step{
				Parallel: &runbook.ParallelStep{
					Branches: []runbook.Branch{
						{Name: "a", Steps: []runbook.NamedStep{
							{Name: "ret", Step: runbook.Step{Return: ptr("${10}")}},
						}},
						{Name: "b", Steps: []runbook.NamedStep{
							{Name: "ret", Step: runbook.Step{Return: ptr("${20}")}},
						}},
					},
				},
			}},
			{Name: "done", Step: runbook.Step{
				Return: ptr("${results[0] + results[1]}"),
			}},
		},
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, nil)
	if result.Err != nil {
		t.Fatalf("error: %v", result.Err)
	}
	if result.Value.AsNumber() != 30 {
		t.Errorf("got %v, want 30", result.Value.AsNumber())
	}
}

func TestParallelIteration(t *testing.T) {
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "par", Step: runbook.Step{
				Parallel: &runbook.ParallelStep{
					In: "${[1, 2, 3]}",
					As: "item",
					Steps: []runbook.NamedStep{
						{Name: "calc", Step: runbook.Step{
							Return: ptr("${item * 10}"),
						}},
					},
				},
			}},
			{Name: "done", Step: runbook.Step{
				Return: ptr("${results[0] + results[1] + results[2]}"),
			}},
		},
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, nil)
	if result.Err != nil {
		t.Fatalf("error: %v", result.Err)
	}
	if result.Value.AsNumber() != 60 {
		t.Errorf("got %v, want 60 (10+20+30)", result.Value.AsNumber())
	}
}

func TestCallHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"msg": "hello"})
	}))
	defer srv.Close()

	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "get", Step: runbook.Step{
				Call: &runbook.CallStep{
					Func:   "http.get",
					Args:   map[string]string{"url": "${args.url}"},
					Result: "resp",
				},
			}},
			{Name: "done", Step: runbook.Step{
				Return: ptr("${resp.status}"),
			}},
		},
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, map[string]expr.Value{
		"url": expr.String(srv.URL),
	})
	if result.Err != nil {
		t.Fatalf("error: %v", result.Err)
	}
	if result.Value.AsNumber() != 200 {
		t.Errorf("got %v, want 200", result.Value.AsNumber())
	}
}

func TestHTTPBlocksLocalhost(t *testing.T) {
	blockedURLs := []string{
		"http://localhost/secret",
		"http://127.0.0.1/secret",
		"http://[::1]/secret",
		"http://169.254.169.254/latest/meta-data",
	}

	for _, u := range blockedURLs {
		rb := &runbook.Runbook{
			Steps: []runbook.NamedStep{
				{Name: "get", Step: runbook.Step{
					Call: &runbook.CallStep{
						Func:   "http.get",
						Args:   map[string]string{"url": u},
						Result: "resp",
					},
				}},
			},
		}

		r := runbook.NewRunner()
		r.AllowLocalNet = false
		result := r.Run(context.Background(), rb, nil)
		if result.Err == nil {
			t.Errorf("expected error for blocked URL %s", u)
		}
	}
}

func TestHTTPRejectsNonHTTPScheme(t *testing.T) {
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "get", Step: runbook.Step{
				Call: &runbook.CallStep{
					Func:   "http.get",
					Args:   map[string]string{"url": "file:///etc/passwd"},
					Result: "resp",
				},
			}},
		},
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, nil)
	if result.Err == nil {
		t.Fatal("expected error for file:// scheme")
	}
}

func TestNestedForInSwitch(t *testing.T) {
	// for each row, switch on even/odd, inner for accumulates
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "init", Step: runbook.Step{
				Assign: []runbook.Assignment{
					{Name: "evens", Expression: "${0}"},
					{Name: "odds", Expression: "${0}"},
				},
			}},
			{Name: "outer", Step: runbook.Step{
				For: &runbook.ForStep{
					In: "${array.range(1, 11)}",
					As: "n",
					Steps: []runbook.NamedStep{
						{Name: "classify", Step: runbook.Step{
							Switch: []runbook.SwitchCase{
								{Condition: "${n % 2 == 0}", Steps: []runbook.NamedStep{
									{Name: "add_even", Step: runbook.Step{
										Assign: []runbook.Assignment{
											{Name: "evens", Expression: "${evens + n}"},
										},
									}},
								}},
								{Condition: "${true}", Steps: []runbook.NamedStep{
									{Name: "add_odd", Step: runbook.Step{
										Assign: []runbook.Assignment{
											{Name: "odds", Expression: "${odds + n}"},
										},
									}},
								}},
							},
						}},
					},
				},
			}},
			{Name: "done", Step: runbook.Step{
				Return: ptr("${evens + odds}"),
			}},
		},
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, nil)
	if result.Err != nil {
		t.Fatalf("error: %v", result.Err)
	}
	// 1+2+...+10 = 55
	if result.Value.AsNumber() != 55 {
		t.Errorf("got %v, want 55", result.Value.AsNumber())
	}
}

func TestNestedForInFor(t *testing.T) {
	// matrix multiplication-style: sum of i*j for i in 1..3, j in 1..3
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "init", Step: runbook.Step{
				Assign: []runbook.Assignment{
					{Name: "total", Expression: "${0}"},
				},
			}},
			{Name: "outer", Step: runbook.Step{
				For: &runbook.ForStep{
					In: "${[1, 2, 3]}",
					As: "i",
					Steps: []runbook.NamedStep{
						{Name: "inner", Step: runbook.Step{
							For: &runbook.ForStep{
								In: "${[1, 2, 3]}",
								As: "j",
								Steps: []runbook.NamedStep{
									{Name: "mul", Step: runbook.Step{
										Assign: []runbook.Assignment{
											{Name: "total", Expression: "${total + i * j}"},
										},
									}},
								},
							},
						}},
					},
				},
			}},
			{Name: "done", Step: runbook.Step{
				Return: ptr("${total}"),
			}},
		},
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, nil)
	if result.Err != nil {
		t.Fatalf("error: %v", result.Err)
	}
	// (1+2+3)*(1+2+3) = 36
	if result.Value.AsNumber() != 36 {
		t.Errorf("got %v, want 36", result.Value.AsNumber())
	}
}

func TestNestedTryInFor(t *testing.T) {
	// for loop with try-except: bad items are caught, good items accumulated
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "init", Step: runbook.Step{
				Assign: []runbook.Assignment{
					{Name: "results", Expression: "${[]}"},
				},
			}},
			{Name: "loop", Step: runbook.Step{
				For: &runbook.ForStep{
					In:    `${["1", "bad", "3"]}`,
					As: "item",
					Steps: []runbook.NamedStep{
						{Name: "attempt", Step: runbook.Step{
							Try: &runbook.TryStep{
								Steps: []runbook.NamedStep{
									{Name: "parse", Step: runbook.Step{
										Assign: []runbook.Assignment{
											{Name: "parsed", Expression: `${json.decode(item)}`},
											{Name: "results", Expression: "${array.push(results, parsed)}"},
										},
									}},
								},
								ExceptAs: "err",
								ExceptSteps: []runbook.NamedStep{
									{Name: "handle", Step: runbook.Step{
										Assign: []runbook.Assignment{
											{Name: "results", Expression: `${array.push(results, -1)}`},
										},
									}},
								},
							},
						}},
					},
				},
			}},
			{Name: "done", Step: runbook.Step{
				Return: ptr("${results}"),
			}},
		},
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, nil)
	if result.Err != nil {
		t.Fatalf("error: %v", result.Err)
	}
	arr := result.Value.AsArray()
	if len(arr) != 3 {
		t.Fatalf("got %d items, want 3", len(arr))
	}
	if arr[0].AsNumber() != 1 || arr[1].AsNumber() != -1 || arr[2].AsNumber() != 3 {
		t.Errorf("got [%v, %v, %v], want [1, -1, 3]", arr[0], arr[1], arr[2])
	}
}

func TestContextCancellation(t *testing.T) {
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "loop", Step: runbook.Step{
				For: &runbook.ForStep{
					In: "${array.range(1000000)}",
					As: "i",
					Steps: []runbook.NamedStep{
						{Name: "noop", Step: runbook.Step{
							Assign: []runbook.Assignment{
								{Name: "x", Expression: "${i}"},
							},
						}},
					},
				},
			}},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := runbook.NewRunner()
	result := r.Run(ctx, rb, nil)
	if result.Err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestSieveRunbook(t *testing.T) {
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "setup", Step: runbook.Step{
				Assign: []runbook.Assignment{
					{Name: "sieve", Expression: "${array.fill(array.range(args.maxNumber), true)}"},
					{Name: "primes", Expression: "${[]}"},
				},
			}},
			{Name: "initial", Step: runbook.Step{
				Assign: []runbook.Assignment{
					{Name: "_a", Expression: "${array.set(sieve, 0, false)}"},
					{Name: "_b", Expression: "${array.set(sieve, 1, false)}"},
				},
			}},
			{Name: "loop", Step: runbook.Step{
				For: &runbook.ForStep{
					In: "${array.range(2, math.ceil(math.sqrt(args.maxNumber)))}",
					As: "index",
					Steps: []runbook.NamedStep{
						{Name: "if", Step: runbook.Step{
							Switch: []runbook.SwitchCase{
								{Condition: "${sieve[index] == false}", Next: "continue"},
								{Condition: "${sieve[index] != false}", Steps: []runbook.NamedStep{
									{Name: "updateSieve", Step: runbook.Step{
										For: &runbook.ForStep{
											In: "${array.range(index * 2, args.maxNumber, index)}",
											As: "n",
											Steps: []runbook.NamedStep{
												{Name: "set", Step: runbook.Step{
													Assign: []runbook.Assignment{
														{Name: "sieve", Expression: "${array.set(sieve, n, false)}"},
													},
												}},
											},
										},
									}},
								}},
							},
						}},
						{Name: "continue"},
					},
				},
			}},
			{Name: "printPrimes", Step: runbook.Step{
				For: &runbook.ForStep{
					In: "${array.range(2, args.maxNumber)}",
					As: "index",
					Steps: []runbook.NamedStep{
						{Name: "if", Step: runbook.Step{
							Switch: []runbook.SwitchCase{
								{Condition: "${sieve[index] == true}", Steps: []runbook.NamedStep{
									{Name: "push", Step: runbook.Step{
										Assign: []runbook.Assignment{
											{Name: "primes", Expression: "${array.push(primes, index)}"},
										},
									}},
								}},
							},
						}},
					},
				},
			}},
			{Name: "done", Step: runbook.Step{
				Return: ptr("${primes}"),
			}},
		},
	}

	r := runbook.NewRunner()
	result := r.Run(context.Background(), rb, map[string]expr.Value{
		"maxNumber": expr.Number(30),
	})
	if result.Err != nil {
		t.Fatalf("error: %v", result.Err)
	}
	if result.Value.Type() != expr.TypeArray {
		t.Fatalf("expected array, got %s", result.Value.Type())
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
