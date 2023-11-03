---
title: Observing Phoenix on Heroku
slug: 2019-08-21-observing-phoenix-on-heroku
published_at: 2019-08-21
published: true
summary: >-
  In this blog post, Jonathan Clem demonstrates how recent updates to the Heroku
  command line interface (CLI) enable the use of Erlang's Observer on Phoenix
  apps running on Heroku. He explains the necessary changes to a typical Phoenix
  deployment on Heroku and provides a step-by-step guide on how to connect
  directly to your dyno and open the Observer. This new functionality allows
  developers to monitor and troubleshoot their Phoenix applications more
  effectively.
---

In this post, I'm going to show you how recent updates to the Heroku command
line interface allows one to use Erlang's
[Observer](http://erlang.org/doc/apps/observer/observer_ug.html) on Phoenix apps
running on Heroku.

I wish I had time to write a lengthy blog post on using Observer, why it wasn't
possible on Heroku before, and why it is now, but I don't. Instead, I'm just
going to give a brief overview of what's now possible due to recent changes in
the CLI.

In order to connect directly to an Erlang node running your Phoenix application,
two things typically are needed. First, your node must be registered with the
[Erlang Port Mapper Daemon](http://erlang.org/doc/man/epmd.html) (or `epmd`)
that you connect to locally. Second, your node must be listening on a port that
you can connect to directly.

The Heroku command line interface has a command called `heroku ps:forward` that
forwards a local port to a port on your dyno. Until recently, though, this
command could _only forward a single port at a time_. What this meant was that
you couldn't simultaneously forward your local `epmd` client to `epmd` on the
dyno and also forward a local port to the port your node listens on for
distributed connections.

Recently, however, with the help of Herokai [Joe
Kutner](https://github.com/jkutner), I was able to land a [pull
request](https://github.com/heroku/heroku-ps-exec/pull/16) in the Heroku command
line interface that updates `heroku ps:forward` to accept a comma-separated list
of ports to forward.

In order for this to work, a few changes to a normal Phoenix deploy to Heroku
are necessary. You can see these changes in my
[jclem/phoenix-template](https://github.com/jclem/phoenix-template) repository,
or you can [generate your own
project](https://github.com/jclem/phoenix-template/generate) from the template
and deploy it directly to Heroku.

First, the application's Procfile needs to set a few options that allows us to
easily make remote connections:

```text
web: elixir --cookie $OTP_COOKIE --name server@127.0.0.1 --erl '-kernel inet_dist_listen_min 9000' --erl '-kernel inet_dist_listen_max 9000' -S mix phx.server
```

This is a lengthy Procfile command, so here's the command itself in a more
readable form:

```shell
$ elixir --cookie $OTP_COOKIE \
  --name server@127.0.0.1 \
  --erl '-kernel inet_dist_listen_min 9000' \
  --erl '-kernel inet_dist_listen_max 9000' \
  -S mix phx.server
```

This new Procfile sets the distributed Erlang cookie to the value of an
`OTP_COOKIE` environment variable. It also gives your node a predictable name to
connect to (`server@127.0.0.1`) and ensures that the node will listen for remote
connections on port `9000` (we set the port min and max to `9000`, so it will
only be that value).

Once your app is running, you can easily connect directly to your dyno. First,
in one shell, forward a couple of ports directly to your dyno:

```shell
$ heroku ps:forward 9001:4369,9000
```

This forwards your local port `9001` to the default port that `epmd` will be
listening on on the dyno, which is `4369`. Next, it forwards your local port
`9000` to port `9000` on the dyno, which is where the node is listening for
remote connections.

In another shell, you can now start a console, connect to the remote node, and
open observer (you'll need to get the value of the `OTP_COOKIE` secret on the
dyno and use it here):

```shell
$ env ERL_EPMD_PORT=9001 iex --cookie $OTP_COOKIE --name console@127.0.0.1
Erlang/OTP 22 [erts-10.4.2] [source] [64-bit] [smp:4:4] [ds:4:4:10] [async-threads:1] [hipe]

Interactive Elixir (1.9.1) - press Ctrl+C to exit (type h() ENTER for help)
iex(console@127.0.0.1)1> Node.connect(:"server@127.0.0.1")
true
iex(console@127.0.0.1)2> :observer.start
:ok
```

Once the Observer starts, you can use the "Nodes" dropdown menu to select your
remote node (`server@127.0.0.1`) and begin observing!
