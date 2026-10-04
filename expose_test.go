package chromedp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	jsonv2 "encoding/json/v2"
	"github.com/chromedp/cdproto/runtime"
)

// awaitPromise makes Evaluate wait for the promise that the expression gives.
func awaitPromise(p *runtime.EvaluateParams) {
	p.AwaitPromise = new(true)
}

func TestExposeFunc(t *testing.T) {
	t.Parallel()

	type user struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	type greeting struct {
		Text string `json:"text"`
		Age  int    `json:"age"`
	}

	ctx, cancel := testAllocate(t, "expose2.html")
	defer cancel()

	var calls atomic.Int64
	if err := Do(ctx,
		ExposeFunc("square", func(ctx context.Context, n float64) (float64, error) {
			calls.Add(1)
			return n * n, nil
		}),
		ExposeFunc("greet", func(ctx context.Context, u user) (greeting, error) {
			return greeting{Text: "hello " + u.Name, Age: u.Age + 1}, nil
		}),
		ExposeFunc("sum", func(ctx context.Context, args []float64) (float64, error) {
			var sum float64
			for _, a := range args {
				sum += a
			}
			return sum, nil
		}),
		ExposeFunc("count", func(ctx context.Context, args []any) (int, error) {
			return len(args), nil
		}),
		ExposeFunc("names", func(ctx context.Context, names []string) ([]string, error) {
			return append(names, "end"), nil
		}),
		ExposeFunc("first", func(ctx context.Context, args []any) (any, error) {
			return args[0], nil
		}),
		ExposeFunc("fail", func(ctx context.Context, msg string) (Void, error) {
			return Void{}, errors.New("failed: " + msg)
		}),
		ExposeFunc("boom", func(ctx context.Context, _ Void) (Void, error) {
			panic("exploded")
		}),
		ExposeFunc("nothing", func(ctx context.Context, _ Void) (Void, error) {
			return Void{}, nil
		}),
	); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		expr string
		want any
	}{
		{"number", `square(7)`, 49.0},
		{"object", `greet({name: "Ann", age: 3}).then(g => g.text + ":" + g.age)`, "hello Ann:4"},
		{"several arguments", `sum(1, 2, 3.5)`, 6.5},
		{"several arguments of any type", `count(1, "a", {b: 2}, [3])`, 4.0},
		{"no arguments", `count()`, 0.0},
		{"array of arguments", `names("a", "b").then(n => n.join(","))`, "a,b,end"},
		{"array as one argument", `first(["a", "b"], 1).then(n => n.join(","))`, "a,b"},
		{"Go error", `fail("x").then(() => "resolved", e => e instanceof Error && e.message)`, "failed: x"},
		{"panic", `boom().then(() => "resolved", e => e.message)`, "panic: exploded"},
		{"no result", `nothing().then(v => String(v))`, "undefined"},
		{"two arguments for one value", `square(1, 2).then(() => "resolved", e => e.message)`, "expected one argument: "},
		{"wrong type", `square("a").then(() => "resolved", e => e.message.startsWith("decoding the argument"))`, true},
		{"parallel", `Promise.all([1, 2, 3, 4, 5, 6].map(n => square(n))).then(a => a.join(","))`, "1,4,9,16,25,36"},
		{"is a function", `typeof square`, "function"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Run(ctx, Evaluate[any](test.expr, awaitPromise))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if s, ok := test.want.(string); ok && strings.HasSuffix(s, ": ") {
				if g, _ := got.(string); !strings.HasPrefix(g, "expected one argument") {
					t.Fatalf("expected an error about the arguments, got: %v", got)
				}
				return
			}
			if got != test.want {
				t.Fatalf("expected %v, got: %v", test.want, got)
			}
		})
	}
	if n := calls.Load(); n != 7 {
		t.Errorf("expected 7 calls of square, got: %d", n)
	}
}

func TestExposeFuncSurvivesNavigation(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "expose2.html")
	defer cancel()

	if err := Do(ctx, ExposeFunc("square", func(ctx context.Context, n float64) (float64, error) {
		return n * n, nil
	})); err != nil {
		t.Fatal(err)
	}
	for i, page := range []string{"expose2.html", "form.html", "expose2.html"} {
		if err := Do(ctx, Navigate(testdataDir+"/"+page)); err != nil {
			t.Fatal(err)
		}
		got, err := Run(ctx, Evaluate[float64](`square(3)`, awaitPromise))
		if err != nil {
			t.Fatalf("navigation %d: got error: %v", i, err)
		}
		if got != 9 {
			t.Fatalf("navigation %d: expected 9, got: %v", i, got)
		}
	}

	// A call that is in flight when the page goes away does not break the next
	// page.
	if err := Do(ctx, ExposeFunc("slow", func(ctx context.Context, _ Void) (Void, error) {
		return Void{}, ctx.Err()
	})); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ctx, Evaluate[Void](`slow(); location.href = "form.html"`)); err != nil {
		t.Fatal(err)
	}
	if err := Do(ctx, WaitReady(CSS("form"))); err != nil {
		t.Fatal(err)
	}
	if got, err := Run(ctx, Evaluate[float64](`square(4)`, awaitPromise)); err != nil || got != 16 {
		t.Fatalf("expected 16, got: %v (%v)", got, err)
	}
}

