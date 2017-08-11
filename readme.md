# Myrmica Lobicornis - Update and Merge Pull Request

[![Build Status](https://travis-ci.org/containous/lobicornis.svg?branch=master)](https://travis-ci.org/containous/lobicornis)

```bash
Myrmica Lobicornis:  Update and Merge Pull Request from GitHub.

Usage: lobicornis [--flag=flag_argument] [-f[flag_argument]] ...     set flag_argument to flag(s)
   or: lobicornis [--flag[=true|false| ]] [-f[true|false| ]] ...     set true/false to boolean flag(s)

Flags:
    --debug                    Debug mode.                                          (default "false")
    --dry-run                  Dry run mode.                                        (default "true")
    --marker                   GitHub Labels.                                       (default "true")
    --marker.merge-in-progress Label use when the bot update the PR (merge/rebase). (default "status/4-merge-in-progress")
    --marker.need-human-merge  Label use when the bot cannot perform a merge.       (default "bot/need-human-merge")
    --marker.need-merge        Label use when you want the bot perform a merge.     (default "status/3-needs-merge")
    --merge-method             Default merge method.(merge|squash|rebase)           (default "squash")
    --merge-method-prefix      Use to override default merge method for a PR.       (default "bot/merge-method-")
    --min-review               Minimal number of review.                            (default "1")
-o, --owner                    Repository owner. [required]
-r, --repo-name                Repository name. [required]
    --ssh                      Use SSH instead HTTPS.                               (default "false")
-t, --token                    GitHub Token. [required]
-h, --help                     Print Help (this message) and exit
```

## Description

The bot:
- find all open PRs with a specific label (`--marker.need-merge`)
- take one PR
    - with a specific label (`--marker.merge-in-progress`) if exists
    - or the least recently updated PR
- verify:
    - GitHub checks (CI, ...)
    - "Mergeability"
    - Reviews (`--min-review`)
- check if the PR need to be updated
    - if yes: rebase or merge with the base PR branch (ex: `master`)
- merge the PR with the chosen merge method. (`--merge-method`, `--merge-method-prefix`)
- closes related issues and add the same milestone as the PR
- if errors occurs add a specific label (`--marker.need-human-merge`)

## Examples
 
```bash
lobicornis --debug --ssh -t xxxxxxxxxxxxx -o containous -r traefik --min-review=3
```

```bash
lobicornis --debug --ssh -t xxxxxxxxxxxxx -o containous -r traefik --min-review=3 \
    --marker.merge-in-progress="merge-pending" \
    --marker.need-human-merge="merge-fail" \
    --marker.need-merge="merge"
```
