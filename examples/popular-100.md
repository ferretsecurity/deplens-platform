# Popular repository sweep

`popular-100.yaml` selects 100 popular GitHub code repositories. The selection
starts from the first 200 repositories sorted by stars, excludes archived
repositories and forks, and filters out many reference lists and tutorials.
It is a curated sample, not an exact top-100 ranking. The recorded selection,
star counts, and scanner and sweep commits are in `popular-100-selection.json`.

The config expects a locally built scanner at `var/popular-100/bin/deplens`.
It uses six workers, a 30-minute per-repository timeout, and a 10 GiB free-space
reserve. Results and temporary checkouts stay under the ignored `var/` directory.

Run from the platform repository root with an authenticated GitHub CLI:

```sh
GITHUB_TOKEN="$(gh auth token)" var/popular-100/bin/deplens-sweep --config examples/popular-100.yaml
```

Sweep saves reports but does not upload them. Upload each saved `scan.json`
using the raw report endpoint described in the root README. Use the adjacent
`metadata.json` for repository identity, revision, branch, and scan time.

A sweep outcome of `failed` can include a usable report with partial or failed
extraction. Preserve that report and display its diagnostics. Do not treat an
HTTP upload success as proof of complete extraction. Compare uploaded source
and dependency counts, and the complete report endpoint, against the saved JSON.

The November 2026 local run keeps its import script, upload receipts, detailed
selection, batch summaries, and decision log under `var/popular-100/`. Its import
script uses `python-dotenv` from the run's virtual environment and does not edit
`.env`. The script labels incomplete analyses in scan metadata and verifies the
stored reports after upload. Existing receipts prevent duplicate imports.
