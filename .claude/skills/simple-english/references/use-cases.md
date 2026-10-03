# Use cases beyond documentation

STE was built for aircraft maintenance manuals. The same properties transfer to any text where a misreading has a cost: one meaning per word, short sentences, condition-first commands. Each case below names the mode and the adaptations.

## Error messages and CLI output

Mode: procedural. An error message is an instruction to a reader who is in the middle of a failure.

Pattern: state what happened (simple past), state the cause if known, give the command or condition that fixes it.

> Before: Oops! Something went wrong while attempting to establish a connection. Please ensure your credentials are properly configured and try again.
> After: Connection to the database failed. The password for user `app` was not correct. Set `DB_PASSWORD` and connect again.

## Runbooks and standard operating procedures

Mode: procedural. An on-call runbook is a maintenance manual, which is the text type that STE was written for.

- Every step is imperative, one instruction per step, condition first.
- A warning comes before its step: command first, risk second.
- No sentence has more than 20 words.

## Incident reports and postmortems

Mode: descriptive, simple past only. A timeline in present perfect ("we have identified") hides when things happened.

> Before: We have identified an issue that may have impacted some users' ability to access the service.
> After: Between 14:02 and 14:31 UTC, 12% of requests failed. A deploy at 14:00 removed the cache warmup step.

STE bans hedges such as "may have impacted". The report states what is known and says "unknown" for the rest.

## Commit messages and PR descriptions

Mode: imperative subject line, descriptive body. The convention already matches STE. Apply the word swaps and the 25-word limit to the body. Delete "this PR aims to".

## API changelogs and release notes

Mode: descriptive. One entry, one change, one sentence where possible. A "Breaking:" entry follows the warning pattern, command first: "Update your calls to `v2/users`. The `name` field split into `first_name` and `last_name`."

## Instructions for AI agents (prompts, AGENTS.md, skills)

Mode: procedural. A system prompt is a procedure for a reader that cannot ask questions.

- One instruction per sentence keeps each rule quotable and hard to half-follow.
- One word, one meaning stops the model from treating "check", "verify", and "validate" as three operations.
- A condition first ("If the build fails, stop") beats a trailing condition, which models drop.
- No "should". A model reads "should" as optional. Write "must" or delete the rule.

## Support macros and status-page updates

Mode: descriptive, 25-word limit. Many readers of these messages are non-native speakers.

> Before: We sincerely apologize for any inconvenience this may have caused.
> After: The API was down for 18 minutes. Uploads made during this time were saved and will process today.

## Translation and localization prep

Mode: strict. STE was written so that non-native maintenance crews can read English manuals. The same rules prepare text for machine translation. One meaning per word and complete grammar (articles, "that") remove many ambiguities that a translator must otherwise guess at.

## UI copy and empty states

Mode: procedural, hard length limits. Buttons and labels are technical names and are exempt. Body copy follows the rules: "No projects yet. Create a project to start."

## Where STE does not fit

Do not use STE for marketing pages, launch posts, blog posts, or brand writing. STE removes persuasion. Write those texts in your own voice, and use STE for the docs that they link to.
