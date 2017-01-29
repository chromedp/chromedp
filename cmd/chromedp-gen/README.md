# About chromedp-gen

`chromedp-gen` is a standalone cli tool for the [chromedp](https://github.com/knq/chromedp)
package that generates the types for the chrome domain APIs as defined in
Chrome's `protocol.json`.

## Updating protocol.json

Run [`update.sh`](update.sh) to retrieve and combine the latest
[`browser_protocol.json`](https://chromium.googlesource.com/chromium/src/+/master/third_party/WebKit/Source/core/inspector/browser_protocol.json) and
[`js_protocol.json`](https://chromium.googlesource.com/v8/v8/+/master/src/inspector/js_protocol.json)
from the [Chromium source tree](https://chromium.googlesource.com/) into [`protocol.json`](protocol.json).

## Installing quicktemplate and easyjson

`chromedp-gen` uses [quicktemplate](https://github.com/valyala/quicktemplate) for
code generation templates, and uses [easyjson](https://github.com/mailru/easyjson)
for JSON marshaling/unmarshaling.

Unfortunately, both of these projects cannot be used directly out of the box,
as the versions used by `chromedp-gen` make use of a couple minor modifications:

1. quicktemplate has been modified to recognize '{% end %}' templates
2. easyjson has been modified to always output generated code in the same order
   (previously, the mailru version did not always generate the same output, as
   a couple points in the code were being affected by the Go map/key order
   issue).

I ([kenshaw](https://github.com/kenshaw)) have put in issues/pull requests to
both projects in the hope that the changes I've made get merged and/or fixed
otherwise.

For now, the exact versions that are in use with `chromedp-gen` can be obtained
on my personal Github account:

* [github.com/kenshaw/quicktemplate](https://github.com/kenshaw/quicktemplate)
* [github.com/kenshaw/easyjson](https://github.com/kenshaw/easyjson)

The commands (and their associated dependencies) can be installed in the usual
Go way:

```sh
go get -u github.com/kenshaw/quicktemplate/qtc github.com/kenshaw/easyjson/easyjson
```

Each package provides a command (`qtc` and `easyjson`, respectively) that need
to be available on the path for the `chromedp-gen` command to work.

## Generating types for chromedp

Assuming the `qtc` and `easyjson` commands are available (see above), one can
then run the [`build.sh`](build.sh) which will handle generating template
files, and building/invoking the `chromedp-gen` command.

## Reference Output

The output of running `update.sh` and `build.sh` is below:
```sh
ken@ken-desktop:~/src/go/src/github.com/knq/chromedp/cmd/chromedp-gen$ ./update.sh
# download
curl -s $BROWSER_PROTO | base64 -d > $BROWSER_TMP
curl -s $JS_PROTO | base64 -d > $JS_TMP

# merge browser_protocol.json and js_protocol.json
jq -s '[.[] | to_entries] | flatten | reduce .[] as $dot ({}; .[$dot.key] += $dot.value)' $BROWSER_TMP $JS_TMP > $OUT

# convert boolean values listed as strings to real booleans
# (this is not used in favor of using the custom Bool type that correctly JSON unmarshals the value)
# left here for completeness
#perl -pi -e 's/"\s*:\s*"(true|false)"/": \1/g' $OUT
ken@ken-desktop:~/src/go/src/github.com/knq/chromedp/cmd/chromedp-gen$ ./build.sh
go generate
qtc: 2017/01/29 09:10:16 Compiling *.qtpl template files in directory "templates"
qtc: 2017/01/29 09:10:16 Compiling "templates/domain.qtpl" to "templates/domain.qtpl.go"...
qtc: 2017/01/29 09:10:16 Compiling "templates/extra.qtpl" to "templates/extra.qtpl.go"...
qtc: 2017/01/29 09:10:16 Compiling "templates/file.qtpl" to "templates/file.qtpl.go"...
qtc: 2017/01/29 09:10:16 Compiling "templates/type.qtpl" to "templates/type.qtpl.go"...
qtc: 2017/01/29 09:10:16 Total files compiled: 4

go build

time ./chromedp-gen
2017/01/29 09:10:17 skipping command Page.getCookies [redirect:Network]
2017/01/29 09:10:17 skipping command Page.deleteCookie [redirect:Network]
2017/01/29 09:10:17 skipping command Page.setDeviceMetricsOverride [redirect:Emulation]
2017/01/29 09:10:17 skipping command Page.clearDeviceMetricsOverride [redirect:Emulation]
2017/01/29 09:10:17 skipping command Page.setGeolocationOverride [redirect:Emulation]
2017/01/29 09:10:17 skipping command Page.clearGeolocationOverride [redirect:Emulation]
2017/01/29 09:10:17 skipping command Page.setDeviceOrientationOverride [redirect:DeviceOrientation]
2017/01/29 09:10:17 skipping command Page.clearDeviceOrientationOverride [redirect:DeviceOrientation]
2017/01/29 09:10:17 skipping command Page.setTouchEmulationEnabled [redirect:Emulation]
2017/01/29 09:10:17 skipping command param Emulation.setDeviceMetricsOverride.offsetX [deprecated]
2017/01/29 09:10:17 skipping command param Emulation.setDeviceMetricsOverride.offsetY [deprecated]
2017/01/29 09:10:17 skipping command param Tracing.start.categories [deprecated]
2017/01/29 09:10:17 skipping command param Tracing.start.options [deprecated]
2017/01/29 09:10:17 skipping domain Console (console) [deprecated]
2017/01/29 09:10:17 running goimports
2017/01/29 09:10:31 running easyjson (stubs)
2017/01/29 09:10:31 running easyjson
2017/01/29 09:10:39 done.

real	0m22.318s
user	1m15.132s
sys	0m53.592s

go install ../../cdp/...
ken@ken-desktop:~/src/go/src/github.com/knq/chromedp/cmd/chromedp-gen$
```
