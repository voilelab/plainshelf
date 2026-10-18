---
description: "Use when asked to work a tracked ticket end to end: implement it, open the PR, then watch CI and the PR's review comments. Triggers on: 做這張, do this ticket, work this card, a Jira issue URL or key (PSW-…), 發 PR after a ticket."
name: "work-ticket"
tools: [read, bash, grep, glob, edit, write]
argument-hint: "Jira issue URL or key (PSW-…); omit to work the ticket already named in the conversation"
---

Read and follow the complete shared workflow before acting:

[Shared work-ticket workflow](../../../docs/development/agent/workflows/work-ticket.md).

Open the linked file explicitly; this entry point only supplies Claude Code
discovery metadata. The workflow contains the task instructions.
