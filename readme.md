# Myrmica Lobicornis - Update and Merge Pull Request

[![GitHub release](https://img.shields.io/github/release/containous/lobicornis.svg)](https://github.com/containous/lobicornis/releases/latest)
[![Build Status](https://travis-ci.com/containous/lobicornis.svg?branch=master)](https://travis-ci.com/containous/lobicornis)
[![Docker Build Status](https://img.shields.io/docker/build/containous/lobicornis.svg)](https://hub.docker.com/r/containous/lobicornis/builds/)

## Description

The bot:

- find all open PRs with a specific label (`marker.needMerge`)
- manage all the repositories of a user or an organization
- take one PR
    - with a specific label (`marker.mergeInProgress`) if exists
    - or the least recently updated PR
- verify:
    - GitHub checks (CI, ...)
    - "Mergeability"
    - Reviews (`minReview`)
- check if the PR need to be updated
    - if yes: rebase or merge with the base PR branch (ex: `master`)
- merge the PR with the chosen merge method. (`mergeMethod`, `marker.mergeMethodPrefix`)
- closes related issues and add the same milestone as the PR
- if errors occurs add a specific label (`marker.needHumanMerge`)
- if the description of the PR contains a co-author (`Co-authored-by: login <email@email.com>`) the co-author is set on the merge commit.

```yaml
Myrmica Lobicornis:
  -config string
        Path to the configuration file. (default "./lobicornis.yml")
  -h    Show this help.
  -server
        Run as a web server.
  -version
        Display version information.
```

`GITHUB_TOKEN`: GitHub token

Configuration file overview:

```yaml
github:
  # can be organization name or user name.
  user: foo
  # GitHub token.
  token: XXXX
  # optional only for GitHub Enterprise. 
  url: http://my-private-github.com

git:
  # Git user email.
  email: bot@example.com
  # Git user name.
  userName: botname
  # if true, use SSH instead HTTPS.
  ssh: false

server:
  # server port. (only used in server mode)
  port: 80

extra:
  # Debug mode.
  debug: false
  # Dry run mode.
  dryRun: true

# GitHub Labels.
markers:
  # Label use when a pull request need a lower minimal review as default.
  lightReview: bot/light-review
  # Label use when the bot update the PR (merge/rebase).
  mergeInProgress: status/4-merge-in-progress
  # Use to override default merge method for a PR.
  mergeMethodPrefix: bot/merge-method-
  # Use to manage merge retry.
  mergeRetryPrefix: bot/merge-retry-
  # Label use when the bot cannot perform a merge.
  needHumanMerge: bot/need-human-merge
  # Label use when you want the bot perform a merge.
  needMerge: status/3-needs-merge
  # Label use when a PR must not be merge.
  noMerge: bot/no-merge

# Merge retry configuration.
retry:
  # Time between retry.
  interval: 1m0s
  # Number of retry before failed.
  number: 1
  # Retry on PR mergeable state (GitHub information).
  onMergeable: false
  # Retry on GitHub checks (aka statuses).
  onStatuses: false

# default configuration used by all repositories of the user.
default:
  # Use GitHub repository configuration to check the need to be up-to-date.
  checkNeedUpToDate: false
  # Forcing need up-to-date. (checkNeedUpToDate must be false)
  forceNeedUpToDate: true
  # Default merge method. (merge|squash|rebase|ff)
  mergeMethod: squash
  # Minimal number of review (light review).
  minLightReview: 0
  # Minimal number of review.
  minReview: 1
  # Forcing PR to have a milestone.
  needMilestone: true
  # Add a comment in the pull request when an error occurs.
  addErrorInComment: false

# defines override of the default configuration by repository.
repositories:
  'foo/myrepo1':
    minLightReview: 1
    minReview: 3
    needMilestone: true
  'foo/myrepo2':
    minLightReview: 1
    minReview: 1
    needMilestone: false
```

## Examples
 
```bash
export GITHUB_TOKEN=xxx
lobicornis
```

```bash
export GITHUB_TOKEN=xxx
lobicornis -server
```

```bash
export GITHUB_TOKEN=xxx
lobicornis -config="./my-config.yml"
```

## What does Myrmica Lobicornis mean?

![Myrmica Lobicornis](http://www.antwiki.org/wiki/images/5/51/Myrmica_lobicornis_casent0172718_head_1.jpg)
