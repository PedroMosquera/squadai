---
description: Capture a decision, learning, or incident note into project memory
---

Capture a new note into project memory.

If `$ARGUMENTS` is provided, use it as the note text. Otherwise ask the user:
"What would you like to remember? (decision, learning, incident, etc.)"

Once you have the note text, run:

```bash
squadai memory add "<note text>"
```

Print the saved path that the command reports. If the path is inside
`docs/memory/_inbox/`, remind the user they can run `/memory-promote` to
graduate it to a permanent topic folder when ready.
