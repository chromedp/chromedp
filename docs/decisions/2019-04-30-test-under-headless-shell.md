# Run the tests under headless-shell as well as Chrome

Status: Decided.

On 2019-04-30 the commit `4fe9ec5` made CI run the tests twice. One run uses
the Chrome on the CI machine. The other run uses the `headless-shell` image
through Docker.

The reason, from that commit, is this. `headless-shell` is a smaller build of
Chrome with a different set of flags. A second run covers cases that Chrome
does not. Today `contrib/docker-test.sh` runs the second pass, and
`.github/workflows/test.yml` calls it on Linux. CI also runs the tests with
Chrome on Windows and macOS.

A test that fails on `headless-shell` must skip there, with a comment that says
why. The `HEADLESS_SHELL` variable, which the script sets, marks that run.
