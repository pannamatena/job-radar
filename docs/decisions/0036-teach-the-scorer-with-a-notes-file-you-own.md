## ADR-0036: Teach the scorer with a notes file you own, not a provider's hosted memory

**Status:** Accepted (the decision is made; the feature is not built yet)

**Context:** Today job-radar records your 👍/👎 votes, but nothing learns from them. You improve the scorer by hand: change your profile or rubric, then measure it with `eval` ([ADR-0030](0030-tune-on-labelled-cases-one-change-at-a-time.md)). Over months that needs to become less work. The ways to give a scorer memory are not equal:
- A model provider's hosted agent platform can keep memory for an agent between runs, in the provider's own storage.
- The same lessons can be written in plain text and added to the prompt.
- Past decisions can be shown as examples in every prompt.

**Options considered:**
1. **Manual only.** Safe and measurable, but every lesson costs the user effort.
2. **A notes file in the user's own config (chosen).** job-radar reads your votes and your notes and *proposes* short rubric lines, for example "roles whose core work is infrastructure reliability score at most 2". You approve or edit each one. `eval` measures the change before it is kept. The notes are a plain-text file that is added to the scorer's prompt.
3. **Provider-hosted agent memory.** The agent keeps and edits its own notes in the provider's storage.
4. **Past decisions as examples in every prompt.** Direct, but it makes every scoring more expensive and it breaks the evaluation, because the examples would be the cases being measured.

**Decision:** Option 2. What the scorer has learned is a plain-text file in the user's own config, which the user can read, edit, delete and keep in git. Changes are proposed by job-radar and approved by the user, and the prompt stays fixed between changes so `eval` can say whether a note helped.

The open-source tool does not depend on one provider's platform for this. Because the notes are just text added to the prompt, they work with any model provider that job-radar supports, and the user decides which one. A provider's hosted agent features may still be worth exploring later as an optional extension, outside the open-source core, but nothing in the core will need them.

**Trade-offs:**
- job-radar has to build the proposal and approval flow itself, where a hosted memory would come ready-made.
- The user approves each note, so learning is slower than if the agent wrote its own memory unattended. That is the point: every change is visible and measurable.
- The notes only help the AI scorer. The free rules scorer is unchanged.
- Notes come from your votes and your own words, but a proposal may draw on text from job ads, and ads are untrusted ([ADR-0020](0020-untrusted-job-ads-can-t-make.md)). The approval step is the guard: nothing reaches the prompt without the user reading it.
- Notes can grow and contradict each other. The file stays short and readable on purpose, and `eval` is how a bad note is found.

**Why:** the project's promise is that everything personal stays in files the user owns ([ADR-0014](0014-open-source-with-all-personal-data.md)), that it works without a particular vendor, and that changes to the scorer are measured, not assumed. A notes file keeps all three. A provider's memory would be quicker to start with, but it would put the user's preferences in someone else's storage, tie the tool to one platform, and make the prompt change invisibly between runs.

**Revisit if:** the notes file becomes too large to read, or several providers offer a common way to store agent memory that the user can export as text.

**In one sentence:** "What job-radar learns about you is a text file you own, you approve every line, and `eval` checks it helps, so it works with any provider."
