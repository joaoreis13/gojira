# Graph Report - gojira  (2026-10-18)

## Corpus Check
- 31 files · ~23,630 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 2 file(s) not represented in the graph (top: (none) 2)

## Summary
- 345 nodes · 1005 edges · 10 communities (9 shown, 1 thin omitted)
- Extraction: 91% EXTRACTED · 9% INFERRED · 0% AMBIGUOUS · INFERRED: 93 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `0f27293a`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- oauth.go
- profile_cmd.go
- format.go
- gojira
- testing.T
- checklist_cmd.go
- checklist_cmd_test.go
- checklist.go
- gojira — Jira Cloud CLI for agents
- github.com/joaoreis13/gojira

## God Nodes (most connected - your core abstractions)
1. `Parse()` - 31 edges
2. `newFakeJira()` - 25 edges
3. `newOpts()` - 24 edges
4. `jsonResp()` - 19 edges
5. `runChecklistGet()` - 18 edges
6. `Doc` - 15 edges
7. `Profile` - 15 edges
8. `Status` - 14 edges
9. `checklistOpts` - 14 edges
10. `runChecklistSet()` - 14 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `Execute()`  [EXTRACTED]
  cmd/gojira/main.go → internal/cli/root.go
- `TestParseStatusAliases()` --calls--> `ParseStatus()`  [INFERRED]
  internal/checklist/checklist_test.go → internal/checklist/checklist.go
- `TestExtractTextRejectsUnknownShape()` --calls--> `ExtractText()`  [INFERRED]
  internal/checklist/checklist_test.go → internal/checklist/checklist.go
- `TestExtractTextShapes()` --calls--> `ExtractText()`  [INFERRED]
  internal/checklist/checklist_test.go → internal/checklist/checklist.go
- `TestChecklistFieldsListsCandidates()` --calls--> `runChecklistFields()`  [INFERRED]
  internal/cli/checklist_cmd_test.go → internal/cli/checklist_cmd.go

## Import Cycles
- None detected.

## Communities (10 total, 1 thin omitted)

### Community 1 - "oauth.go"
Cohesion: 0.07
Nodes (43): AccessibleResource, persistingTokenSource, Status, storedToken, fakeTokenSource, legacyConfig, EnsureCloudID(), GetStatus() (+35 more)

### Community 2 - "profile_cmd.go"
Cohesion: 0.08
Nodes (11): Response, main(), Execute(), installSkill(), resolveSkillDest(), TestInstallSkillOverwritesExistingInstall(), TestInstallSkillWritesExpectedFiles(), TestResolveSkillDest() (+3 more)

### Community 3 - "format.go"
Cohesion: 0.10
Nodes (23): parseQuery(), readBody(), renderResponse(), TestParseQuery(), TestParseQueryRejectsMissingEquals(), TestReadBodyEmpty(), TestReadBodyFile(), TestReadBodyInline() (+15 more)

### Community 4 - "gojira"
Cohesion: 0.06
Nodes (31): Giving yourself the on-machine skill (recommended, one-time), graphify, Guardrails, Installing, Using gojira to talk to Jira, What this is, Where to look up operation-specific details, Working on gojira's own source (+23 more)

### Community 5 - "testing.T"
Cohesion: 0.09
Nodes (52): Parse(), TestAddAppendsItemAndPreservesExisting(), TestAddHeadingLevelCountsLeadingHashesOnly(), TestAddRejectsControlAndLineSeparatorChars(), TestAddRejectsNewlines(), TestAddToEmptyChecklist(), TestExtractTextRejectsUnknownShape(), TestExtractTextShapes() (+44 more)

### Community 6 - "checklist_cmd.go"
Cohesion: 0.19
Nodes (24): checklistItemJSON, checklistOpts, checklistReportJSON, jiraField, currentChecklistOpts(), isChecklistsField(), listFields(), mutateChecklist() (+16 more)

### Community 7 - "checklist_cmd_test.go"
Cohesion: 0.23
Nodes (29): fakeJira, recordedRequest, runChecklistGet(), issueWithChecklist(), jsonResp(), newFakeJira(), newOpts(), TestChecklistAddAppendsAndWrites() (+21 more)

### Community 8 - "checklist.go"
Cohesion: 0.14
Nodes (15): adfNode, Item, Kind, Line, adfText(), ambiguousError(), ExtractText(), extractText() (+7 more)

### Community 9 - "gojira — Jira Cloud CLI for agents"
Cohesion: 0.29
Nodes (6): Convenience commands (thin wrappers, prefer these when they fit), First: check it's installed and authenticated, gojira — Jira Cloud CLI for agents, Guardrails, Making calls, Where to find operation details

## Knowledge Gaps
- **1 isolated node(s):** `github.com/joaoreis13/gojira`
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 63 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **1 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Doc` connect `checklist.go` to `testing.T`, `checklist_cmd.go`?**
  _High betweenness centrality (0.041) - this node is a cross-community bridge._
- **Are the 24 inferred relationships involving `Parse()` (e.g. with `TestAddAppendsItemAndPreservesExisting()` and `TestAddHeadingLevelCountsLeadingHashesOnly()`) actually correct?**
  _`Parse()` has 24 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/joaoreis13/gojira` to the rest of the system?**
  _1 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `oauth.go` be split into smaller, more focused modules?**
  _Cohesion score 0.06946386946386947 - nodes in this community are weakly interconnected._
- **Why does `Parse()` connect `testing.T` to `checklist.go`, `checklist_cmd.go`, `checklist_cmd_test.go`?**
  _High betweenness centrality (0.033) - this node is a cross-community bridge._
- **Should `profile_cmd.go` be split into smaller, more focused modules?**
  _Cohesion score 0.08181818181818182 - nodes in this community are weakly interconnected._
- **Why does `Status` connect `checklist.go` to `checklist_cmd.go`?**
  _High betweenness centrality (0.020) - this node is a cross-community bridge._