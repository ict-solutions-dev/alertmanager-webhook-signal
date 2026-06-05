# Contributing

Contributions are welcome. This document outlines the process and conventions.

## Upstream

This is a fork of [`schlauerlauer/alertmanager-webhook-signal`](https://github.com/schlauerlauer/alertmanager-webhook-signal).
Changes to the **core application logic** are best contributed upstream where appropriate. This fork focuses on
ICT Solutions' packaging, image publishing, and deployment conventions.

## Getting Started

1. Fork the repository
2. Create a feature branch from `develop`
3. Make your changes
4. Submit a pull request against `develop`

## Branch Strategy

- `develop` — active development, triggers `edge` image builds
- `main` — not used for direct development
- Version tags (`v*`) — trigger release image builds

## Commit and PR Conventions

This project uses [Conventional Commits](https://www.conventionalcommits.org/). PR titles are validated via [prlint](https://github.com/ewolfe/prlint).

Valid prefixes:

| Prefix | Usage |
| --- | --- |
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `chore` | Maintenance, dependencies |
| `refactor` | Code restructuring without behavior change |
| `perf` | Performance improvement |
| `test` | Adding or updating tests |
| `style` | Formatting, whitespace |
| `lang` | Translations |

Examples:

```
feat: add support for per-alert message templates
fix: handle missing recipients label gracefully
docs: update configuration reference
```

## Code Style

- Go code is formatted with `gofmt` and must pass `go vet -mod=vendor ./...`
- Use full, descriptive variable names — no single/two-letter abbreviations
- Comments, error messages, and identifiers in English

## Dependencies

Dependencies are vendored. If you change `go.mod`, run `go mod tidy && go mod vendor` and commit the updated
`vendor/` tree so the Docker build (which uses `-mod=vendor`) stays reproducible.

## Dockerfile & Entrypoint Changes

- The Dockerfile is linted with [hadolint](https://github.com/hadolint/hadolint) in CI
- Use `# hadolint ignore=DLXXXX` for intentional rule suppressions, with a comment explaining why
- The entrypoint ([`assets/01-init.sh`](assets/01-init.sh)) should be `shellcheck` clean
- Configuration is generated from environment variables at startup; keep the entrypoint and the README's
  environment-variable table in sync when adding options

## Review Process

All pull requests require review from [@jozefrebjak](https://github.com/jozefrebjak) (see [`CODEOWNERS`](.github/CODEOWNERS)).
