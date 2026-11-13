# Vulnerability assessments

Run this journey when changing manifest-to-lockfile relationships, range assessment, or `/app/packages/assess`.

Build the sibling CLI and run the disposable supervisor from the platform root:

```sh
mkdir -p var/verification
go -C ../deplens build -o "$PWD/var/verification/deplens-assessment" ./cmd/deplens
VERIFY_SCANNER="$PWD/var/verification/deplens-assessment" VERIFY_DRIVER="$PWD/.agents/skills/verify-deplens-platform/scripts/assessments.mjs" .agents/skills/verify-deplens-platform/scripts/verify.py
```

The driver creates fixture repositories, runs that CLI, and uploads its real reports. It checks missing, fixed, vulnerable, competing, broken, and shared-workspace lockfiles. It also checks exact package identity, every API result page, invalid range rejection, form submission, and results after page reload.

A pass requires `result.json`, successful assertions in `drive.log`, and complete cleanup in `cleanup.json`. Evidence includes the actual scanner reports, assessment API results, a screenshot, an ARIA snapshot, and a browser trace. `source-state.json` identifies both repositories and the scanner binary hash. The `source` directory preserves the tested code, including untracked files.

These fixtures exercise npm. Ecosystem range semantics and further workspace boundaries have unit tests. Provider observations have store integration coverage; this driver does not contact Socket or Snyk.
