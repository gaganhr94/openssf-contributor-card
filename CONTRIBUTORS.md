# Contributing

Thanks for considering a contribution. This site displays GitHub contribution
data for [OpenSSF projects](https://openssf.org/projects/); it's not an
official OpenSSF property.

## Where to start

| You want to… | Where it lives |
|---|---|
| Add or remove an OpenSSF project | [`data/projects.yaml`](data/projects.yaml) |
| Add a service account to the bot filter | [`data/excluded_logins.yaml`](data/excluded_logins.yaml) |
| Opt yourself out of the site | [`CONTRIBUTORS_OPTOUT.md`](CONTRIBUTORS_OPTOUT.md) |
| Report a data issue or rendering bug | Open an issue on this repo |
| Propose a code change | Open a PR (see below) |

## Adding or removing a project

[`data/projects.yaml`](data/projects.yaml) is the source of truth for which
projects and repositories the site ingests. Each entry looks like:

```yaml
- slug: sigstore
  name: Sigstore
  maturity: graduated     # graduated | incubating | sandbox
  url: https://openssf.org/projects/sigstore/
  repos:
    - sigstore/cosign
    - sigstore/rekor
    - sigstore/fulcio
```

Rules of thumb:

- **List the project's actual code repos**, not its `.github`, governance,
  website, friends, or `*-installer` companion repos. Those add noise.
- Repos can live in any GitHub org — Sigstore is under `sigstore/`, GUAC under
  `guacsec/`, SLSA under `slsa-framework/`, etc. Use the canonical owner.
- One repo per line, format `owner/name`. Case must match GitHub.
- The slug must be unique. `maturity` is restricted to the three values above.

A push to `data/**` triggers a rebuild on the next CI run; you don't need to
do anything else.

## Excluding service accounts

`*[bot]` accounts (dependabot[bot], renovate[bot], github-actions[bot], …) are
filtered out automatically by a regex in `internal/config/config.go`. Service
accounts that *don't* have the `[bot]` suffix go in
[`data/excluded_logins.yaml`](data/excluded_logins.yaml). One login per line.

## Local development

```bash
# 1. Build
go build ./cmd/contribcard

# 2. Fetch data (needs a GitHub token)
export GH_TOKEN=$(gh auth token)
./contribcard fetch --project bomctl       # one project, ~10s
./contribcard fetch                        # everything, ~10–20 min cold

# 3. Render
./contribcard render --site-url http://localhost:8080

# 4. Preview
./contribcard serve --addr :8080
```

Useful subcommand flags are listed in `./contribcard --help`. The SQLite
database (`build.db`) and cached avatars (`.cache/avatars`) are gitignored;
deleting either forces a fresh fetch on the next run.

## Project layout

```
cmd/contribcard/         CLI entry (cobra)
internal/config/         YAML loaders + bot/exclusion matcher
internal/github/         GraphQL client
internal/store/          SQLite schema + queries
internal/etl/            Per-repo fetch orchestrator (commits + PRs + issues)
internal/avatar/         Avatar download + disk cache
internal/render/html/    html/template renderer
internal/render/og/      Open Graph image generator (fogleman/gg)
web/templates/           Page templates
web/static/              CSS, JS, logo, favicon
data/                    projects.yaml + excluded_logins.yaml
.github/workflows/       Build + deploy workflow
```

## Code change checklist

Before opening a PR:

1. `go build ./...` and `go vet ./...` should be clean.
2. If you touched the schema or data shape, bump the cache key prefix in
   `.github/workflows/build.yml` so CI rebuilds from scratch.
3. Render locally and check the rendered HTML / OG card before pushing —
   visual changes are hard to review in a diff alone.
4. Keep commits focused and the message a single descriptive line. The
   project history avoids long bodies and AI-attribution trailers.

## Reporting issues

When opening an issue, please include:

- The contributor login or repo affected, if applicable
- The URL of the page you're seeing the problem on
- Expected vs. actual behavior
- Browser / device for visual issues

## License

By contributing, you agree your contribution is licensed under
[Apache 2.0](LICENSE).
