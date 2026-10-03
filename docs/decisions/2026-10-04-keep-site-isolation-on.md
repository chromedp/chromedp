# Keep site isolation on

Status: Decided.

The default exec allocator options no longer pass `site-per-process` in
`--disable-features`. This file records what the flag did, and why the option is
removed and not renamed. The maintainer decided on 2026-10-04 to keep site
isolation on, so pull request 1606 and issue 1605 are closed.

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

## What the maintainer decided

Keep it as it is. A program that wants to turn off site isolation can add
`Flag("disable-features", "SitePerProcess")` to its allocator options. The default
options do not do it.

The test `TestDefaultFeatureNames` fails when a name in `enable-features` or
`disable-features` has the form of a switch.
