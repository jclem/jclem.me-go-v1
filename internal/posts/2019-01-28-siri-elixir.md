---
title: Hey Siri, Deploy My Elixir App
slug: 2019-01-28-hey-siri-deploy-my-elixir-app
published_at: 2019-01-28T00:00:00-04:00
published: true
summary: >-
  In this blog post, Jonathan Clem demonstrates how to deploy an Elixir app
  using Siri on an iPhone. He walks through the process of building a GitHub
  Actions workflow that deploys the app to the Heroku Container Runtime using
  Actions, and then shows how to trigger the deployment using Siri. While this
  method may not be recommended for real-world production apps, it serves as a
  fun example of what can be done with GitHub Actions.
---

<figure class="vid-figure">
  <video controls>
    <source
      src="https://jclem.nyc3.cdn.digitaloceanspaces.com/2019-01-28-hey-siri-deploy-my-elixir-app/deploy-ping.mp4"
      type="video/mp4"
    />
  </video>

  <figcaption>
  A video showing a user saying "Hey Siri, deploy Ping", and then watching
  GitHub Actions deploy the app. Apologies for my horrendous editing and lack of
  stabilization.
  </figcaption>
</figure>

Just for the heck of it, I decided to attempt to build a GitHub Actions workflow
that would allow me to deploy an Elixir app using Siri on my iPhone. In this
post, I'm going to demonstrate how I deploy the app to the [Heroku Container
Runtime][container_registry] using Actions, as well as how I can trigger
deploying the app using Siri.

I'm not sure I recommend what I'm doing in this post for a real-world production
app, but it's a fun example of what can be done with GitHub Actions.

## Building the Elixir App

The app that we're going to be deploying is a simple server built with
[Plug][plug] that sends "pong \$app_version" in response to a GET request to the
"/ping" route. You can view the source code for this app before I added any of
the workflow- or container-related code [here][a0d0e9] (the code that returns
the app version was added later, but it's not particularly interesting).

After I built the basic app, I used [Distillery][distillery] to generate a
[release configuration file][rel]. It's pretty straightforward—the only change
from the default generated release configuration is that it grabs the Erlang
cookie from an environment variable.

I'm not going to get into much more detail on the app itself, but feel free to
poke around in the code, if you're interested.

## A Dockerfile for the App

In order to run the app on the Heroku Container Runtime, the app needs a
Dockerfile that copies the built release, and then runs it in the foreground.
You can see the app's Dockerfile [here][app_dockerfile].

The Dockerfile is based on Alpine Linux. It copies the built release file from
the "\_build" directory, and then calls `./bin/ping foreground`. Running the app
in the foreground is necessary to get log output from the `heroku logs -t`
command.

## Building the Workflow

The [workflow file][workflow_file] is a little more complicated. We need to
ensure that the environment in which the release is compiled is identical to the
environment of the app's Dockerfile. Since the Dockerfile is based on Alpine
Linux, the command that compiles the release as a part of our Actions workflow
also needs to be Alpine Linux.

I'm going to step through the actions one-by-one that the workflow uses.

### The Workflow

In order to use Siri to deploy this app, we're going to tell the workflow to
trigger on the ["repository_dispatch"][repository_dispatch] event.

```hcl
workflow "Build & Release" {
  on = "repository_dispatch"
  resolves = "Container Release"
}
```

Triggering the workflow on this event will allow us to use Siri shortcuts to
make a POST request to the GitHub API in order to deploy our app. I'll go into
more detail on that later.

### Create Release

The first thing that the workflow does is creating the release. This is the
self-contained archive with everything necessary in order to run the app. If
you're unfamiliar with OTP releases, you can [read more about them (and
Distillery) on ElixirSchool][elixir_school_releases].

```hcl
action "Create Release" {
  uses = "./.github/mix"
  args = "do deps.get, compile, release"
  secrets = ["COOKIE"]
}
```

