---
title: Building a Router in React
slug: building-a-router-in-react
published_at: 2019-12-19T00:00:00-04:00
published: true
summary: >-
  In this blog post, Jonathan Clem shares his experience building a simple
  router in React, inspired by a tweet from Joel Califa. The goal was to create
  a way to render different "page" components depending on the location's path,
  link between those pages without passing state around, and use no dependencies
  other than React. Clem walks readers through the process of setting up the
  basic application skeleton, creating a RouterProvider component using React
  Context, wiring it up with the application component, and handling user
  navigation. While the experiment was successful, Clem acknowledges that there
  are more complex use cases and features that real routing libraries handle.
---

Recently, I was inspired by a
[tweet](https://twitter.com/notdetails/status/1204638051009519617) by Joel
Califa to attempt to build a simple router in React. The goal was to end up with
three things:

1. A way to render different "page" components depending on the location's path
1. A way to link between those pages without having to pass state around
1. Use no dependencies (other than React, of course)

My first instinct was that I could just use existing browser history event APIs.
As long as I have a `PageLink` component that calls `history.pushState(path)`
when clicked, there must be some event that a router context can listen in on
when that happens. Unfortunately, this isn't the case! The browser provides a
`popstate` event, but that's only called as a result of user action (such as
clicking the back button) or some other history APIs. We will end up using
`popstate`, eventually, though, so stay tuned, `popstate` fans!

## The Setup

First, let's set up the basic application skeleton. We'll have an `App`
component that renders a different page, depending on the current route.
Sketched out roughly, it looks something like this:

```jsx
export default function App() {
  // Hand-wavy "get the current route state"
  const route = getRoute();

  switch (route) {
    case "/":
      return <Home />;
    case "/about":
      return <About />;
    default:
      return <NotFound />;
  }
}
```

Pretty simple! We get the route (somehow) and render the appropriate page based
on that route.

We also need some way of linking between these pages. For that, we'll create a
`PageLink` component:

```jsx
export default function PageLink(props) {
  return (
    <a
      href={props.path}
      onClick={(evt) => {
        evt.preventDefault();
        setRoute(props.path); // Hand-wavy
      }}
    >
      {props.children}
    </a>
  );
}
```

This component just renders its children inside of a link. When clicked, we'll
have to come up with some way for it to set the route to the `path` property.
Note that we want to avoid passing state around, so we're going to rule out
anything like `props.setRoute`. In addition, we want to be able to put these
page components in separate files, so relying on `route` and `setRoute` being in
scope and using closures is out of the picture, as well.

## Context

Thankfully, React has a very useful tool for when we want to pass data through
the component tree without having to pass props down manually at every level.
It's called [Context](https://reactjs.org/docs/context.html), and its
documentation states that it "provides a way to pass data through the component
tree without having to pass props down manually at every level"!

