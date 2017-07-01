package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	cdp "github.com/knq/chromedp"
)

var (
	flagPort = flag.Int("port", 8544, "port")
)

func main() {
	var err error

	flag.Parse()

	// create http server and result channel
	result := make(chan int, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(res, uploadHTML)
	})
	mux.HandleFunc("/upload", func(res http.ResponseWriter, req *http.Request) {
		f, _, err := req.FormFile("upload")
		if err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}
		defer f.Close()

		buf, err := ioutil.ReadAll(f)
		if err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}

		fmt.Fprintf(res, resultHTML, len(buf))

		result <- len(buf)
	})

	go http.ListenAndServe(fmt.Sprintf(":%d", *flagPort), mux)

	// create context
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create chrome instance
	c, err := cdp.New(ctxt, cdp.WithLog(log.Printf))
	if err != nil {
		log.Fatal(err)
	}

	// get wd
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	filepath := wd + "/main.go"

	// get some info about the file
	fi, err := os.Stat(filepath)
	if err != nil {
		log.Fatal(err)
	}

	// run task list
	var sz string
	err = c.Run(ctxt, upload(filepath, &sz))
	if err != nil {
		log.Fatal(err)
	}

	// shutdown chrome
	err = c.Shutdown(ctxt)
	if err != nil {
		log.Fatal(err)
	}

	// wait for chrome to finish
	err = c.Wait()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("original size: %d, upload size: %d", fi.Size(), <-result)
}

func upload(filepath string, sz *string) cdp.Tasks {
	return cdp.Tasks{
		cdp.Navigate(fmt.Sprintf("http://localhost:%d", *flagPort)),
		cdp.SendKeys(`input[name="upload"]`, filepath, cdp.NodeVisible),
		cdp.Click(`input[name="submit"]`),
		cdp.Text(`#result`, sz, cdp.ByID, cdp.NodeVisible),
	}
}

const (
	uploadHTML = `<!doctype html>
<html>
<body>
  <form method="POST" action="/upload" enctype="multipart/form-data">
    <input name="upload" type="file"/>
    <input name="submit" type="submit"/>
  </form>
</body>
</html>`

	resultHTML = `<!doctype html>
<html>
<body>
  <div id="result">%d</div>
</body>
</html>`
)