You'll notice that this action uses a repo-relative action at "./.github/mix".
The reason for this is what I stated earlier—I need to ensure that the action I
use to run my Mix commands is based on Alpine Linux. I do have a [Mix
action][mix_action] that I use in some other workflows, but it's not based on
Alpine Linux.

The [Dockerfile][mix_dockerfile] for this action installs rebar and hex, and
then sets `mix` as the entrypoint for the container by using an entrypoint shell
script.

### Registry Login

After we've created the release, we need to log in to the Heroku container
registry.

```hcl
action "Registry Login" {
  uses = "./.github/heroku"
  needs = "Create Release"
  args = "container:login"
  secrets = ["HEROKU_API_KEY"]
}
```

This action also uses a custom repository-relative action. For the Heroku
container registry commands to work, we need both Docker and the Heroku command
line interface tool to be available in our container. This action is built based
on the "docker" image, and then installs Node and the Heroku CLI with NPM. It
sets `heroku` as the container entrypoint.

Notice that we are also providing a "HEROKU_API_KEY" secret that I've configured
on the repository. The Heroku CLI tool picks up this environment variable and
uses it for authentication.

### Container Push

Now that we're logged into the Heroku container registry, we need to push our
app to the registry.

```hcl
action "Container Push" {
  uses = "./.github/heroku"
  needs = "Registry Login"
  args = "container:push web --app ping-ex"
  secrets = ["HEROKU_API_KEY"]
}
```

The "args" value here calls `heroku container:push web --app ping-ex`, which
tells the Heroku CLI to build the app using the Dockerfile in the project, and
then push it to the container registry for the "ping-ex" app's "web" dyno.

### Container Release

Once the container is pushed, we want to actually release it so that it becomes
the "active" container running for our app.

```hcl
action "Container Release" {
  uses = "./.github/heroku"
  needs = "Container Push"
  args = "container:release web --app ping-ex"
  secrets = ["HEROKU_API_KEY"]
}
```

This action calls `heroku container:release web --app ping-ex`, which takes the
most recently-pushed image and releases it to the "web" dyno. Heroku will start
routing requests to this dyno once the container starts.

## Building the Siri Shortcut

Now that we've built the workflow, we need to build a Siri Shortcut to deploy
the app. You can get the shortcut that I built for this [here][shortcut].

The shortcut makes a POST request to the repository dispatch endpoint that looks
like this:

```curl
curl -X POST https://api.github.com/repos/$username/$repo/dispatches \
  -H "Accept: application/vnd.github.everest-preview+json" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $personal_access_token" \
  -d '{ "event_type": "deploy" }'
```

For this to work, you need a personal access token with "repo" scope in order to
make a request to the repository dispatches endpoint.

Once the shortcut is added to your Shortcuts app, you can add it to Siri. Apple
has a [helpful article][siri_article] explaining how to do this.

Now, you can simply say "Hey Siri, deploy my app" in order to push the latest
code on GitHub to Heroku!

[a0d0e9]: https://github.com/jclem/ping_ex/tree/a0d0e9ea45725e95faa297a53ff7bd594aa58c1e
[app_dockerfile]: https://github.com/jclem/ping_ex/blob/master/Dockerfile
[container_registry]: https://devcenter.heroku.com/articles/container-registry-and-runtime
[distillery]: https://hexdocs.pm/distillery
[elixir_school_releases]: https://elixirschool.com/en/lessons/libraries/distillery/
[mix_action]: https://github.com/jclem/actions/tree/master/mix
[mix_dockerfile]: https://github.com/jclem/ping_ex/blob/master/.github/mix/Dockerfile
[plug]: https://hexdocs.pm/plug/readme.html
[rel]: https://github.com/jclem/ping_ex/blob/master/rel/config.exs
[repository_dispatch]: https://developer.github.com/actions/creating-workflows/triggering-a-repositorydispatch-webhook
[shortcut]: https://www.icloud.com/shortcuts/c4f25126496e4203aed7617d82288098
[siri_article]: https://support.apple.com/en-us/HT209055
[workflow_file]: https://github.com/jclem/ping_ex/blob/master/.github/main.workflow
