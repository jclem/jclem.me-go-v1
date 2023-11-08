---
title: GitHub Actions for Elixir
slug: 2019-01-26-github-actions-for-elixir
published_at: 2019-01-26T00:00:00-04:00
published: true
summary: >-
  In this blog post, Jonathan Clem demonstrates how to use GitHub Actions in an
  Elixir project, creating a simple workflow that tests and checks the
  formatting of code. He walks through the process of creating an action that
  can run mix commands, and then builds a workflow using the visual editor. The
  post also covers adding actions for fetching dependencies, running tests, and
  checking code formatting. By the end, readers will have a better understanding
  of how to integrate GitHub Actions into their Elixir projects and improve
  their development workflow.
---

In this post, I'm going to demonstrate how to use GitHub Actions in an Elixir
project. By the end of it, we'll have a simple workflow that tests and checks
formatting of our code.

If you want to skip ahead, you can see the workflow file I'll be building in
this post [on GitHub here][workflow_file].

For this example, I'm going to use my [EncryptedField][encrypted_field] package.
It provides an [Ecto][ecto] type that is encrypted when stored in the database,
but decrypted at runtime.

Most other posts I've seen on Actions use workflow code, so I'm going to walk
through this using the visual editor this time.

## Creating the Action

Before I create a workflow file, I need an action that can run mix commands for
me. Thankfully, I have already [created one][mix_action], but I'm going to walk
through how it's built, anyway.

The [Dockerfile][mix_action_dockerfile] for the action is relatively
straightforward. It is based on the "elixir" image, and makes just a couple of
small changes:

```dockerfile
FROM elixir

# Set MIX_HOME outside of the user home directory.
ARG MIX_HOME=/.mix
ENV MIX_HOME=$MIX_HOME

# Set MIX_ENV to "dev" by default.
ARG MIX_ENV=dev
ENV MIX_ENV=$MIX_ENV

# Install rebar and hex locally in MIX_HOME.
RUN mix local.rebar --force
RUN mix local.hex --force

# Copy our entrypoint script.
COPY entrypoint.sh /entrypoint.sh

# Set the entrypoint script as our entrypoint.
ENTRYPOINT ["/entrypoint.sh"]
```

In this Dockerfile, we're first overriding the default `$MIX_HOME` environment
variable, which is usually at `$HOME/.mix`. The reason we are doing this is that
GitHub Actions provides the user home at runtime, and that would remove the
contents of `$MIX_HOME` created at build time when we run things like `mix
local.rebar`.

You'll also notice that rather than setting `ENTRYPOINT ["mix"]`, the action
instead has an executable "entrypoint.sh" file, which we set as the entrypoint:

```bash
#!/bin/sh

sh -c "mix $*"
```

This entrypoint script ensures that if a user of our action passes shell
variables in their `args`, the variables will be properly expanded. For example,
an entrypoint ensures that an action such as the following will do what the user
expects, and expand their `MY_SECRET` to its value when the action runs:

```hcl
action "Something Secret" {
  uses = "jclem/actions/mix@master"
  args = "use_secret $MY_SECRET"
  secrets = ["MY_SECRET"]
}
```

So that's an overview of how my Mix action works. To recap:

1. Build from the Elixir image.
2. Set `$MIX_HOME` so that it isn't overridden at runtime.
3. Use an entrypoint script for shell variable expansion.

Next I'm going to create a workflow in a project of mine, and use this action.

## Creating the Workflow

First, I'm going to create a new ".github/main.workflow" file in the repository.
You'll notice that when I enter the "main.workflow" file name, GitHub prompts me
to switch over to the visual editor. I also switch to full screen mode in order
to get a better view of the workflow.

<figure class="vid-figure">
  <video controls>
    <source
      src="https://jclem.nyc3.cdn.digitaloceanspaces.com/2019-01-26-github-actions-for-elixir/create-workflow.mp4"
      type="video/mp4"
    />
  </video>

  <figcaption>A user creating a "main.workflow" file using a visual editor</figcaption>
</figure>

By default, workflows trigger on the "push" event, so I've left that as it is in
this workflow.

## Adding a Deps Action

Next, we'll create an action. First, I rename the workflow, giving it short name
that describes what it's for. This one is pretty straightforward—"Test and Check
Formatting".

<figure class="vid-figure">
  <video controls>
    <source
      src="https://jclem.nyc3.cdn.digitaloceanspaces.com/2019-01-26-github-actions-for-elixir/get-deps.mp4"
      type="video/mp4"
    />
  </video>

  <figcaption>A user adding a "mix deps.get" action to the workflow</figcaption>
