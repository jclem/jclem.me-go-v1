---
title: Labeling Public Repo PRs with GitHub Actions
slug: labeling-prs-on-public-github-repositories
published_at: 2020-09-22T00:00:00-04:00
published: true
summary: >-
  In this blog post, Jonathan Clem shares a simple method for labeling pull
  requests in public repositories based on files changed, made possible by a new
  event in GitHub Actions called `pull_request_target`. He explains the
  limitations of previous approaches and demonstrates how to create a workflow
  that runs on `pull_request_target` to label PRs according to a configuration
  file. This new method helps save Actions usage by running the labeling
  workflow only when needed.
---

In this post, I'm going to outline a simple way for public repositories to label
the pull requests based on files changed that has been enabled by a new event
recently shipped in GitHub Actions.

A very commonly-used workflow in GitHub Actions is one in which a repository
maintainer wants to apply a specific label to a pull request based on any paths
changed in that pull request. Until recently, this was relatively difficult to
pull off in a public repository, because workflows running from pull requests
coming from forks of public repositories do not get tokens that have write
access to the base repository. We do this for security reasons—any user could
fork a public repository and gain write access to it by changing the contents of
any workflow file in it.

Currently, many users solve this with actions like
[paulfantom/periodic-labeler](https://github.com/paulfantom/periodic-labeler),
an extremely clever action that is meant to run in a scheduled workflow. On a
schedule, this action looks for any unlabeled pull requests and labels them
according to a configuration file provided by the repository maintainer. This
approach works, but it wastes a good deal of users' Actions minutes, especially
if the rate of incoming pull requests is significantly lower than the configured
schedule's frequency.

Recently, a new event called `pull_request_target` was [added to GitHub
Actions](https://github.blog/2020-08-03-github-actions-improvements-for-fork-and-pull-request-workflows/).
This event runs in the same circumstances as the existing `pull_request` event,
except that it runs in the context of the base repository. This means that it
ignores any changes made to the workflow file in the pull request, and that the
`actions/checkout` action will also by default clone from the base branch
instead of the pull request's merge commit. Since this means that there is no
"untrusted" code in the workflow context, we provide a `GITHUB_TOKEN` to
workflows run from `pull_request_target` that has write access to the
repository.

Let's write a workflow that runs on `pull_request_target` and labels the PR
according to a configuration file:

```yaml
on: pull_request_target

jobs:
  label:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/labeler@v2
        with:
          repo-token: ${{ secrets.GITHUB_TOKEN }}
```

That's all there is to it! Since `pull_request_target` was added, we can use the
existing [`actions/labeler`](https://github.com/actions/labeler) action with the
same label configuration format shared by it and `paulfantom/periodic-labeler`
in order to label our PRs!

Now, we have a labeling workflow that runs _only_ when we need it to, so
hopefully some Actions users can use this knowledge to save on some of their
Actions usage.
