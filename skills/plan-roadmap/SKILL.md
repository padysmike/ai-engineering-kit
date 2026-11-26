---
name: plan-roadmap
description: Turn the BDD scenarios into a phased roadmap (PoC, MVP, optionally the finished product) in the project tracker, for example Linear, through its MCP tools. Use after the scenarios are written and before implementation starts.
license: MIT
metadata:
  invocation: manual
---

# Plan the roadmap

## Pick the tracker
Use the tracker whose MCP tools are connected. Linear is the usual choice; Jira works too. If both are connected, ask which one to use. If none is connected, write the same plan to `docs/roadmap.md` and say so.

## Plan
1. Read `scenarios/` and `docs/adr/`. Look at what the tracker already holds, so that you do not create anything twice.
2. Group the feature files into phases. PoC proves the riskiest assumption. MVP is the smallest useful product. Add a third phase, finished product, only if there is work after the MVP.
3. Use one project per product. Ask before you create a project; the human may name an existing one. Each phase is:
   - in Linear: a milestone of the project;
   - in Jira: an epic in the project.
4. Plan one issue per feature file. It must fit in one pull request; if it does not, say so, so that the spec can be split. The issue holds:
   - the whole feature file, word for word, in a code block;
   - acceptance criteria;
   - the open questions that apply to it.
5. Mark dependencies with blocked-by relations.

## Create
6. If you will create more than 10 items, show the plan to a human first and wait. Count the project, the phases and the issues.
7. Create the items. If the tracker tools cannot create something, such as a milestone, a relation or a link, write it into the issue text and tell the human.
8. Report the key and link of every issue.
