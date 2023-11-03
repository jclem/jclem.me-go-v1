---
title: How I Take Notes
slug: how-i-take-notes
published_at: 2020-07-01
published: true
summary: >-
  In this blog post, Jonathan Clem shares his personal note-taking journey and
  how he has settled on using the Bear app for the foreseeable future. He
  explains his daily log system, which involves creating a new note for each day
  and appending tasks, thoughts, and events. Clem also discusses his use of
  "contexts" and "pages" to organize information related to projects and people.
  He appreciates Bear's hashtag feature, which allows for easy organization and
  referencing of topics. Although he is satisfied with Bear, Clem suggests some
  improvements he would make if he were to create his own note-taking app again,
  such as automation and the ability to create tag synonyms.
---

Like many people, I have for years switched back and forth between numerous
note-taking applications. The services and applications I've used over time
include, but are not limited to:

- Plain text
- Apple Notes
- [Canvas][canvas] (my own startup, now shut down)
- [Dropbox Paper][paper] (whose infrastructure I worked on for a year)
- [Bear][bear]
- [Notion][notion]
- [Notational Velocity][nvelocity]

...and many more.

For the last couple of years, I have switched back and forth heavily between
Notion and Bear. I appreciate how Notion looks on the desktop and am interested
in its powerful database features, but prefer the way that Bear lets me organize
and reference topics. For the past few months, it's become clear to me that the
winner (for me) is Bear for the forseeable future. In this post, I'm going to
talk about why I enjoy taking personal notes in Bear, and what I would change if
I were writing my own app, again.

## A Daily Log

The root of my note taking is a simple, mostly linear daily log. For each day, I
create a new note and begin appending tasks, relevant thoughts, and relevant
events to the log. Generally if I have a meeting, I won't create a new note for
it, but will add notes in a nested list:

![A screenshot of a daily note, with meeting notes in a nested list](https://jclem.nyc3.cdn.digitaloceanspaces.com/how-i-take-notes/log.png){: .figure}

There's nothing particularly novel here—I have a log per day and keep a list of
notes for that day.

In order to make these daily logs easy to peruse, however, I make use of one of
Bear's most powerful features, its hashtags (and particularly their ability to
be nested). For each week (beginning on Monday), I use a hashtag such as
`#log/2020/06/29`, where the final day segment is the start of the week for that
log's day.

![A screenshot of a daily note with the tag “#log/2020/06/29”](https://jclem.nyc3.cdn.digitaloceanspaces.com/how-i-take-notes/log-tag.png){: .figure}

Although Bear can sort tags by date created or modified, I find it helpful to be
able to also scope them down to a span of time that I care about.

<figure class="vid-figure">
  <video controls width="1286" height="1070">
    <source
      src="https://jclem.nyc3.cdn.digitaloceanspaces.com/how-i-take-notes/log-tags.mp4"
      type="video/mp4"
    />
  </video>

  <figcaption>A video showing a user filtering Bear notes by selecting various spans of time: Year, month, and week.</figcaption>
</figure>

Since I organize my daily logs using nested tags, I can peruse them by each
subsection of that tag—all logs, all logs for the given year, all logs for the
given year/month, or all logs for the given year/month/week-start. I find this
really helpful for focusing on specific timespans when doing things like writing
my self-reviews at GitHub.

## Contexts and Pages

The next main category of organization that I employ in Bear is what I call
"contexts". Contexts allow me to reference a thing of a specific type, and to
easily browse all notes that reference that thing. In addition, pages give me
the ability to keep a "home" page for every thing that needs it in Bear.

### Projects

The context that I use the most is the "projects" context. In the screenshots of
the daily log near the start of this article, you'll see references to "X
Project" and "Y Project". The first time I reference any of these projects, I do
a couple of things:

1. I begin referencing them by a tag, such as `#projects/GitHub/X Project#`.

   ![A screenshot showing project reference tags such as “#projects/GitHub/X Project”](https://jclem.nyc3.cdn.digitaloceanspaces.com/how-i-take-notes/project-tags.png){:.figure}

1. I create a new home page for that project with basic information.

   ![A screenshot showing a home page for the “#projects/GitHub/X Project” tag](https://jclem.nyc3.cdn.digitaloceanspaces.com/how-i-take-notes/project-page.png){:.figure}

Now, I have a tag which lets me reference that project in any note (and gives me
the ability to find any note where I referenced that project). Additionally, it
gives me a home page for that project where I can keep general information about
it. Frequently, these pages are where I keep lists like who else is working on
the project, what all of the relevant issues, pull requests, pieces of
documentation are, and sometimes links to Slack conversations.

### People

The second context I use heavily is the "people" context. Whenever I mention
someone in my daily log—for example, because I had a meeting with them or spoke
to them on Slack about something—I do the same thing that I do for the projects
context: I create a tag for that person that I then use to refer to them, and
also create a home page with basic information (generally this is only their
GitHub username).

I should also note that for a long time, I was uncomfortable with this. It felt
like I was keeping a creepy collection of people in my personal notes. Over
time, though, I realized what a huge help it is when I have to write reviews for
others. It really improves the quality of those reviews when I can easily refer
back to conversations we had and projects we worked on together without having
to rely entirely on memory.

![A screenshot showing use of a person tag: “#people/GitHub/Ada Coleman”](https://jclem.nyc3.cdn.digitaloceanspaces.com/how-i-take-notes/person-tags.png){:.figure}

![A screenshot showing a tag home page: “#people/GitHub/Ada Coleman”](https://jclem.nyc3.cdn.digitaloceanspaces.com/how-i-take-notes/person-page.png){:.figure}

Bear has excellent tag autocompletion, so to refer to this person in my notes,
just typing `#Ad[TAB]` is sufficient.

## Building a Habit

So far, I've found this system to be incredibly helpful. It's possible to create
something similar in Notion, but it requires joining databases and a huge amount
of manual labor that I couldn't keep up with. Except for some minor friction,
I've found it easiest to keep a daily logging and note-taking habit with Bear.

You may notice that I haven't used Bear's ability to link directly to another
page in this. That feature is something that I rarely use, because contexts
essentially do the same thing—it'll be very easy to quickly find the "home page"
for a given project amongst its tags, or I just look in the sidebar for all of
my project pages. Likewise, linking directly to another page doesn't allow me to
easily build a network of cross-linked topics (this is something Roam does, but
I didn't enjoy the experience of taking notes in Roam).

## Improvements

If I were building a personal note-taking app again, I think that I'd employ a
very similar model to Bear, but I have a few specific improvements I would make
in mind:

The most important change I might make would be some automation—I create a home
page for the majority of tags that I create with my context system, and so
having that done automatically would be really helpful.

I would also love to be able to create tag synonyms. At GitHub, some people are
referred to by their "real" name, and some like to be called by their GitHub
handle, and some go back and forth. Ideally, I would be able to keep both as
viable hashtags for that person, so if I were mentioning myself, I could either
use the tag `#people/GitHub/Jonathan Clem#` or something short like my actual
GitHub handle, `@jclem`.

That's just for organization, though. There are a _huge_ amount of changes to
the editing experience I would make, but I'll save those for another time!

[bear]: https://bear.app
[canvas]: https://github.com/usecanvas
[notion]: https://notion.so
[nvelocity]: http://notational.net
[paper]: https://www.dropbox.com/paper
