---
description: "Delete the code comments the comment policy does not keep and fix the code they excused. Run manually with /no-comments."
name: "no-comments"
disable-model-invocation: true
tools: [read, bash, grep, glob, edit, agent]
argument-hint: "Optional: files or directories; omit to use the current diff against origin/dev"
---

Read and follow the complete shared workflow before acting:

[Shared no-comments workflow](../../../docs/development/agent/workflows/no-comments.md).

Open the linked file explicitly; this entry point only supplies Claude Code
discovery metadata. The workflow contains the task instructions.