</figure>

After I rename the workflow, I drag downwards from the blue dot at the bottom of
the workflow node and release to create a new action. There isn't currently an
"official" Mix action yet, so I'm using the one I created before. In this case,
I've specified "master" as the Git ref to target, but it's generally safer to
use a full commit SHA so that you know exactly what version your action is
running.

After I've created the action node, I rename it and set "args" to "deps.get".
Remember that the entrypoint script for the mix action calls `sh -c "mix $*"`,
so this action will essentially run `mix deps.get`. This will fetch our
dependencies before we run our tests.

## Adding a Test Action

Adding an action for running the tests is relatively straightforward. We use the
mix action again, except this time our "args" is simply "test", for running `mix
test`.

<figure class="vid-figure">
  <video controls>
    <source
      src="https://jclem.nyc3.cdn.digitaloceanspaces.com/2019-01-26-github-actions-for-elixir/test.mp4"
      type="video/mp4"
    />
  </video>

  <figcaption>A user adding a "mix test" action to the workflow</figcaption>
</figure>

Note that we also must set the `$MIX_ENV` environment variable to "test", since
the default value for the mix action is "dev".

## Adding a Format Action

The final action that we want to add to this workflow is to check our formatting
using [`mix format`][mix_format]. Since the project's dependencies are not
necessary for formatting code, we can run our format action from the root of the
workflow, rather than waiting for dependencies to be fetched.

<figure class="vid-figure">
  <video controls>
    <source
      src="https://jclem.nyc3.cdn.digitaloceanspaces.com/2019-01-26-github-actions-for-elixir/format.mp4"
      type="video/mp4"
    />
  </video>

  <figcaption>A user adding a concurrent "mix format --check-formatted" action to the workflow</figcaption>
</figure>

For the "args" property of this action, I've set "format --check-formatting".
The "--check-formatting" flag ensures that if the code isn't properly formatted,
the action will exit with a non-zero code and be considered to have failed.

Note that it's also possible to create an action that formats the code, verifies
that the formatted code has an equivalent AST, and then creates and pushes a
commit with the newly formatted code. I'll cover that in another post.

The workflow is now complete, so I commit the workflow file to my project's
repository.

## Viewing Action Runs

Now that we have a workflow file committed to the repository, it will run every
time a "push" event happens. If I open a pull request, for example, with
improper code formatting, we will have a helpful notification in the pull
request:

![A screenshot of GitHub Actions checks showing a failed “mix format” check](https://jclem.nyc3.cdn.digitaloceanspaces.com/2019-01-26-github-actions-for-elixir/checks.png)

If I click to view details of the failing format check, I'll be able to see the
output of my action:

```text
** (Mix) mix format failed due to --check-formatted.
The following files were not formatted:

  * /github/workspace/test/encrypted_field/encryption_test.exs

### FAILED Check Formatting 19:16:47Z (5.358s)
```

## More GitHub Actions & Elixir Ideas

There's a lot more that could be done with Elixir and Mix in GitHub Actions. I
have a personal project, for example, that uses a large Docker image with
PostgreSQL and Chromedriver for running tests (including integration tests with
[Hound][hound]) for a [Phoenix][phoenix] project.

I haven't had time to dig into these ideas yet, but I would love to see examples
of things like:

- Formatting code before a pull request is merged.
- Deploying an Elixir release from `master` when a pull request is merged.
- Creating [Nerves][nerves] releases, and making them available for download,
  when a pull request is merged (I'm actually not sure this is possible, yet,
  because of some runtime details).

If you come up with anything good, make sure and post it to
[ElixirStatus][elixir_status], and send it to me on [Twitter][twitter]!

[dialyzer]: http://erlang.org/doc/man/dialyzer.html
[ecto]: https://hex.pm/packages/ecto
[elixir_status]: https://elixirstatus.com
[encrypted_field]: https://hex.pm/packages/encrypted_field
[hound]: https://hex.pm/packages/hound
[mix_action]: https://github.com/jclem/actions/tree/master/mix
[mix_action_dockerfile]: https://github.com/jclem/actions/blob/master/mix/Dockerfile
[mix_format]: https://hexdocs.pm/mix/master/Mix.Tasks.Format.html
[nerves]: https://nerves-project.org/
[phoenix]: https://phoenixframework.org/
[twitter]: https://twitter.com/_clem
[workflow_file]: https://github.com/jclem/encrypted_field/blob/master/.github/main.workflow
