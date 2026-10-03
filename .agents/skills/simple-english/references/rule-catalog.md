# Rule catalog: ASD-STE100 Issue 9 for software text

Read this file for CHECK mode, for Strict mode, or when a rule number is in question. The core skill in `SKILL.md` names the rules that move output the most. This file holds the whole catalog.

53 rules in 9 sections, paraphrased from ASD-STE100 Issue 9 with software examples. Rules marked (S) are Strict mode only (see `references/strict-vocabulary.md`). The official wording is in the free standard at asd-ste100.org.

### Section 1: Words (Rules 1.1-1.14)

| Rule | Instruction |
|---|---|
| 1.1-1.4, 1.6 (S) | Use only approved words, as their listed part of speech, meaning, and form. |
| 1.5 | You can use domain words as technical nouns ("webhook", "commit", "endpoint"). |
| 1.7 | Do not use technical nouns as verbs. |
| 1.8 | Use the technical nouns of your project or industry. |
| 1.9 | When you pick a technical noun, pick a short and clear one. |
| 1.10 | No regional, slang, or jargon words as technical nouns. |
| 1.11 | One item, one name. Do not call it "config" here and "settings" there. |
| 1.12 | You can use domain verbs as technical verbs ("deploy", "compile", "merge"). The standard names computer verbs as legal: click, type, copy, paste, delete, save, install, download, update, and more. When a common verb does the same job, prefer it: "find" instead of "detect". |
| 1.13 | Do not use technical verbs as nouns. |
| 1.14 | Use American English spelling. |

In Plain mode, rules 1.5, 1.8, and 1.12 make your domain vocabulary legal.

Before: You can webhook the event, then do a deploy.
After: Send the event to the webhook. Then deploy the service.

### Section 2: Multi-word nouns (Rules 2.1-2.2)

| Rule | Instruction |
|---|---|
| 2.1 | Write multi-word nouns of three words or fewer. |
| 2.2 | When a technical noun needs more than three words, write it in full once, then give a short form or hyphenate the units. |

Break long noun chains with prepositions (of, on, in, for):

Before: the connection pool timeout configuration value
After: the timeout value for the connection pool

### Section 3: Verbs (Rules 3.1-3.7)

| Rule | Instruction |
|---|---|
| 3.1 (S) | Use only the verb forms that the dictionary gives. |
| 3.2 | Use only: infinitive, imperative, simple present, simple past, simple future, past participle as adjective. |
| 3.3 | Use the past participle only as an adjective ("the cached response"). |
| 3.4 | No auxiliary verbs for complex constructions. No present perfect, no "is to be installed". |
| 3.5 | Use an "-ing" form only as a technical noun or inside one ("logging", "the mounting bracket"), never as a verb. |
| 3.6 | Active voice. In descriptive text, passive is legal only when the agent is unknown. To repair an agentless passive, use "you" (the reader) or "we" (your company): "Indexes are not used on this table" → "We do not use indexes on this table." |
| 3.7 | Describe an action with a verb, not a noun ("compress the file", not "perform compression of the file"). |

Approved modals: can, will, must. Banned: should, would, may, might, could. The modal ladder at the end of this file gives the replacement for each banned modal.

### Section 4: Sentences (Rules 4.1-4.5)

| Rule | Instruction |
|---|---|
| 4.1 | Write short and clear sentences. |
| 4.2 | Do not omit words or use contractions to shorten sentences. Keep articles, keep "that". |
| 4.3 | Use a vertical list for complex text: colon on the lead-in, uppercase start, a period only on full-sentence items, no mixed instructions and facts, no nesting. |
| 4.4 | Use connecting words between sentences on related topics ("Then", "As a result"). |
| 4.5 | Put an article (the, a, an) or a demonstrative adjective (this, these) before nouns where applicable. Exception: no article before a noun when an identifier follows it: "Restart pod web-7f9b2". |

Rule 4.2 is the anti-terseness rule. Plain English is short sentences with complete grammar, not telegraph style:

Wrong shortening: Ensure file exists before running.
Plain: Make sure that the file exists before you run the command.

### Section 5: Procedural writing (Rules 5.1-5.5)

| Rule | Instruction |
|---|---|
| 5.1 | Maximum 20 words per sentence. Warnings and cautions included. |
| 5.2 | One instruction per sentence, unless two actions happen at the same time. A step can add one sentence for an immediate result or limit. |
| 5.3 | Write instructions in the imperative: "Run the migration." |
| 5.4 | Put a required condition before the command, divided by a comma: "If the build fails, read the log." |
| 5.5 | Notes give information, never instructions or limits. A limit belongs with its action. Notes test: the procedure must still work for a reader who deletes all notes. |

Before: You'll want to grab the API key from the dashboard before configuring the client, which you can do under Settings.
After: Get the API key from the dashboard, under Settings. Then configure the client with this key.

