---
title: On the Utility of Phoenix LiveView
slug: on-the-utility-of-phoenix-live-view
published_at: 2019-09-19T00:00:00-04:00
published: true
summary: >-
  In this blog post, Jonathan Clem explores the benefits of using Phoenix
  LiveView, a package that allows for dynamic page updates without the need for
  a reload. He shares his experience of incorporating LiveView into his personal
  website, enabling him to create and edit blog posts within the site itself.
  Clem highlights the ease of implementing live side-by-side rendering of post
  content while writing in Markdown, without the need for JavaScript or
  WebSockets. Ultimately, he appreciates the freedom Phoenix LiveView provides
  in quickly implementing features that would otherwise be too time-consuming or
  complex to bother with.
---

Lately, I've been playing around a lot with [Phoenix
LiveView](https://github.com/phoenixframework/phoenix_live_view). If you're not
already familiar, it's a package that allows page contents to be dynamically
updated without a reload. For example, if you want to display how many times a
user on a page has clicked a given button, Phoenix LiveView would allow you to
do that without writing any JavaScript.

What I've found as I have been using it more and more is that it's giving me the
freedom to try things out that I otherwise wouldn't think about doing. I write a
lot of JavaScript, particularly React and, in the past, Ember. However, the
thought of pulling in one of these dependencies in in order to implement a
simple feature seems like overkill. Likewise, so does the thought of writing
vanilla JavaScript and having to think about how to organize it.

In order to encourage myself to write a bit more, I've rewritten my personal
website as a Phoenix-backed blog engine. I love GitHub Pages for a lot of use
cases, but I have future plans for this site that will require an API.
Additionally, I like to be able to make instant changes without having to write
a Git commit message, do a push, and wait for a build pipeline to kick off. As a
part of this rewrite, I decided that I wanted the ability to create and edit
blog posts _within_ the site. I generally also enjoy writing in Notion and other
Markdown editors, but for blogging, I prefer to be able to just write and
publish to the same place (all of this may change if Notion ever allows me to
have a custom domain for a workspace).

The posts for this blog are written and stored in a database in Markdown. I have
an administrator interface that includes a simple plain text editor, and as I
was building it, I realized that it would be nice to have a live side-by-side
rendering of my post content as I was writing Markdown. Typically, in order to
do this, I would have to do one of a couple of different things:

1. Use JavaScript (or, more likely, a JavaScript framework) to render my post as
   Markdown, ensuring that it had the same output as the server-side Markdown
   rendering package that I use.
2. Wire up my own implementation of a live preview using WebSockets, where I
   observe change events in the post form, send post content, over a socket,
   render the Markdown and send HTML _back_ over the socket, then display that
   rendered Markdown alongside the editor view.

Neither of these sound particularly good to me—after all, running a blog isn't
my day job.

Phoenix LiveView, on the other hand, presents another option. With it, I can
move my editing template from a normal Phoenix `*.html.eex` template into a
LiveView module. The template contents remain basically the same, other than
that I now also render a preview of my post contents alongside my editing form,
using the same Markdown rendering function used in the view that publicly
renders posts.

In order to ensure that live updating happens, I simply add a
`phx-change="updated"` attribute to my HTML `form` tag. Now, whenever the
contents of my form change (i.e. as I type into the text area), the contents of
the form are sent over a WebSocket connection to the server. There, the view is
re-rendered using the current form contents, updated content is then sent back
over the socket, and the preview is automatically updated in the DOM for me. In
order to do all of this, the only JavaScript that I had to write was _two_ lines
to set up the initial LiveView connection!

What I'm most excited for with Phoenix LiveView isn't necessarily the cool
technology that it really is, but more the freedom that it gives me to quickly
implement nice-to-have features that would otherwise be too much work to bother
with.

<figure class="vid-figure">
  <video controls>
    <source
      src="https://jclem.nyc3.cdn.digitaloceanspaces.com/on-the-utility-of-phoenix-liveview/live-view.mp4"
      type="video/mp4"
    />
  </video>

  <figcaption>A video showing Phoenix LiveView updating a post preview while a user edits the post contents</figcaption>
</figure>
