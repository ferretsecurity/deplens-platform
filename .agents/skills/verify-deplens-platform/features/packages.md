# Package search

Run the reusable LiteLLM journey through the disposable supervisor:

```sh
VERIFY_DRIVER="$PWD/.agents/skills/verify-deplens-platform/scripts/packages.mjs" .agents/skills/verify-deplens-platform/scripts/verify.py
```

The driver signs in, issues a scanner token, uploads nine PyPI usages through the public API, and checks the package search page. It covers recorded versions, manifest ranges, equality, exclusions, prereleases, page links, search resetting the page, clear, and invalid version rejection. Screenshots and JSON readbacks are saved for each version. Fixtures include a recorded `1.100.4` with a broad constraint to prove the recorded version takes precedence.

Constraint-only matches use the reported ecosystem. `pip` and `pypi` share PyPI rules while keeping their displayed values. Unknown ecosystems support identical recorded versions only. Prereleases follow the ecosystem parser's constraint rules. Empty or malformed constraints do not match. Version searches count all matching candidates before reporting pagination and retain only the requested page in application memory.

This fixture uses CLI inventory. Store integration tests separately cover Socket and Snyk recorded versions and tenant isolation. It does not verify live provider imports or the original development app.
