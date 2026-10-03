# Strict mode: the dictionary discipline

Read this file when the user names STE, ASD-STE100, or compliance. Strict mode adds the dictionary rules to the document. It does not change the reply to the user, which stays in Plain mode.

The official dictionary (about 900 approved words and 1,200 rejected words with their alternatives) is copyrighted by ASD and is not reproduced here. This file gives the rules that depend on it, the rulings that software writers meet most, and the words the standard names as recurring errors. Tell the user, one time per conversation and in one sentence, that this index is lossy and that full compliance needs the official dictionary (free at asd-ste100.org).

## Rules that exist only with the dictionary

| Rule | Instruction |
|---|---|
| 1.1 | Use only approved words, technical nouns, or technical verbs. |
| 1.2 | Use an approved word only as its listed part of speech. |
| 1.3 | Use an approved word only with its approved meaning. |
| 1.4 | Use only the approved forms of verbs and adjectives. |
| 1.6 | Use an unapproved word only when it is a technical noun or part of one. |
| 3.1 | Use only the verb forms that the dictionary gives. |
| 9.2 | Use each approved word correctly: approved meaning, approved part of speech. |
| GR-5 | Avoid false friends: words that look like a word in the reader's language but mean something else. |
| GR-8 | Use the possessive apostrophe only when you are sure that it is correct. If unsure, write "the file of the user". |

Issue 9 adds a quick-reference list of approved verbs in the dictionary introduction. Check your verbs against it.

## Part-of-speech rulings

| Word | Ruling |
|---|---|
| test, check, work | Noun only. "Do a test", not "test the pump". "Check that X" becomes "make sure that X". |
| oil | Technical noun only. For the verb, the dictionary gives "lubricate": "Lubricate the linkage with oil." |
| help | Verb only. For the noun, the dictionary gives "aid": "with the aid of". |
| fall (noun) | Rejected. Use "decrease" for a reduction in value. Use "fall" (verb) only for movement downward by gravity: "Make sure that the tools do not fall into the engine." |
| follow | "To come after" only, never "obey". Write "obey the instructions". |
| above, below | Physical positions only. For limits write "more than", "less than". |

## Dictionary rulings on common software verbs

The standard has already chosen. Use the approved word.

| You wrote | Dictionary status | Use instead |
|---|---|---|
| check (verb), verify, confirm, ensure | All rejected as verbs | Route by intent: `make sure that` (a state), `examine` (look for faults: "examine the log"), `measure` (get a value), or the noun: "do a check of". |
| validate | Not in the dictionary | Legal as a technical verb (Rule 1.12), or replace with `make sure that`. |
| delete, drop (verb), destroy | All rejected as dictionary verbs | `erase` (data), `remove` (physical). In computer contexts `delete` is also a legal technical verb (Rule 1.12). Do not use `drop` or `destroy`. |
| remove | Approved verb | Keep it. |
| run, execute | Both rejected | `operate` for run, `do` for execute. |
| invoke, launch | Not in the dictionary | Legal as technical verbs (Rule 1.12). |
| display (verb), render, present (verb) | All rejected | `show` covers most software cases. Official alternatives: display → `show`, render → `make`, present → `give` or `show`. |
| issue | Not in the dictionary | Use as a technical noun, or replace with `problem` (approved). |
| failure | Rejected in general use; approved as a technical noun for performance loss | Use only for a performance error: "a failure of the pump". |
| error, problem | Approved nouns | Keep them. |

## Recurring errors the standard names

The dictionary introduction lists the words that writers get wrong most often. This is the software-relevant set, as rulings only.

| You wrote | STE writes |
|---|---|
| however | but |
| therefore | thus, as a result |
| since (= because) | because |
| any | Delete it, or restructure: "if you have any questions" → "if you have questions" |
| now | at this time. Better, delete it: "now start the service" → "start the service" |
| need to, have to | Imperative in procedures ("install"); "it is necessary to" in descriptive text |
| perform | do |
| insert | put (but SQL `INSERT` stays: it is quoted text) |
| reach | get, get to |
| avoid | prevent |
| repeat | do … again |
| acceptable | permitted. Better, give the limit: "a latency of less than 200 ms" |
| complete (adjective) | completed |
| the example below, the section above | Name the target, or put the reference after it: "the example that follows" |

## Strict self-check

Add these two steps to the self-check in SKILL.md:

1. Search the draft for every verb in the two tables above. Replace each hit with the approved word.
2. Search for the phrasal verbs you built ("set up", "go down"). Replace each with the one-word verb (Rule 9.3: "install", "decrease").
