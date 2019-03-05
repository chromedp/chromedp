package main

import (
	"context"
	"fmt"
	"log"

	"github.com/chromedp/chromedp"
)

func main() {
	// create a new context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// grab the title
	var title string
	if err := chromedp.Run(ctx, grabTitle(&title)); err != nil {
		log.Fatal(err)
	}

	// print it
	fmt.Println(title)

	// ensure all resources are cleaned up
	cancel()
	chromedp.FromContext(ctx).Wait()
}

func grabTitle(title *string) chromedp.Tasks {
	return []chromedp.Action{
		chromedp.Navigate("https://github.com/"),
		chromedp.WaitVisible("#start-of-content", chromedp.ByID),
		chromedp.Title(title),
	}
}
