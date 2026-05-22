## Project Memory Protocol

`docs/memory/` is this project's indexed memory store for decisions, learnings,
and incidents. Use it as follows:

**Before starting significant work:**
Use `/memory-search <query>` to find prior decisions relevant to your task.
Pass any findings as context before implementing or planning.

**During and after significant work:**
Use `/memory-add <note>` to capture decisions, key learnings, or incidents
while they are fresh. Notes land in `docs/memory/_inbox/` as drafts.

**For deeper research:**
The `@librarian` agent is available for multi-query investigation. Invoke it
with a plain query: `@librarian what do we know about <topic>`.

**Periodically:**
Use `/memory-promote` to graduate inbox drafts into permanent topic folders.
The `docs/memory/_inbox/` folder holds unprocessed notes — keep it tidy.

Never skip the memory-search step before architecture or API decisions.
