# OpenSSF Contributor Card

A community-built site that generates personalized "contributor cards" for
people who've shipped code to [OpenSSF projects](https://openssf.org/projects/).
Each contributor gets a static page with rich social-media previews
(personalized OG image, title, description) so links shared on
Twitter/LinkedIn/Slack actually market the OpenSSF community.

Inspired by [`cncf/contribcard`](https://github.com/cncf/contribcard) but
implemented in Go and explicitly designed so that **shared URLs preview
correctly** — see "Why a static-per-contributor build" below.

> Not an official OpenSSF site. See [`CONTRIBUTORS_OPTOUT.md`](CONTRIBUTORS_OPTOUT.md)
> if you'd like your card removed.

## How it works

1. **Source of truth.** [`data/projects.yaml`](data/projects.yaml) lists every
   OpenSSF project (graduated, incubating, sandbox) and the GitHub repos that
   constitute it.
2. **Daily ETL.** GitHub Actions runs `contribcard build` once a day. It calls
   the GitHub GraphQL API (~1500 requests on a cold rebuild, <200 on
   incremental days), aggregates per-contributor commit counts into a SQLite
   file persisted across runs via Actions cache, and generates static output.
3. **Static site.** The output is a directory of HTML pages
   (one per contributor at `/c/<login>.html`), JPEG OG cards
   (one per contributor at `/og/<login>.jpg`), and a search index page at `/`.
4. **Deploy.** GitHub Pages serves the output; no runtime backend.

## Stack

- **Go** for the CLI/ETL/renderer (matches OpenSSF's own ecosystem — Scorecard,
  Sigstore, GUAC are all Go)
- **`modernc.org/sqlite`** (pure Go, no CGO) for the build-time data store
- **`fogleman/gg`** for OG image rendering
- **`html/template`** for the per-contributor pages
- Vanilla JS (~3KB) for the index search

## Why a static-per-contributor build

[`cncf/contribcard`](https://github.com/cncf/contribcard) is a SPA: every
contributor URL returns the same HTML shell with the same OG metadata, and
the actual card is rendered client-side. That breaks link previews on every
social platform — scrapers don't run JS — so CNCF works around it with a
"Download as PNG" button that users have to attach manually to their posts.

Since OpenSSF's whole goal here is community marketing, link previews matter
more than browse-page slickness. So this project diverges and renders one real
HTML page per contributor with personalized OG tags, plus a personalized OG
image, all at build time.

## Running locally

```bash
# 1. Build the CLI
go build ./cmd/contribcard

# 2. Fetch data from GitHub (needs a token)
export GH_TOKEN=$(gh auth token)
./contribcard fetch                                   # all projects
./contribcard fetch --repo bomctl/bomctl              # one repo, fast iteration

# 3. Render to dist/
./contribcard render --site-url http://localhost:8080

# 4. Serve and browse
./contribcard serve --addr :8080
open http://localhost:8080
```

## CLI

| Command | What it does |
|---|---|
| `fetch` | Sync projects.yaml → SQLite, then GraphQL → SQLite. Incremental after first run. |
| `render` | Render HTML + OG images from SQLite into `dist/`. |
| `build` | `fetch` then `render`. CI uses this. |
| `serve` | Serve `dist/` over HTTP for local preview. |

Useful flags:
- `--db PATH` — SQLite path (default `build.db`)
- `--projects PATH` — projects YAML (default `data/projects.yaml`)
- `--exclusions PATH` — excluded logins YAML (default `data/excluded_logins.yaml`)
- `--site-url URL` — absolute site URL used in OG tags / share links
- `--avatar-cache DIR` — where to cache downloaded avatars
- `--skip-fetch` (on `fetch`) — only sync YAML, don't hit GitHub
- `--skip-og` (on `render`) — skip OG image generation for fast iteration
- `--dry-run` (on `build`) — used by CI on PR builds to validate without deploying

## Project list

To add or remove an OpenSSF project, edit
[`data/projects.yaml`](data/projects.yaml). Each entry needs a slug, name,
maturity (`graduated` / `incubating` / `sandbox`), URL, and the list of repos
that constitute the project (often spread across multiple GitHub orgs — see
the existing entries for examples). Push to `main` and the workflow will
rebuild against the new list.

## Bot filtering

`*[bot]` accounts are dropped automatically. Service accounts that don't have
the `[bot]` suffix (like `step-security-bot`, `mergify`) live in
[`data/excluded_logins.yaml`](data/excluded_logins.yaml).

## GitHub auth

The workflow uses `secrets.GITHUB_TOKEN` by default (1000 GraphQL req/hr per
repo). Cold rebuilds — when you've cleared the SQLite cache or added many
new repos — can exceed that. To raise the ceiling to 5000/hr, create a
fine-grained PAT with `Metadata: read` + `Contents: read` and add it as
`secrets.CONTRIBCARD_GH_TOKEN`. The workflow auto-prefers it when present.

## License

Apache 2.0 — see [`LICENSE`](LICENSE).
