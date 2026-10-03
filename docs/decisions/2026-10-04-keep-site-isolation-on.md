# Keep site isolation on

Status: Proposed.

The default exec allocator options no longer pass `site-per-process` in
`--disable-features`. This file records what the flag did, and why the option is
removed and not renamed. The maintainer must confirm it.

## What the flag did

`--disable-features` takes the names of features, and Chrome matches them with
case. The feature for full site isolation is `SitePerProcess`. The Chromium
source `chrome/common/chrome_features.cc` defines it with the macro
`BASE_FEATURE(kSitePerProcess, ...)`. It is on by default on desktop, and the
source has no alias for it. The name `site-per-process` is a command line
switch, so Chrome did not find a feature of that name and ignored the value.
The flag did nothing in any recent Chrome.

## Why the option is removed

The pull request 1606 changes the value to `SitePerProcess`. That turns off
full site isolation for every program that uses the default options. Chrome
then keeps frames of other sites in the same process. That changes the
security model of the browser, and it changes how frames become targets in
`chromedp`. The change is silent, and the option has never worked, so
nobody depends on it. Removing the dead name keeps the behavior of today.

## What the maintainer can decide

- Keep it as it is. This is the current state.
- Turn off site isolation on purpose, with `Flag("disable-features", "SitePerProcess")`. A program that wants it can add it today. The default options then need a note in the docs.

The test `TestDefaultFeatureNames` fails when a name in `enable-features` or
`disable-features` has the form of a switch.
