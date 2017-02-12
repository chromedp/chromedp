package chromedp

import (
	"log"
	"os"
)

var (
	// Logger is the default package logger
	Logger = log.New(os.Stderr, "ChromeDP ", log.LstdFlags)
)