func TestExposeFuncIframe(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.FileServer(http.Dir("testdata")))
	// Cleanup waits for the parallel subtests, and a defer does not.
	t.Cleanup(ts.Close)

	square := ExposeFunc("square", func(ctx context.Context, n float64) (float64, error) {
		return n * n, nil
	})
	const frameResult = `document.getElementById("child").contentWindow.result`
	const waitResult = `new Promise(function (resolve) {
		(function poll() {
			var r = ` + frameResult + `;
			if (r !== null) { resolve(r); } else { setTimeout(poll, 10); }
		})();
	})`

	t.Run("loaded later", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "")
		defer cancel()
		// The function is there before the page loads, and the script of
		// the iframe calls it when the iframe loads.
		if err := Do(ctx, square, Navigate(ts.URL+"/expose.html")); err != nil {
			t.Fatal(err)
		}
		got, err := Run(ctx, Evaluate[float64](waitResult, awaitPromise))
		if err != nil {
			t.Fatal(err)
		}
		if got != 25 {
			t.Fatalf("expected 25, got: %v", got)
		}
	})

	t.Run("open already", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "")
		defer cancel()
		if err := Do(ctx, Navigate(ts.URL+"/expose.html"), WaitReady(CSS("iframe"))); err != nil {
			t.Fatal(err)
		}
		if err := Do(ctx, square); err != nil {
			t.Fatal(err)
		}
		got, err := Run(ctx, Evaluate[float64](`document.getElementById("child").contentWindow.square(6)`, awaitPromise))
		if err != nil {
			t.Fatal(err)
		}
		if got != 36 {
			t.Fatalf("expected 36, got: %v", got)
		}
	})
}

func TestExposeFuncErrors(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "expose2.html")
	defer cancel()

	fn := func(ctx context.Context, _ Void) (Void, error) { return Void{}, nil }
	if err := Do(ctx, ExposeFunc("once", fn)); err != nil {
		t.Fatal(err)
	}
	if err := Do(ctx, ExposeFunc("once", fn)); err == nil || !strings.Contains(err.Error(), "in use already") {
		t.Fatalf("expected an error for the same name, got: %v", err)
	}
	if err := Do(ctx, ExposeFunc("", fn)); err == nil {
		t.Fatal("expected an error for an empty name")
	}
	// The first func still works.
	if got, err := Run(ctx, Evaluate[string](`once().then(v => "ok")`, awaitPromise)); err != nil || got != "ok" {
		t.Fatalf("expected ok, got: %q (%v)", got, err)
	}
}

// TestExposeFuncNames makes sure that unusual names work, and that two names
// never share a binding. The name "a_reply" must not clash with the reply func
// of the name "a".
func TestExposeFuncNames(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "expose2.html")
	defer cancel()

	echo := func(prefix string) func(context.Context, Void) (string, error) {
		return func(context.Context, Void) (string, error) { return prefix, nil }
	}
	names := []string{"a", "a_reply", "a\x07b", "a b", `a"b`, "a\\b", "__proto__", "é"}
	for _, name := range names {
		if err := Do(ctx, ExposeFunc(name, echo("from "+name))); err != nil {
			t.Fatalf("name %q: %v", name, err)
		}
	}
	for _, name := range names {
		expr, err := jsonv2.Marshal(name)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Run(ctx, Evaluate[string](`Object.getOwnPropertyDescriptor(window, `+string(expr)+`).value()`, awaitPromise))
		if want := "from " + name; err != nil || got != want {
			t.Errorf("name %q: got %q, %v, want %q", name, got, err, want)
		}
	}

	for _, name := range []string{"__chromedp_1", "bad \xff"} {
		if err := Do(ctx, ExposeFunc(name, echo(""))); err == nil {
			t.Errorf("name %q: expected an error", name)
		}
	}
}

// TestExposeFuncBadUTF8 makes sure that the message of an error with bytes that
// are not valid UTF-8 reaches the page, with U+FFFD for each such byte.
func TestExposeFuncBadUTF8(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "expose2.html")
	defer cancel()

	if err := Do(ctx, ExposeFunc("bad", func(context.Context, Void) (Void, error) {
		return Void{}, errors.New("bad \xff byte")
	})); err != nil {
		t.Fatal(err)
	}
	got, err := Run(ctx, Evaluate[string](`bad().then(() => "resolved", e => e.message)`, awaitPromise))
	if want := "bad � byte"; err != nil || got != want {
		t.Fatalf("got %q, %v, want %q", got, err, want)
	}
}

// TestExposeFuncFailureCleanup makes sure that a failed ExposeFunc removes its
// binding and its script. The name "location" fails in the browser, because
// window.location cannot be redefined.
func TestExposeFuncFailureCleanup(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "expose2.html")
	defer cancel()

	fn := func(context.Context, Void) (Void, error) { return Void{}, nil }
	if err := Do(ctx, ExposeFunc("location", fn)); err == nil {
		t.Fatal("expected an error for the name location")
	}
	// Documents that load after the failure must not get the script or the
	// binding.
	if err := Do(ctx, Navigate(testdataDir+"/expose2.html")); err != nil {
		t.Fatal(err)
	}
	const find = `Object.getOwnPropertyNames(window).filter(n => n.startsWith("__chromedp_")).join(",")`
	if got, err := Run(ctx, Evaluate[string](find)); err != nil || got != "" {
		t.Fatalf("expected no private name in the page, got %q, %v", got, err)
	}
	// A retry with a good name works.
	if err := Do(ctx, ExposeFunc("fine", fn)); err != nil {
		t.Fatal(err)
	}
	if got, err := Run(ctx, Evaluate[string](`fine().then(() => "ok")`, awaitPromise)); err != nil || got != "ok" {
		t.Fatalf("got %q, %v", got, err)
	}
	if got, err := Run(ctx, Evaluate[string](find)); err != nil || !strings.Contains(got, "__chromedp_2") || strings.Contains(got, "__chromedp_1") {
		t.Fatalf("expected the names of the second binding only, got %q, %v", got, err)
	}
}
