---
description: Search project memory for prior decisions, learnings, and incidents
---

Search project memory for entries matching the query.

The query is: `$ARGUMENTS`

If no query was provided, ask the user: "What are you looking for in project memory?"

Run:

```bash
squadai memory search "$ARGUMENTS" --json
```

Parse the JSON results and format them as a numbered list:

```
1. [0.82] docs/memory/auth/jwt-decision.md: We chose HS256 because the service is stateless…
2. [0.74] docs/memory/infra/db-migration.md: Migrations run at startup via golang-migrate…
3. [0.61] docs/memory/_inbox/payment-retry.md: Draft — exponential backoff for payment retries…
```

Each line: `<rank>. [<score>] <path>: <first non-empty line of the note body>`

If results are found, offer: "Would you like me to read any of these in full?"
Read only the files the user requests (one or two at most).

If no results are returned, say:
`No entries found in docs/memory/ matching "<query>". Consider running /memory-add to capture something new.`
