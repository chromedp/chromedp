package main

import (
	"context"
	"log"

	"github.com/chromedp/chromedp"
)

func main() {
	dockerAllocatorOpts := []chromedp.DockerAllocatorOption{}

	ctxt, cancel := chromedp.NewAllocator(context.Background(), chromedp.WithDockerAllocator(dockerAllocatorOpts...))
	defer cancel()

	task1Context, cancel := chromedp.NewContext(ctxt)
	defer cancel()

	if err := chromedp.Run(task1Context, myTask()); err != nil {
		log.Fatal(err)
	}
}

func myTask() chromedp.Tasks {
	return []chromedp.Action{}
}
