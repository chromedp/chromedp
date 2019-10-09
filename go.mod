module github.com/chromedp/chromedp

go 1.11

require (
	github.com/chromedp/cdproto v0.0.0-20191005232412-878241ce2dca
	github.com/gobwas/httphead v0.0.0-20180130184737-2c6c146eadee // indirect
	github.com/gobwas/pool v0.2.0 // indirect
	github.com/gobwas/ws v1.0.2
	github.com/mailru/easyjson v0.7.0
	golang.org/x/sys v0.0.0-20191005200804-aed5e4c7ecf9 // indirect
)

replace github.com/chromedp/cdproto => ./../cdproto
