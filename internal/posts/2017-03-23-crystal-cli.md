---
title: Building a Command-Line Application in Crystal
slug: 2017-03-23-building-a-command-line-application-with-crystal
published_at: 2017-03-23T00:00:00-04:00
published: true
summary: >-
  In this blog post, Jonathan Clem shares his experience building a command-line
  application in Crystal to help with filling in _.env_ files. He discusses the
  ease of parsing command-line options with Crystal's built-in option parser and
  compares JSON parsing in Crystal to other languages like Go. Clem also touches
  on error handling and how Elixir's approach has influenced his preferences.
  Overall, he enjoyed writing the application in Crystal and plans to continue
  using it for building command-line applications.
---

One of the things I do quite frequently as a developer is setting up a
development environment. I do lots of prototyping, so I'm always creating new
applications that have differing environment requirements. Typically, I use
[foreman][foreman] to run an application in an environment defined in a _.env_
file (e.g. `foreman run mix phoenix.server`), and I define an application's
environment needs in an [app.json][app_json] file. This works well, but filling
in a _.env_ file from the requirements outlined in an _app.json_ file is really
tedious.

In order to solve this, I decided to build a command-line application to help
with filling in _.env_ files. The application would need to:

1. Parse the ["env"][app_json_env] section in the _app.json_ file
1. Merge the parsed "env" with values from the
   ["environments"][app_json_environment] section, if needed
1. Merge any existing values from an existing _.env_ file
1. Prompt the user to optionally override any values
1. Write the values back to the _.env_ file

I wanted the application to be easy to install with as few requirements as
possible. For me, that narrowed the choices down to [Crystal][crystal_lang] and
[Go][go_lang]. I decided to go with Crystal, because although I appreciate that
it's very easy to cross-compile Go with no needed dependencies, I like Crystal's
[Ruby][ruby_lang]-like syntax and useful built-in [command-line option
parser][crystal_option_parser]. If you like, you can skip to the end result at
[jclem/bstrap][bstrap] (`brew tap jclem/bstrap && brew install bstrap` on a
Mac). In this post, I'm going to reflect on what I liked about building this
small application with Crystal and what was difficult.

## Parsing Command-Line Options

One of the first things that I found useful was, as I mentioned before,
Crystal's command-line option parser that comes as part of its standard library.
I wanted to have the commonly seen sort of command line options where a user can
pass a full option name or an alias. This was just as easy to do in Crystal as
it is in Ruby:

```crystal
require "option_parser"

class MyCLI
  def run
    path = "./default-path.txt"

    OptionParser.parse! do |parser|
      parser.banner = "Usage mycli [arguments]"

      parser.on("-p PATH", "--path PATH", "Path to a file") do |opt_path|
        path = opt_path
      end

      parser.on("-h", "--help", "Show this help") do
        puts parser
        exit 0
      end
    end

    puts "Your path is #{path}."
  end
end
```

With just a handful of lines and some other simple code to call this class, a
user can `mycli -p file.txt`, `mycli --path=file.txt`, and `mycli --help`. I was
really happy with how easy this was.

If you've ever used Ruby's [`OptionParser`][ruby_option_parser] class before,
you'll notice that the Crystal equivalent is almost identical. A more complete
example of option parsing in Crystal is [in the bstrap
repo][bstrap_option_parsing], or you can try out similar code in a [Crystal
playground][play_option_parsing].

## Parsing JSON

The next hurdle for me, parsing the JSON contents of an _app.json_ file, was the
one I knew I'd have the most trouble with. Crystal is a [statically
type-checked][static_type_checking] language, so I knew that there might be a
good deal of boilerplate involved in parsing an _app.json_ file and ensuring
that I'm working with the types I expect to be working with. Further
complicating things is the fact that the _app.json_ specification allows
multiple different types for many values. For example, an entry in "env" can be
either a string representing the default value of that environment variable or
an object describing the environment variable, e.g.:

```json
{
  "env": {
    "NODE_ENV": "production",
    "DATABASE_URL": {
      "description": "A URL pointing to a PostgreSQL database"
    }
  }
}
```

