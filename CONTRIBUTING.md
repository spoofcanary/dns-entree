# Contributing to dns-entree

Thanks for your interest in contributing. This document covers the basics for
building, testing, and submitting changes.

## Prerequisites

- Go 1.26 or newer
- Git

## Building

```
go build ./...
```

The CLI binary builds with:

```
go build -o bin/entree ./cmd/entree
```

## Testing

```
go test ./...
```

Integration tests that hit live provider APIs are gated behind environment
variables and skip cleanly when credentials are absent.

## Linting

Before opening a PR, run:

```
go vet ./...
gofmt -l .
```

`gofmt -l .` must produce no output. `staticcheck ./...` is recommended but
optional.

## Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation only
- `refactor:` code change that neither fixes a bug nor adds a feature
- `test:` adding or correcting tests
- `chore:` tooling, dependencies, build config

Scope is optional but encouraged: `feat(providers/cloudflare): add proxied flag`.

## Pull Request Workflow

1. Fork the repository and create a feature branch off `main`
2. Make focused commits (one logical change per commit)
3. Run `go build ./... && go test ./... && go vet ./... && gofmt -l .`
4. Open a PR against `main` with a clear description of the change and rationale
5. Address review feedback with additional commits (do not force-push during review)

## Branch Strategy

- `main` is always shippable
- Feature work happens on short-lived branches off `main`
- No long-lived development branches

## Reporting Security Issues

Do not open public issues for security problems. See [SECURITY.md](SECURITY.md).
