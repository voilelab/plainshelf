---
description: "Cut machine-written tells from English prose: docs, agent guidance, code comments, commit messages, PR titles and bodies. Run manually with /unslop."
name: "unslop"
disable-model-invocation: true
tools: [read, bash, grep, glob, edit]
argument-hint: "Optional: file path, doc, or commit range; omit to edit the prose in the current diff"
---

Read and follow the complete shared workflow before acting:

[Shared unslop workflow](../../../docs/development/agent/workflows/unslop.md).

Open the linked file explicitly; this entry point only supplies Claude Code
discovery metadata. The workflow contains the task instructions.
