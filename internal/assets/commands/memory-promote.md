---
description: Promote inbox notes to permanent topic folders in project memory
---

Review inbox notes and promote them to permanent topic folders.

First, list inbox contents:

```bash
squadai memory status
```

Show the user the inbox files. If the inbox is empty, say:
`The inbox is empty — nothing to promote.`

Otherwise, ask:
1. Which note(s) to promote (by path or number)
2. Which category/topic folder to promote each one into
   (e.g. `auth`, `infra`, `decisions`, `incidents`)

Then run for each chosen note:

```bash
squadai memory promote <path> --category <category>
```

Report the new path after each promotion. After all promotions are done,
remind the user that the index is updated automatically by the promote command.
