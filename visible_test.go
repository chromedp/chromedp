package chromedp

import (
	"context"
	"errors"
	"maps"
	"os"
	"runtime"
	"strings"
	"testing"
)

// headlessFlags is the flag map of the default options. The test
// TestDefaultOptionsHeadless fails when the default list changes.
var headlessFlags = map[string]any{
	"no-first-run":                           true,
	"no-default-browser-check":               true,
	"headless":                               true,
	"hide-scrollbars":                        true,
	"mute-audio":                             true,
	"disable-background-networking":          true,
	"enable-features":                        "NetworkService,NetworkServiceInProcess",
	"disable-background-timer-throttling":    true,
	"disable-backgrounding-occluded-windows": true,
	"disable-breakpad":                       true,
	"disable-client-side-phishing-detection": true,
	"disable-default-apps":                   true,
	"disable-dev-shm-usage":                  true,
	"disable-extensions":                     true,
	"disable-features":                       "site-per-process,Translate,BlinkGenPropertyTrees",
	"disable-hang-monitor":                   true,
	"disable-ipc-flooding-protection":        true,
	"disable-popup-blocking":                 true,
	"disable-prompt-on-repost":               true,
	"disable-renderer-backgrounding":         true,
	"disable-sync":                           true,
	"force-color-profile":                    "srgb",
	"metrics-recording-only":                 true,
	"safebrowsing-disable-auto-update":       true,
	"enable-automation":                      true,
	"password-store":                         "basic",
	"use-mock-keychain":                      true,
}

func TestDefaultOptionsHeadless(t *testing.T) {
	t.Parallel()

	a := setupExecAllocator(DefaultExecAllocatorOptions[:]...)
	if !maps.Equal(a.initFlags, headlessFlags) {
		t.Fatalf("the default flags changed:\nwant %v\ngot  %v", headlessFlags, a.initFlags)
	}
	if a.visibleWindow {
		t.Fatal("the default options must not ask for a visible window")
	}

	// The list that NewContext builds must be the same list.
	b := setupExecAllocator(defaultExecAllocatorOptions(false, false)...)
	if !maps.Equal(b.initFlags, headlessFlags) {
		t.Fatalf("NewContext flags differ from the default list: %v", b.initFlags)
	}
}

func TestVisibleWindowFlags(t *testing.T) {
	t.Parallel()

	want := maps.Clone(headlessFlags)
	for _, name := range []string{
		"headless", "hide-scrollbars", "mute-audio", "enable-automation", "disable-extensions",
	} {
		delete(want, name)
	}
	want["start-maximized"] = true

	for name, opts := range map[string][]ExecAllocatorOption{
		"VisibleWindow after the defaults": append(DefaultExecAllocatorOptions[:len(DefaultExecAllocatorOptions):len(DefaultExecAllocatorOptions)], VisibleWindow),
		"the list of NewContext":           defaultExecAllocatorOptions(true, false),
	} {
		a := setupExecAllocator(opts...)
		if !maps.Equal(a.initFlags, want) {
			t.Errorf("%s:\nwant %v\ngot  %v", name, want, a.initFlags)
		}
		if !a.visibleWindow {
			t.Errorf("%s: visibleWindow is not set", name)
		}
		if _, ok := a.initFlags["auto-open-devtools-for-tabs"]; ok {
			t.Errorf("%s: the developer tools must not open by default", name)
		}
	}
}

func TestVisibleWindowFromEnv(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  bool
	}{
		{"", false},
		{"false", false},
		{"FALSE", false},
		{"0", false},
		{"1", true},
		{"true", true},
		{"yes", true},
	} {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv("CHROMEDP_VISIBLEWINDOW", tt.value)
			if got := visibleWindowFromEnv(); got != tt.want {
				t.Fatalf("value %q: want %v, got %v", tt.value, tt.want, got)
			}
		})
	}
}

func TestNewContextVisibleWindow(t *testing.T) {
	t.Setenv("CHROMEDP_VISIBLEWINDOW", "")
	visible := func(c *Context) bool {
		return c.Allocator.(*ExecAllocator).visibleWindow
	}

	ctx, cancel := NewContext(context.Background())
	defer cancel()
	if visible(FromContext(ctx)) {
		t.Error("NewContext without options must build a headless allocator")
	}

	ctx, cancel = NewContext(context.Background(), WithVisibleWindow())
	defer cancel()
	if !visible(FromContext(ctx)) {
		t.Error("WithVisibleWindow must build a visible allocator")
	}

	t.Setenv("CHROMEDP_VISIBLEWINDOW", "1")
	ctx, cancel = NewContext(context.Background())
	defer cancel()
	if !visible(FromContext(ctx)) {
		t.Error("the environment variable must build a visible allocator")
	}
}

func TestWithVisibleWindowKeepsCallerAllocator(t *testing.T) {
	t.Setenv("CHROMEDP_VISIBLEWINDOW", "1")
	allocCtx, cancel := NewExecAllocator(context.Background(), DefaultExecAllocatorOptions[:]...)
	defer cancel()
	ctx, cancel := NewContext(allocCtx, WithVisibleWindow())
	defer cancel()
	if FromContext(ctx).Allocator.(*ExecAllocator).visibleWindow {
		t.Fatal("WithVisibleWindow must not change an allocator that the caller made")
	}
}

func TestErrNoDisplay(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only Linux checks for a display")
	}
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	// The path does not exist, so no test can start a browser by mistake.
	missing := ExecPath(os.DevNull + "/no-such-browser")

	for name, newCtx := range map[string]func() (context.Context, context.CancelFunc){
		"option": func() (context.Context, context.CancelFunc) {
			return NewContext(context.Background(), WithVisibleWindow())
		},
		"allocator option": func() (context.Context, context.CancelFunc) {
			allocCtx, cancel := NewExecAllocator(context.Background(), missing, VisibleWindow)
			ctx, cancel2 := NewContext(allocCtx)
			return ctx, func() { cancel2(); cancel() }
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := newCtx()
			defer cancel()
			err := Do(ctx)
			if !errors.Is(err, ErrNoDisplay) {
				t.Fatalf("want ErrNoDisplay, got %v", err)
			}
		})
	}

	t.Run("env", func(t *testing.T) {
		t.Setenv("CHROMEDP_VISIBLEWINDOW", "1")
		ctx, cancel := NewContext(context.Background())
		defer cancel()
		if err := Do(ctx); !errors.Is(err, ErrNoDisplay) {
			t.Fatalf("want ErrNoDisplay, got %v", err)
		}
	})

	t.Run("message", func(t *testing.T) {
		if got := ErrNoDisplay.Error(); !strings.Contains(got, "CHROMEDP_VISIBLEWINDOW") {
			t.Fatalf("the message must name the variable: %q", got)
		}
	})
}

func TestCheckDisplayLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only Linux checks for a display")
	}
	for _, tt := range []struct {
		display, wayland string
		wantErr          bool
	}{
		{"", "", true},
		{":0", "", false},
		{"", "wayland-0", false},
		{":0", "wayland-0", false},
	} {
		t.Setenv("DISPLAY", tt.display)
		t.Setenv("WAYLAND_DISPLAY", tt.wayland)
		if err := checkDisplay(); (err != nil) != tt.wantErr {
			t.Errorf("DISPLAY=%q WAYLAND_DISPLAY=%q: got %v", tt.display, tt.wayland, err)
		}
	}
}
