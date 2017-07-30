# Brahma - Update and Merge Pull Request

```bash
Brahma, God of Creation.
Update and Merge Pull Request from GitHub.


Usage: brahma [--flag=flag_argument] [-f[flag_argument]] ...     set flag_argument to flag(s)
   or: brahma [--flag[=true|false| ]] [-f[true|false| ]] ...     set true/false to boolean flag(s)

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

## Examples
 
```bash
brahma --debug --ssh -t xxxxxxxxxxxxx -o containous -r traefik --min-review=3
```

```bash
brahma --debug --ssh -t xxxxxxxxxxxxx -o containous -r traefik --min-review=3 \
    --marker.merge-in-progress="merge-pending" \
    --marker.need-human-merge="merge-fail" \
    --marker.need-merge="merge"
```
