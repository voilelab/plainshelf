---
description: "Use when project docs (README.md, docs/) are stale relative to code changes. Triggers on: update docs, sync docs, docs are out of date, document this feature."
name: "update-docs"
tools: [read, bash, grep, glob, edit]
argument-hint: "Optional: doc path, feature, or commit range"
---

Read and follow the complete shared workflow before acting:

[Shared update-docs workflow](../../../docs/development/agent/workflows/update-docs.md).

Open the linked file explicitly; this entry point only supplies Claude Code
discovery metadata. The workflow contains the task instructions.
