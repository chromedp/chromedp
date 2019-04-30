package chromedp

import (
	"testing"

	"github.com/chromedp/cdproto/page"
)

func TestCloseDialog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		accept     bool
		promptText string
		dialogType page.DialogType
		sel        string
		want       string
	}{
		{
			name:       "AlertAcceptWithPromptText",
			accept:     true,
			promptText: "this is a prompt text",
			dialogType: page.DialogTypeAlert,
			sel:        "#alert",
			want:       "alert text",
		},
		{
			name:       "AlertDismissWithPromptText",
			accept:     false,
			promptText: "this is a prompt text",
			dialogType: page.DialogTypeAlert,
			sel:        "#alert",
			want:       "alert text",
		},
		{
			name:       "AlertAcceptWithoutPromptText",
			accept:     true,
			dialogType: page.DialogTypeAlert,
			sel:        "#alert",
			want:       "alert text",
		},
		{
			name:       "AlertDismissWithoutPromptText",
			accept:     false,
			dialogType: page.DialogTypeAlert,
			sel:        "#alert",
			want:       "alert text",
		},
		{
			name:       "PromptAcceptWithPromptText",
			accept:     true,
			promptText: "this is a prompt text",
			dialogType: page.DialogTypePrompt,
			sel:        "#prompt",
			want:       "prompt text",
		},
		{
			name:       "PromptDismissWithPromptText",
			accept:     false,
			promptText: "this is a prompt text",
			dialogType: page.DialogTypePrompt,
			sel:        "#prompt",
			want:       "prompt text",
		},
		{
			name:       "PromptAcceptWithoutPromptText",
			accept:     true,
			dialogType: page.DialogTypePrompt,
			sel:        "#prompt",
			want:       "prompt text",
		},
		{
			name:       "PromptDismissWithoutPromptText",
			accept:     false,
			dialogType: page.DialogTypePrompt,
			sel:        "#prompt",
			want:       "prompt text",
		},
		{
			name:       "ConfirmAcceptWithPromptText",
			accept:     true,
			promptText: "this is a prompt text",
			dialogType: page.DialogTypeConfirm,
			sel:        "#confirm",
			want:       "confirm text",
		},
		{
			name:       "ConfirmDismissWithPromptText",
			accept:     false,
			promptText: "this is a prompt text",
			dialogType: page.DialogTypeConfirm,
			sel:        "#confirm",
			want:       "confirm text",
		},
		{
			name:       "ConfirmAcceptWithoutPromptText",
			accept:     true,
			dialogType: page.DialogTypeConfirm,
			sel:        "#confirm",
			want:       "confirm text",
		},
		{
			name:       "ConfirmDismissWithoutPromptText",
			accept:     false,
			dialogType: page.DialogTypeConfirm,
			sel:        "#confirm",
			want:       "confirm text",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "")
			defer cancel()

			ListenTarget(ctx, func(ev interface{}) {
				switch e := ev.(type) {
				case *page.EventJavascriptDialogOpening:
					if e.Type != test.dialogType {
						t.Errorf("expected dialog type to be '%s', got: '%s'", test.dialogType, e.Type)
					}
					if e.Message != test.want {
						t.Errorf("expected dialog message to be '%s', got: '%s'", test.want, e.Message)
					}

					task := page.HandleJavaScriptDialog(test.accept)
					if test.promptText != "" {
						task = task.WithPromptText(test.promptText)
					}
					if err := Run(ctx, task); err != nil {
						t.Error(err)
					}
				case *page.EventJavascriptDialogClosed:
					if e.Result != test.accept {
						t.Errorf("expected result to be %t, got %t", test.accept, e.Result)
					}
					if e.UserInput != test.promptText {
						t.Errorf("expected user input to be %q, got %q", test.promptText, e.UserInput)
					}
				}
			})

			if err := Run(ctx,
				Navigate(testdataDir+"/dialog.html"),
				Click(test.sel, ByID, NodeVisible),
			); err != nil {
				t.Fatalf("got error on DialogText %v", err)
			}
		})
	}
}
