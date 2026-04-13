---
name: pr-description
description: Generate pull request descriptions from diff analysis
---

# PR Description Skill

Generate a clear, reviewer-friendly pull request description by analyzing
the diff and commit history. Follow this protocol for consistent output.

## Steps

1. **Analyze the diff**: Read all changed files to understand the scope.
   - What files were added, modified, or deleted?
   - What is the primary purpose of the change?
   - Are there multiple logical changes bundled together?

2. **Identify the change type**: Classify the PR.
   - **Feature**: New functionality or capability.
   - **Fix**: Bug fix or error correction.
   - **Refactor**: Code restructuring without behavior change.
   - **Docs**: Documentation updates.
   - **Test**: Test additions or improvements.
   - **Chore**: Build, CI, dependency, or tooling changes.

3. **Write the summary**: Create 1-3 bullet points.
   - Focus on what changed and why, not how.
   - Lead with the most important change.
   - Keep each bullet to one sentence.

4. **List the changes**: Group modifications by area.
   - Group related files together (e.g., "API endpoints", "Database models").
   - Describe what changed in each group, not file-by-file.
   - Note any new dependencies or configuration changes.

5. **Describe testing**: Explain what was verified.
   - What tests were added or updated?
   - How can a reviewer verify the change manually?
   - Are there any known limitations or untested scenarios?

6. **Note breaking changes**: Flag anything that affects existing behavior.
   - API changes (new required parameters, removed endpoints).
   - Database migrations or schema changes.
   - Configuration format changes.
   - Dependency version bumps with breaking changes.

## Output Format

```markdown
## Summary
- <1-3 bullet points describing the change>

## Changes
- <grouped description of modifications>

## Testing
- <what was tested and how to verify>

## Breaking Changes
- <list any breaking changes, or "None">
```

Keep the description concise and factual. Avoid subjective language.
Omit the Breaking Changes section if there are none.