The first step in parsing JSON was to write a simple function to read the
_app.json_ file, parse it, and return a hash or raise if the root of the JSON
document is not an object. This was relatively straightforward—I'll define a
function called `parse_app_json_env` (we'll add the "env" parsing to it soon):

```crystal
class Bstrap::AppJSON
  class InvalidAppJSON < Exception
  end

  def parse_app_json_env(path : String)
    raw_json = File.read(path)

    if app_json = JSON.parse(raw_json).as_h?
      app_json
    else
      raise InvalidAppJSON.new("app.json was file not an object")
    end
  rescue JSON::ParseException
    raise InvalidAppJSON.new("app.json was not valid JSON")
  end
end
```

The basic form of this function was relatively straightforward. First, we read
the file at the given path and parse it as JSON (notice that we `rescue` invalid
JSON and return our custom exception). Then, we check whether the parsed JSON is
an object. We do this because in JSON, a document may be an object, an array, or
a scalar value. Obviously, we want to ensure that the contents of our _app.json_
aren't, for example, an array or simply an integer.

It took me a little bit of getting used to, but Crystal actually makes this
checking pretty easy. The [`JSON.parse`][json_parse] class method returns a type
called [`JSON::Any`][json_any]. This type is simply a wrapper around all
possible JSON types, and provides some useful methods to ensure we're wrapping
the type we want. In the above example, you'll see [`#as_h?`][json_any_as_h]
called. This method returns the type `Hash(String, JSON::Type)?` meaning either
`nil` or a hash with string keys and JSON type values. Putting things together,
we can check `#as_h?` and either return that hash or raise an error because the
contents of our _app.json_ file was something other than an object.

This was relatively straightforward, but remember that I want this method to
return the parsed "env" object, not just the raw parsed _app.json_ file. I
updated my `parse_app_json_env` to call a new method called `parse_env` that
would take care of this:

```crystal
def parse_app_json_env(path : String)
  raw_json = File.read(path)

  if app_json = JSON.parse(raw_json).as_h?
    parse_env(app_json)
  else
    raise InvalidAppJSON.new("app.json was file not an object")
  end
rescue JSON::ParseException
  raise InvalidAppJSON.new("app.json was not valid JSON")
end
```

The `parse_env` method had a tricky job, because as I said before, the
_app.json_ schema allows values in "env" to be either strings or objects. For
the sake of programming ease, I wanted to ensure that this method was always
returning a hash whose values were other hashes, regardless of what was parsed.
To express this, I defined a couple of new type aliases:

```crystal
type JSONObject = Hash(String, JSON::Type)
type ParsedEnv = Hash(String, JSONObject)
```

I first defined `JSONObject` simply to refer to a hash whose keys are strings
and whose values are JSON types. I could have been more specific to the "env"
format and created a type whose keys are strings and whose values are either
strings or booleans (for the "required" key from the _app.json_ schema), but
this didn't seem necessary.

The `ParsedEnv` type refers to a hash whose keys are strings and whose values
are `JSONObject`s.

With these new types in hand, I could create the `parse_env` function that would
read the "env" from an _app.json_ hash and return a `ParsedEnv`:

```crystal
private def parse_env(app_json : JSONHash) : ParsedEnv
  parsed = ParsedEnv.new

  case env = app_json.fetch("env", nil) # Ensure we have an "env"
  when Hash
    env.reduce(parsed) do |parsed, (key, value)|
      case value
      when String
        parsed[key] = {"value" => value.as(JSON::Type)}
      when Hash
        parsed[key] = value
      else
        raise InvalidAppJSON.new(%(app.json "env" value was not a string or an object))
      end
    end
  when nil
    parsed
  else
    raise InvalidAppJSON.new(%(app.json "env" was not an object))
  end
end
```

I think that the above isn't particularly pretty, but it does the job. I fetch
the "env" value and assert that it's an object (we just return an empty
`ParsedEnv` if it's not present, which is acceptable), and then iterate over its
key-value pairs. For each pair, we then have to check the value and ensure that
it's a string or a hash, and raise otherwise.

The above example also introduces some of the things that are still mysteries to
me about the Crystal type system: `ParsedEnv` is an alias for `Hash(String,
JSONObject)`, and `JSONObject` is an alias for `Hash(String, JSON::Type)`.
`JSON::Type`, in turn, is an alias for a number of other types, including
`String`. Why, then is it necessary for me to restrict the type of `value` to
`JSON::Type` when the compiler already knows that it is a `String`?

Another thing that wasn't apparent to me in the example above at first is that I
could call `ParsedEnv.new`. If I were to declare `parsed = {}`, the compiler
would complain that I should declare an empty hash in a way that includes the
expected key-value types, e.g. `parsed = {} of String => JSONObject`. I have a
lot of these in bstrap, still, and didn't realize until someone told me that I
could call `.new` on a type alias, instead, and get the same result.

Overall, I found JSON parsing a little bit easier than I've found it with other
type-checked languages such as Go. The weirdness of type restrictions and the
tedium of checking everything as early as possible is a little tiresome, but
helps prevent bugs.

## Exceptions

Elixir has spoiled me. This command-line application has file reading, JSON
parsing, and file writing, so there is plenty of opportunity for exceptions to
be thrown. Given an imaginary program that reads a file, parses its JSON, and
then writes "ok" to the file, an error-handled Elixir program might look like
this:

```elixir
with {:ok, raw_file} <- File.read(path),
     {:ok, map}      <- Poison.parse(raw_file),
     :ok             <- File.write(path, "ok") do
  :ok
else
  {:error, :enoent}  -> {:error, "Could not read file"}
  {:error, :invalid} -> {:error, "File contained invalid JSON"}
  {:error, _}        -> {:error, "Other error"}
end
```

In Crystal, the same error handling might look like this:

```crystal
begin
  raw_file = File.read(path)
  map = Poison.parse(raw_file)
  File.write(path, "ok")
  :ok
rescue Enoent
  raise "Could not read file"
rescue JSON::ParseException
  raise "Could not parse file"
rescue ex
  raise "Other error"
end
```

I greatly prefer the Elixir way, not only because I think that it reads better,
but also because in Crystal, I'm frequently having to look up and see what
possible exceptions a particular method might raise (or whether it might raise
any at all). Elixir indicates this clearly by either suffixing a function name
with a bang, e.g. `Poison.parse!` or by returning a tagged tuple, where `{:ok,
value}` means success, and `{:error, error}` indicates the error that occurred.
For some reason, this makes me feel much more assured that I am properly
handling errors, rather than ending every method with a `rescue` clause.

There is a similar pattern available in the [Bluebird][bluebird] Promises
library for JavaScript. Bluebird allows me to chain a set of promises, and then
pattern match my error handling based on a predicate (which may be a function or
an error constructor):

```javascript
somePromise()
  .then(anotherPromise)
  .then(aThirdPromise)
  .then(aFourPromise)
  .catch(ReadError, handleReadError)
  .catch(ParseError, handleParseError)
  .catch(WriteError, handleWriteError)
  .catch(handleUnexpectedError);
```

I really like this pattern and was happy when the `with` keyword made its way
into Elixir, which feels very similar. I wish Crystal/Ruby had something like
this.

## Wrap-up

Overall, I really enjoyed writing bstrap in Crystal, and I think I'll continue
to use it for building command-line applications. It can't do fancy things like
statically link libraries like Go can so that I can just send someone a binary
(I have to install some libs with Homebrew, instead), but the ease of
programming in an extremely fast Ruby-like language with static type checking
definitely makes up for that.

[app_json]: https://devcenter.heroku.com/articles/app-json-schema
[app_json_env]: https://devcenter.heroku.com/articles/app-json-schema#env
[app_json_environment]: https://devcenter.heroku.com/articles/app-json-schema#environments
[bluebird]: http://bluebirdjs.com/docs/api/catch.html#filtered-catch
[bstrap]: https://github.com/jclem/bstrap
[bstrap_option_parsing]: https://github.com/jclem/bstrap/blob/8ba7139d5c6f39487edaa755b4b14142a885885f/src/bstrap/cli.cr#L29-L56
[crystal_lang]: https://crystal-lang.org
[crystal_option_parser]: https://crystal-lang.org/api/0.21.1/OptionParser.html
[foreman]: https://github.com/ddollar/foreman
[go_lang]: https://golang.org
[json_any]: https://crystal-lang.org/api/0.21.1/JSON/Any.html
[json_any_as_h]: https://crystal-lang.org/api/0.21.1/JSON/Any.html#as_h%3F%3AHash%28String%2CType%29%3F-instance-method
[json_parse]: https://crystal-lang.org/api/0.21.1/JSON.html#parse%28input%3AString%7CIO%29%3AAny-class-method
[play_option_parsing]: https://play.crystal-lang.org/#/r/1qru
[ruby_lang]: https://www.ruby-lang.org
[ruby_option_parser]: https://docs.ruby-lang.org/en/2.4.0/OptionParser.html
[static_type_checking]: https://en.wikipedia.org/wiki/Type_system#Static_type_checking