### Section 6: Descriptive writing (Rules 6.1-6.6)

| Rule | Instruction |
|---|---|
| 6.1 | Give information gradually: one new fact per sentence. |
| 6.2 | Use key words and phrases to give the text a logical structure. |
| 6.3 | Maximum 25 words per sentence. |
| 6.4 | Group related information in paragraphs. |
| 6.5 | One topic per paragraph. |
| 6.6 | Maximum six sentences per paragraph. |

### Section 7: Safety instructions (Rules 7.1-7.3)

| Rule | Instruction |
|---|---|
| 7.1 | Use a word that shows the risk level ("WARNING" = injury, "CAUTION" = damage). If the two risks occur together, use "WARNING". |
| 7.2 | Start with a clear command or condition. |
| 7.3 | Then give the risk or the possible result. |

Put the instruction before the explanation. Use the same pattern for destructive CLI flags and migrations that you cannot undo.

Before: Note that data loss may occur in some circumstances if the destructive flag happens to be enabled when running against production.
After: CAUTION: Do not use the `--force` flag against production. The flag deletes rows that do not match the source.

### Section 8: Punctuation and word count (Rules 8.1-8.7)

| Rule | Instruction |
|---|---|
| 8.1 | All standard punctuation is legal except the semicolon. Write two sentences instead. |
| 8.2 | Use hyphens to connect words that act as one unit. |
| 8.3 | Parentheses are legal for references, item numbers, abbreviations, plural forms, explanations, alternatives. |
| 8.4 | In a vertical list, the lead-in colon ends a sentence for word count. Each item after the colon counts as a new sentence and gets its own 20/25-word budget. |
| 8.5-8.7 | Count as one word each: text in parentheses, a hyphenated word, numbers, numbers with units, abbreviations, identifiers, quoted text, titles, labels, proper nouns. |

Rule 8.6 matters for software text: `sqlpipe run --config sqlpipe.yaml` in backticks counts as one word.

Dashes (a rule of this skill, not of the standard): an em-dash (`—`) splices two statements and hides the logic between them. Name the relation ("because", "but", "for example") or write two sentences. A spaced or double hyphen between statements is the same dash. A range (`5–10`), a list marker, and a flag (`--force`) are not.

### Section 9: Writing practices (Rules 9.1-9.4, GR-1 to GR-8)

| Rule | Instruction |
|---|---|
| 9.1 | When a word-for-word replacement does not work, restructure the sentence. |
| 9.2 (S) | Use each approved word correctly: approved meaning, approved part of speech. |
| 9.3 | Prefer the one-word verb over the phrasal verb ("decrease", not "go down"; "install", not "set up"). Strict mode: the phrasal verb is a violation. |
| 9.4 | Keep one consistent style and terminology through the whole document. |

General recommendations: keep "that" (GR-1), primary verb first and the tool after "with" (GR-2: "Fetch the URL with curl"), clear pronoun referents (GR-3), "this + noun" (GR-4), inclusive language (GR-7). GR-6: "e.g." → "for example", "i.e." → "that is", delete "etc." and name the items.

### The modal ladder

| You wrote | Write instead |
|---|---|
| should (requirement) | must |
| should (recommendation) | Delete it, or state it as fact: "X is better because Y." |
| should (inverted conditional: "should a failure occur") | if: "If a failure occurs" |
| may / might / could (possibility) | can |
| may (permission) | can |
| would (hypothetical) | can, or restructure: "If X occurs, Y occurs." |

## Signs of AI Writing

AI text drifts in known directions (Wikipedia "Signs of AI writing"). The rules above remove some already. Guard against the rest by direction, in documents and replies alike:

- Inflated significance: no "vital", "crucial", "a testament". State the fact.
- Negative parallelism: no "not just X, it is Y".
- Rule of three: no decorative triplets.
- Vague attribution: no "studies show". Name the source, or drop the claim.
- False ranges: no "ranging from X to Y" without real limits.
- Restating summaries: no "in conclusion" paragraphs.
- Editorializing asides: no "it is important to note".
- Collaborative leftovers: no "I hope this helps", no "Let me know".
- Formatting habits: no bold as decoration, no bold lead-ins, no emoji as structure, no heading for two sentences.

For the specific overused words, `references/word-swaps.md` maps each one to a plain replacement. If a word carries no fact, delete it instead.

## Word Choice

One word, one meaning, one part of speech, for the whole document (Rules 1.11, 9.4).

- The settings file is `configuration`, never config, settings, or options in the same document.
- The verify concept is `make sure that`, never check, verify, confirm, validate, or ensure as verbs. Strict mode routes the rest with `references/strict-vocabulary.md`.
- Common swaps: however → but, therefore → as a result, since (= because) → because, perform → do, avoid → prevent, repeat → do again, acceptable → permitted, now → delete it.

