package chromedp

import (
	"testing"

	"github.com/chromedp/cdproto/page"
)

func TestCloseDialog(t *testing.T) {
	t.Parallel()

	t.Run("Alert", func(t *testing.T) {
		ctx, cancel := testAllocate(t, "")
		defer cancel()

		ListenTarget(ctx, func(ev interface{}) {
			switch ev.(type) {
			case *page.EventJavascriptDialogOpening:
				if err := Run(ctx,
					page.HandleJavaScriptDialog(true),
				); err != nil {
					t.Error(err)
				}
			}
		})

		if err := Run(ctx,
			Navigate(testdataDir+"/dialog.html"),
			Click("#alert", ByID, NodeVisible),
		); err != nil {
			t.Fatalf("got error on DialogText %v", err)
		}
	})
}
