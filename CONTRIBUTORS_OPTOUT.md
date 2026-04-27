# Opt out of OpenSSF Contributor Card

This site shows public contributor data sourced from the GitHub API for
[OpenSSF projects](data/projects.yaml). If you do not want a card generated
for your GitHub account, you have two equally good options:

## Option 1: Open a pull request (preferred)

Add your GitHub login to [`data/excluded_logins.yaml`](data/excluded_logins.yaml)
under the `excluded:` list:

```yaml
excluded:
  - your-github-login
```

You don't need to explain why. The next build will exclude you and remove your
existing card from the deployed site.

## Option 2: Open an issue

If you don't want to file a PR, [open an issue][issues] titled "Opt out: <your-login>".
A maintainer will land the change.

[issues]: https://github.com/ghr/openssf-contributor-card/issues/new

## What gets removed

After the next scheduled build (daily at 06:00 UTC) or a manually triggered
build, your contributor page (`/c/<login>.html`), your OG card image
(`/og/<login>.jpg`), and your tile on the index will all disappear.

## What we display

For each contributor we show only data already public on github.com:
- Your GitHub login and (if set) public display name
- Your public profile bio
- Your avatar (cached on our CDN to avoid hammering avatars.githubusercontent.com)
- Your aggregated commit counts across OpenSSF project repos

We do not display email addresses, private commits, or anything that requires
authentication to see on github.com.
