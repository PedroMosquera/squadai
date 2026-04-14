---
name: find-skills
description: Discover and install community skills from the Vercel skills ecosystem
---

# Find Skills

Search and install community-maintained skill definitions from the Vercel
skills ecosystem (skills.sh). Use this when the user needs capabilities not
covered by existing project skills.

## When to Use

- The user asks for domain-specific guidance you lack (e.g., "how do I write Terraform modules?").
- The user explicitly asks to find or install new skills.
- You need a workflow or protocol for an unfamiliar technology or practice.

## Commands

### Search for skills

```bash
npx skills find [query]
```

Returns matching skills from the registry. Use specific queries: prefer
`find react-testing` over `find testing`.

### Install a skill

```bash
npx skills install [skill-name]
```

Downloads the skill definition into the project. The installed skill is then
available for the AI agent to load in future sessions.

## Key Facts

- The registry at skills.sh contains 91K+ community skills across 40+ AI agents.
- Skills are static markdown files with structured instructions — no runtime dependency.
- `npx skills` requires Node.js but does not add project dependencies.
- agent-manager-pro does not depend on this ecosystem. The AI agent runs these
  commands at the user's request to extend its own capabilities.

## Protocol

1. Confirm the user wants to search for or install a skill.
2. Run `npx skills find [query]` and present the results.
3. Let the user choose which skill to install.
4. Run `npx skills install [skill-name]`.
5. Read the installed skill file to verify it was added correctly.
