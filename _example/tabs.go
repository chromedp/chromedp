package main

import (
	"context"
	"log"

	"github.com/chromedp/chromedp"
)

func main() {
	// first tab
	ctx1, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// create new tab
	ctx2, _ := chromedp.NewContext(ctx1)

	// runs in first tab
	if err := chromedp.Run(ctx1, myTask()); err != nil {
		log.Fatal(err)
	}

	// runs in second tab
	if err := chromedp.Run(ctx2, myTask()); err != nil {
		log.Fatal(err)
	}
}

func myTask() chromedp.Tasks {
	return []chromedp.Action{}
}
