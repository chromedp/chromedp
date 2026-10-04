package chromedp

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestDragAndDrop(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		page string
		from string
		to   string
		want []string
	}{
		{
			"mouse events", "drag_mouse.html", "#handle", "#zone",
			// The handle follows the pointer, so the page sees the release over the zone.
			[]string{"mousedown", "mousemove", "mouseup:zone:360,140"},
		},
		{
			"html5", "drag_html5.html", "#item", "#zone",
			[]string{"dragstart", "dragenter", "dragover", "drop:hello", "dragend"},
		},
		{
			"html5 on an element that takes no drop", "drag_html5.html", "#item", "#other",
			[]string{"dragstart", "dragend"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, test.page)
			defer cancel()

			if err := Do(ctx, DragAndDrop(CSS(test.from), CSS(test.to))); err != nil {
				t.Fatalf("got error: %v", err)
			}
			got, err := Run(ctx, Evaluate[[]string]("log"))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if !slices.Equal(got, test.want) {
				t.Fatalf("expected events %v, got: %v", test.want, got)
			}
		})
	}
}

func TestDragAndDropMovesTheNode(t *testing.T) {
	t.Parallel()

	for _, page := range []struct{ name, from, check string }{
		{"drag_mouse.html", "#handle", `Math.round(handle.getBoundingClientRect().x + handle.getBoundingClientRect().width / 2)`},
		{"drag_html5.html", "#item", `item.parentNode.id`},
	} {
		t.Run(page.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, page.name)
			defer cancel()

			if err := Do(ctx, DragAndDrop(CSS(page.from), CSS("#zone"))); err != nil {
				t.Fatalf("got error: %v", err)
			}
			got, err := Run(ctx, Evaluate[any](page.check))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			switch v := got.(type) {
			case float64:
				if v != 360 {
					t.Fatalf("expected the handle at x 360, got: %v", v)
				}
			case string:
				if v != "zone" {
					t.Fatalf("expected the item in the zone, got: %v", v)
				}
			}
		})
	}
}

func TestDragAndDropXY(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		page  string
		steps []int
		want  []string
	}{
		{"mouse events", "drag_mouse.html", nil, []string{"mousedown", "mousemove", "mouseup:zone:360,140"}},
		{"mouse events, one step", "drag_mouse.html", []int{1}, []string{"mousedown", "mousemove", "mouseup:zone:360,140"}},
		{"mouse events, zero steps", "drag_mouse.html", []int{0}, []string{"mousedown", "mousemove", "mouseup:zone:360,140"}},
		{"html5", "drag_html5.html", nil, []string{"dragstart", "dragenter", "dragover", "drop:hello", "dragend"}},
		{"html5, one step", "drag_html5.html", []int{1}, []string{"dragstart", "dragenter", "dragover", "drop:hello", "dragend"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, test.page)
			defer cancel()

			// The handle and the item are near (35, 40), and the zone is at (360, 140).
			if err := Do(ctx, DragAndDropXY(35, 40, 360, 140, test.steps...)); err != nil {
				t.Fatalf("got error: %v", err)
			}
			got, err := Run(ctx, Evaluate[[]string]("log"))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if !slices.Equal(got, test.want) {
				t.Fatalf("expected events %v, got: %v", test.want, got)
			}
		})
	}
}

// TestDragAndDropTwice makes sure that the action turns the interception off,
// because a second drag then starts from a clean state.
func TestDragAndDropTwice(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "drag_html5.html")
	defer cancel()

	if err := Do(ctx,
		DragAndDrop(CSS("#item"), CSS("#zone")),
		DragAndDrop(CSS("#item"), CSS("#other")),
		DragAndDrop(CSS("#item"), CSS("#zone")),
	); err != nil {
		t.Fatalf("got error: %v", err)
	}
	got, err := Run(ctx, Evaluate[[]string]("log"))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	// The item is in the zone after the first drag, so the second drag
	// passes over the zone, and the page logs a different number of enter
	// events. Count the drops and the drags only.
	count := func(name string) int {
		return len(slices.DeleteFunc(slices.Clone(got), func(s string) bool { return s != name }))
	}
	if drags, drops := count("dragstart"), count("drop:hello"); drags != 3 || drops != 2 {
		t.Fatalf("expected 3 drags and 2 drops, got %d and %d in: %v", drags, drops, got)
	}
}

// TestDragAndDropScrolls makes sure that a drag between two nodes works when
// scrolling the second node into view moves the first node, as long as both fit
// in the viewport.
func TestDragAndDropScrolls(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "drag_scroll.html")
	defer cancel()

	if err := Do(ctx, DragAndDrop(CSS("#a"), CSS("#near"))); err != nil {
		t.Fatalf("got error: %v", err)
	}
	got, err := Run(ctx, Evaluate[[]string](`log`))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	want := []string{"mousedown:a", "mouseup:near"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestDragAndDropOutsideViewport makes sure that a drag between two nodes that
// do not fit in the viewport together returns an error and does not press the
// mouse.
func TestDragAndDropOutsideViewport(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "drag_scroll.html")
	defer cancel()

	err := Do(ctx, DragAndDrop(CSS("#a"), CSS("#far")))
	if !errors.Is(err, ErrDragOutsideViewport) {
		t.Fatalf("got error %v, want %v", err, ErrDragOutsideViewport)
	}
	got, err := Run(ctx, Evaluate[[]string](`log`))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got events %q, want none", got)
	}
}

// TestDragAndDropXYReleasesAfterError makes sure that the action releases the
// mouse button when the context ends in the middle of a drag.
func TestDragAndDropXYReleasesAfterError(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "drag_mouse.html")
	defer cancel()

	// The page cancels the context of the drag on the first mouse move.
	dctx, dcancel := context.WithCancel(ctx)
	defer dcancel()
	if err := Do(ctx, ExposeFunc("stopDrag", func(context.Context, Void) (Void, error) {
		dcancel()
		return Void{}, nil
	})); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ctx, Evaluate[Void](`document.addEventListener('mousemove', () => stopDrag(), {once: true})`)); err != nil {
		t.Fatal(err)
	}

	// With so many steps, the drag cannot end before the cancellation arrives.
	err := Do(dctx, DragAndDropXY(35, 40, 360, 140, 2000))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected a canceled context, got: %v", err)
	}
	got, err := Run(ctx, Evaluate[[]string]("log"))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(got); n == 0 || !strings.HasPrefix(got[n-1], "mouseup:") {
		t.Fatalf("expected the last event to be a mouseup, got: %v", got)
	}
}