In order to make use of React Context, we're going to build a component called a
[Provider](https://reactjs.org/docs/context.html#contextprovider). Essentially,
the Provider component is what will allow
[Consumer](https://reactjs.org/docs/context.html#contextconsumer) components,
which we'll build later, to consume (and update) the routing state! First we'll
create our Context object and our Provider component:

```jsx
// Just create an empty context, we'll give it data later.
export const RouterContext = React.createContext();

/* Technically, we could just put this in our `App` component,
   but I like doing it this way, where only this simple wrapper
   component has the raw state value and its setter in it. */
export function RouterProvider(props) {
  // The initial state for the Provider will always be the current location.
  const [route, setRoute] = useState(location.pathname);

  return (
    <RouterContext.Provider
      // Set the initial state: A `route` value, and a `setRoute` function
      value={{
        route: route,
        setRoute: (path) => {
          history.pushState(null, "", path);
          setRoute(path);
        },
      }}
    >
      {props.children}
    </RouterContext.Provider>
  );
}
```

This `RouterProvider` component uses `RouterContext.Provider` component in the
context object we created. It encapsulates one piece of state: the `route`
itself, which we'll set up soon. In addition to the `route` property, the
component also provides a `setRoute` function. Note, however, that we're not
just blindly passing the `setRoute` function created by
`useState(location.pathname)`. This _would_ work for state-management purposes,
but we also want to ensure that setting the route also updates the current URL!
In order to do that, the `setRoute` function in the provider calls the browser's
history API and "pushes" the new route into the history stack via
`history.pushState(null, '', path)`. After that, _then_ it updates its route
state by calling the original `setRoute` function provided by `useState`.

## Wiring it Up

Now that we have the provider set up, we'll use it (and the consumer) in our
application component to get the current route for rendering the proper page
component:

```jsx
import { RouterProvider, RouterContext } from "./router";
import Home from "./pages/Home";
import About from "./pages/About";
import NotFound from "./pages/NotFound";

export default function App() {
  /* We want to render the provider once at the highest
     necessary level for consuming it */
  return (
    <RouterProvider>
      {/* We can use the consumer now that the provider is setup */}
      <RouterContext.Consumer>
        {({ route }) => {
          switch (route) {
            case "/":
              return <Home />;
            case "/about":
              return <About />;
            default:
              return <NotFound />;
          }
        }}
      </RouterContext.Consumer>
    </RouterProvider>
  );
}
```

Now, anything inside of the component tree from that point down can use
`RouterContext.Consumer` to consume the `route` and (if necessary) `setRoute`
values.

Now, our `PageLink` component can also make use of the consumer in order to read
and set the current route state! Let's also say, maybe, that we want `PageLink`
to only wrap its children in an anchor tag if it's not already the active route.

```jsx
import { RouterContext } from "./router";

export default function PageLink(props) {
  const { route, setRoute } = React.useContext(RouterContext);

  if (route === props.path) return props.children;

  return (
    <a
      href={props.path}
      onClick={(evt) => {
        evt.preventDefault();
        setRoute(props.path);
      }}
    >
      {props.children}
    </a>
  );
}
```

Now, the `PageLink` component just renders its children with no anchor if the
current route state matches the `path` property. If it doesn't, then it renders
an anchor tag. When that tag is clicked, instead of navigating to the URL in the
`href` attribute of the tag, the component instead calls `setRoute` from our
router context.

## Handling User Navigation

Our router works pretty well now! Users can click around and navigate the app,
and the URL changes along with the page contents, all without any page reloads.
In testing this out though, I noticed there's at _least_ one major caveat. When
the user presses the back/forward buttons in the URL, the browser location
changes, but the app doesn't render anything different to reflect that change.

The reason this is happening is that although we're tracking history state
changes in this document when they're initiated by our code calling `setRoute`,
we're not updating state when the _user_ does something to navigate back and
forth through that history stack. Thankfully, there's an API for doing that, and
this is where `popstate` comes back in. When the user presses the back/forward
buttons in their browser, the window object receives a `popstate` event
notifying us that the page's path has changed.

We'll update our `RouterProvider` component to make sure we're reacting to those
events:

```jsx
export const RouterContext = React.createContext();

export function RouterProvider(props) {
  const [route, setRoute] = React.useState(location.pathname);

  // This is the new stuff!
  React.useEffect(
    () => {
      const setRouteToPathname = () => setRoute(location.pathname);
      window.addEventListener("popstate", setRouteToPathname);
      return () => window.removeEventListener("popstate", setRouteToPathname);
    },
    [] /* Only want to call this once! */
  );

  return (
    <RouterContext.Provider
      value={{
        route: route,
        setRoute: (path) => {
          history.pushState(null, "", path);
          setRoute(path);
        },
      }}
    >
      {props.children}
    </RouterContext.Provider>
  );
}
```

Now, `RouterProvider` uses `React.useEffect` to set up an event listener on the
window object. When the window receives a `popstate` event, we'll call
`setRoute` and pass it the new location that resulted from the navigation event.
Likewise, when the `RouterProvider` is torn down, we'll remove that listener so
that we don't accidentally try to set state in an unmounted component.

## Conclusion

All in all, I think that this was a successful and pretty fun experiment. I'm
sure there are some edge cases that I haven't covered, but I'm certainly
impressed by how far one can get just by using the fundamental tools provided by
React itself.

Of course, there's a _lot_ more that a router sometimes needs to do. For
example, this completely ignores the common use case of using path parameters
that would likely get passed to our page components. So I don't mean to
trivialize any real routing libraries, by any means!

If you want to play around with the code, fork
[jclem/react-simple-router](https://github.com/jclem/react-simple-router). You
can even deploy it to Zeit with `yarn deploy`.
