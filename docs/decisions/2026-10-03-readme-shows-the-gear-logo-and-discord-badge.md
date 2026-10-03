# The README shows the gear logo and the Discord badge

Status: Decided.

The maintainer decided on 2026-10-03 that the `chromedp` README shows the new gear logo
from `github.com/chromedp/logo` and a Discord badge.

The logo repository holds the logo as `chromedp.svg`, with a PNG render. The
README loads the SVG from the raw address of that repository, so a later change
to the logo shows in the README without a change here.

Both are on `main`. The logo is commit `53e08ac` and the badge is commit
`8b2fed8`. The badge is the one that `xo/usql` shows. Its invite address and
its server number come from that README, so it points at the same Discord
server.

The maintainer confirmed on 2026-10-03 that this is the right server. It is the only one
that the maintainer runs, and there is no plan to run a second.
