---
title: Building a Pannable, Zoomable Canvas in React
slug: pan-zoom-canvas-react
published_at: 2020-10-16
published: true
has_math: true
summary: >-
  In this blog post, Jonathan Clem shares his experience building a pannable,
  zoomable canvas in React. He details the challenges he faced and the solutions
  he implemented, such as decoupling the desired pan and zoom state from the
  canvas component itself and creating a React context that reported the user's
  desired pan and zoom state. He also explains how he built the panning state
  using a `usePan` hook and the scaling state using a `useScale` hook. Finally,
  he demonstrates how to create the illusion of a pannable, zoomable canvas by
  manipulating the canvas's background offset for panning and the canvas's scale
  for scaling.
---

<details>
  <summary>A quick note, since I've gotten a few questions about this. The code
  for this post is not on GitHub. Feel free to use the code in this post under
  the MIT license:</summary>

  <article class="mt-2 text-xs font-mono">
    Copyright (c) 2020 Jonathan Clem

    Permission is hereby granted, free of charge, to any person obtaining a
    copy of this software and associated documentation files (the "Software"),
    to deal in the Software without restriction, including without limitation
    the rights to use, copy, modify, merge, publish, distribute, sublicense,
    and/or sell copies of the Software, and to permit persons to whom the
    Software is furnished to do so, subject to the following conditions: The
    above copyright notice and this permission notice shall be included in all
    copies or substantial portions of the Software.

    THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
    IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
    FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
    AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
    LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
    OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
    SOFTWARE.
  </article>
</details>

Recently, I was tinkering on a side-project where I wanted to build a sort of
canvas of very large dimensions that I could zoom in on and pan around, similar
to zooming and panning around in a map application.

<iframe src="https://example-use-pan.vercel.app/#tracking" title="Final canvas demo" ></iframe>

In this post, I'm going to detail how I built this in React and what challenges
I had to overcome in doing so. The components I was building were only intended
to be used in a desktop browser, so on touch-enabled devices, the examples have
been replaced with illustrative video clips.

In my first attempts at building this pannable and zoomable canvas, I bound the
canvas's pan and zoom state directly to the canvas's DOM element itself.
Ultimately, this caused a lot problems, because there were certain elements
visually laid out on the canvas that I either did not want to scale, or did not
want to pan (such as some user interface elements on the canvas).

Ultimately, I decided to try an approach that decoupled the desired pan and zoom
state entirely from the canvas component itself. Instead of binding the pan and
zoom state to the canvas, I wanted to create a React context that reported the
user's _desired_ pan and zoom state, but didn't actually manipulate the DOM in
any way.

Here's a simplified explanation of what I wanted in code:

```tsx
export type CanvasState {
  // The pan state
  offset: {x: number, y: number}
  // The zoom state
  scale: number
}

export const CanvasContext = React.createContext<CanvasState>({} as any)
```

---

```tsx
function SomeCanvasComponent() {
  const { state } = useContext(CanvasContext);
  return <div>The desired user zoom level is {state.scale}.</div>;
}
```

Here, you can see that `CanvasContext` doesn't do any direct manipulation of the
DOM. It just tells `SomeCanvasComponent` that the user _wants_ the scale to be
at some value, and leaves it up to that component to actually reflect that
desired state.

My first step was to implement the panning state. To do this, I implemented a
`usePan` hook that that tracked the user panning around a component.

Essentially, `usePan` is a hook that returns a pan offset state and a function.
The function should be called whenever the user starts a pan on the target
element (usually a `mousedown` event). On each `mousemove` event until a
`mouseup` occurs and we remove our event listeners, we calculate the delta
between the last observed mouse position on `mousemove` and the current event's
mouse position. Then, we apply that delta to our offset state.

One quick detail—you may wonder why the `mousemove` and `mouseup` event
listeners in this hook are bound to `document` and not to the target element
specified by the user. This is because we want to ensure that any mouse movement
by the user whatsoever while panning, even if not over the target element
itself, still pans the canvas. For example, on pages like this blog post where
the canvas is contained within another bounding element, we don't want panning
to stop just because the user's mouse happened to leave the bounding element.

Here is the `usePan` hook (note that you'll see the `Point` type and `ORIGIN`
constant referenced in other places in this blog post):

```typescript
import {
  MouseEvent as SyntheticMouseEvent,
  useCallback,
  useRef,
  useState,
} from "react";

type Point = { x: number; y: number };
const ORIGIN = Object.freeze({ x: 0, y: 0 });

/**
 * Track the user's intended panning offset by listening to `mousemove` events
 * once the user has started panning.
 */
export default function usePan(): [Point, (e: SyntheticMouseEvent) => void] {
  const [panState, setPanState] = useState<Point>(ORIGIN);

  // Track the last observed mouse position on pan.
  const lastPointRef = useRef(ORIGIN);

  const pan = useCallback((e: MouseEvent) => {
    const lastPoint = lastPointRef.current;
    const point = { x: e.pageX, y: e.pageY };
    lastPointRef.current = point;

    // Find the delta between the last mouse position on `mousemove` and the
    // current mouse position.
    //
    // Then, apply that delta to the current pan offset and set that as the new
    // state.
    setPanState((panState) => {
      const delta = {
        x: lastPoint.x - point.x,
        y: lastPoint.y - point.y,
      };
      const offset = {
        x: panState.x + delta.x,
        y: panState.y + delta.y,
      };

      return offset;
    });
  }, []);

  // Tear down listeners.
  const endPan = useCallback(() => {
    document.removeEventListener("mousemove", pan);
    document.removeEventListener("mouseup", endPan);
  }, [pan]);

  // Set up listeners.
  const startPan = useCallback(
    (e: SyntheticMouseEvent) => {
      document.addEventListener("mousemove", pan);
      document.addEventListener("mouseup", endPan);
      lastPointRef.current = { x: e.pageX, y: e.pageY };
    },
    [pan, endPan]
  );

  return [panState, startPan];
}
```

Let's use the `usePan` hook in a simple example that will just show us how much
we've panned around total. Note that in this and other examples, I'm omitting
styling for clarity:

```tsx
export const UsePanExample = () => {
  const [offset, startPan] = usePan();

  return (
    <div onMouseDown={startPan}>
      <span>{JSON.stringify(offset)}</span>
    </div>
  );
};
```

<iframe src="https://example-use-pan.vercel.app/#use-pan" title="Panning canvas demo" ></iframe>

If you click on this example and drag around, you'll see a persistent measure of
how far you've dragged both horizontally and vertically.

As you can see, this isn't really panning an element since neither it nor our
viewport are moving, but later we'll see how these values can be used to
simulate panning in various ways depending on our needs.

Now that I had the basics of panning state down, I needed to tackle scaling. For
scaling, I decided to implement a hook called `useScale`. Much like `usePan`, it
doesn't actually _do_ any scaling or zooming. Instead, it listens on certain
events and reports back what it thinks the user _intends_ for the current scale
level to be.[^1]

```typescript
import { RefObject, useState } from "react";
import useEventListener from "./useEventListener";

type ScaleOpts = {
  direction: "up" | "down";
  interval: number;
};

const MIN_SCALE = 0.5;
const MAX_SCALE = 3;

/**
 * Listen for `wheel` events on the given element ref and update the reported
 * scale state, accordingly.
 */
export default function useScale(ref: RefObject<HTMLElement | null>) {
  const [scale, setScale] = useState(1);

  const updateScale = ({ direction, interval }: ScaleOpts) => {
    setScale((currentScale) => {
      let scale: number;

      // Adjust up to or down to the maximum or minimum scale levels by `interval`.
      if (direction === "up" && currentScale + interval < MAX_SCALE) {
        scale = currentScale + interval;
      } else if (direction === "up") {
        scale = MAX_SCALE;
      } else if (direction === "down" && currentScale - interval > MIN_SCALE) {
        scale = currentScale - interval;
      } else if (direction === "down") {
        scale = MIN_SCALE;
      } else {
        scale = currentScale;
      }

      return scale;
    });
  };

  // Set up an event listener such that on `wheel`, we call `updateScale`.
  useEventListener(ref, "wheel", (e) => {
    e.preventDefault();

    updateScale({
      direction: e.deltaY > 0 ? "up" : "down",
      interval: 0.1,
    });
  });

  return scale;
}
```

Let's use the `useScale` hook in an example:

```tsx
export const UseScaleExample = () => {
  const ref = useRef<HTMLDivElement | null>(null);
  const scale = useScale(ref);

  return (
    <div ref={ref}>
      <span>{scale}</span>
    </div>
  );
};
```

<iframe src="https://example-use-pan.vercel.app/#use-scale" title="Scaling canvas demo" ></iframe>

If you scroll up and down inside the example's bounding box, you should see the
scale value update.

Now that we have our `usePan` and `useScale` hooks, how do we actually create a
pannable, zoomable canvas? Or rather, how do we create the _illusion_ of a
pannable, zoomable canvas? For my particular use case, I knew that I could
create the illusion of panning and scaling by manipulating the canvas's
background offset for panning, and the canvas's scale for scaling, rather than
actually trying to move the element itself around.

```tsx
export const UsePanScaleExample = () => {
  const [offset, startPan] = usePan();
  const ref = useRef<HTMLDivElement | null>(null);
  const scale = useScale(ref);

  return (
    <div ref={ref} onMouseDown={startPan}>
      <div
        style={{
          backgroundImage: "url(/grid.svg)",
          transform: `scale(${scale})`,
          backgroundPosition: `${-offset.x}px ${-offset.y}px`,
        }}
      ></div>
    </div>
  );
};
```

<iframe src="https://example-use-pan.vercel.app/#use-pan-scale" title="Panning and scaling canvas demo" ></iframe>

We're _on our way_, but not quite there! In this example, panning seems to work
fine! The background position updates according to the reported `offset` from
`usePan`. Scaling kind of works, but unfortunately, as we scale the element
down, we end up exposing a buffer between its edges and its bounding box. It
doesn't really feel like we're zooming in and out on the canvas so much as it
feels like we're zooming in and out on our tiny window into the canvas.

In order to solve this, I decided to use calculate a buffer based on the
bounding box around the canvas itself. This buffer represents horizontal and
vertical space we need to fill between the bounding box and what would normally
be the edge of the zoomed out canvas. We can calculate this buffer for each side
every time the scale changes using the formula:

$$
xbuf = (boundingWidth - boundingWidth / scale) / 2
$$

In plain English, the buffer to apply to each
horizontal side is equal to one half of the width of the bounding element minus
the width of the bounding element divided by the current scale. The same holds
true for for the vertical sides, only one would use the bounding element's
height.

```tsx
export const BufferExample = () => {
  const [buffer, setBuffer] = useState(pointUtils.ORIGIN);
  const [offset, startPan] = usePan();
  const ref = useRef<HTMLDivElement | null>(null);
  const scale = useScale(ref);

  useLayoutEffect(() => {
    const height = ref.current?.clientHeight ?? 0;
    const width = ref.current?.clientWidth ?? 0;

    // This is the application of the above formula!
    setBuffer({
      x: (width - width / scale) / 2,
      y: (height - height / scale) / 2,
    });
  }, [scale, setBuffer]);

  return (
    <div ref={ref} onMouseDown={startPan} style={{ position: "relative" }}>
      <div
        style={{
          backgroundImage: "url(/grid.svg)",
          transform: `scale(${scale})`,
          backgroundPosition: `${-offset.x}px ${-offset.y}px`,
          position: "absolute",
          bottom: buffer.y,
          left: buffer.x,
          right: buffer.x,
          top: buffer.y,
        }}
      ></div>
    </div>
  );
};
```

<iframe src="https://example-use-pan.vercel.app/#buffer" title="Buffer canvas demo" ></iframe>

In this example, we absolutely position the actual canvas background DOM node
within the bounding element and use the calculated buffer values to set the
`top`, `right`, `bottom`, and `left` positions. Essentially, if the user has
scaled out to `0.5` and the bounding element is 100 pixels wide, we know that we
need to set the `left` and `right` positions out 50 pixels further than usual,
so we set them each to `-50px`.

Now, we have a canvas that feels infinite in size (barring integer overflow)
that we can pan and zoom on. I was proud of this accomplishment, but something
still didn't feel _quite_ right about it. You'll notice that when you zoom, the
focal point is always the top-left corner of the bounding container—you're
_always_ going to be zooming in on that corner, and then would have to pan to
your destination (or place it in the corner before zooming). I have seen other
implementations solve this by always just zooming into the exact center of the
canvas, where the user may be somewhat more likely to have placed the area they
are trying to focus in on. To me, this still felt like it was putting a lot of
burden on the user to position things in the exact right place before zooming.

Instead, I wanted to find the point to zoom in on dynamically, under the
assumption that the mouse pointer is probably pointing at the thing the user is
trying to focus on. In order to accomplish this, I made several important
changes to the component.

First, I implemented a hook called `useMousePos`. I won't show its source code
here, but essentially it just returns a ref whose value is the last known
position of the user's mouse pointer, based on listening to `mousemove` and
`wheel` events on the canvas's bounding container.

Then, I implemented a hook called `useLast`. This hook maintains a reference to
the previous value passed to it and returns that last known value. This is just
an easy way to track the last known value of `scale` and `offset` and the
current value. We'll get to why this is necessary shortly.

Finally, calculating the adjusted offset based on the user's mouse position as
the user scales is where things get really tricky. In order to store this
adjusted value, I create a container ref for it called `adjustedOffset`. If on
any given render the scale has not changed (`lastScale === scale`), we set the
adjusted offset to be the sum of the current adjusted offset and the delta
between the current and last _non_-adjusted offset, scaled according to our
current `scale` value. In other words, when the scale has _not_ changed:

$$
adjOffset = adjOffset + (offsetDelta / scale)
$$

Keep in mind that the math operations here are on points, so \\(+\\) is summing the
\\(x\\) and \\(y\\) values of each point. We "scale" a point by dividing by the scale
value (so if the user pans by 10 pixels but scale is `0.5`, we adjust that delta
value to `10 / 0.5`, yielding 20 pixels at our current scale).

When we want to get the adjusted offset when the scale _has_ changed, we need to
do things a little differently, because we now want to ensure that the focal
point of the change in scale is the user's mouse pointer. In other words, as we
scale (as long as the user is not panning at the same time), the point on the
canvas directly under the user's mouse pointer _should not change_. First, we
get the mouse position adjusted according to the _last known scale value_:

$$
lastMouse = mousePos / lastScale
$$

Then, we get the mouse position adjusted according to the _current scale value_:

$$
newMouse = mousePos / scale
$$

Next, we calculate how much the mouse has moved relative to our canvas as a
result of the scaling by subtracting the \\(newMouse\\) value from the \\(lastMouse\\)
value. This will tell us how much we need to adjust our offset by in order to
compensate for the change in relative position of the mouse pointer to the
canvas as a result of the scaling:

$$
mouseOffset = lastMouse - newMouse
$$

Finally, we set apply this offset by adding the \\(mouseOffset\\) we calculated to
the current adjusted offset value:

$$
adjOffset = adjOffset + mouseOffset
$$

Rather than using our offset provided by `usePan`, we now use our new adjusted
offset, which maintains our pan offset relative to the user's mouse position as
they zoom in and out.

```tsx
export const TrackingExample = () => {
  const [buffer, setBuffer] = useState(pointUtils.ORIGIN);
  const ref = useRef<HTMLDivElement | null>(null);
  const [offset, startPan] = usePan();
  const scale = useScale(ref);

  // Track the mouse position.
  const mousePosRef = useMousePos(ref);

  // Track the last known offset and scale.
  const lastOffset = useLast(offset);
  const lastScale = useLast(scale);

  // Calculate the delta between the current and last offset—how far the user has panned.
  const delta = pointUtils.diff(offset, lastOffset);

  // Since scale also affects offset, we track our own "real" offset that's
  // changed by both panning and zooming.
  const adjustedOffset = useRef(pointUtils.sum(offset, delta));

  if (lastScale === scale) {
    // No change in scale—just apply the delta between the last and new offset
    // to the adjusted offset.
    adjustedOffset.current = pointUtils.sum(
      adjustedOffset.current,
      pointUtils.scale(delta, scale)
    );
  } else {
    // The scale has changed—adjust the offset to compensate for the change in
    // relative position of the pointer to the canvas.
    const lastMouse = pointUtils.scale(mousePosRef.current, lastScale);
    const newMouse = pointUtils.scale(mousePosRef.current, scale);
    const mouseOffset = pointUtils.diff(lastMouse, newMouse);
    adjustedOffset.current = pointUtils.sum(
      adjustedOffset.current,
      mouseOffset
    );
  }

  useLayoutEffect(() => {
    const height = ref.current?.clientHeight ?? 0;
    const width = ref.current?.clientWidth ?? 0;

    setBuffer({
      x: (width - width / scale) / 2,
      y: (height - height / scale) / 2,
    });
  }, [scale, setBuffer]);

  return (
    <div ref={ref} onMouseDown={startPan} style={{ position: "relative" }}>
      <div
        style={{
          backgroundImage: "url(/grid.svg)",
          transform: `scale(${scale})`,
          backgroundPosition: `${-adjustedOffset.current.x}px ${-adjustedOffset
            .current.y}px`,
          position: "absolute",
          bottom: buffer.y,
          left: buffer.x,
          right: buffer.x,
          top: buffer.y,
        }}
      ></div>
    </div>
  );
};
```

<iframe src="https://example-use-pan.vercel.app/#tracking" title="Final canvas demo" ></iframe>

In this example, notice that as you zoom in and out, the focal point always
remains the mouse cursor, even if you pan and zoom simultaneously, or move the
mouse as you zoom.

With this final addition, I had something that felt very natural to use, much
like a maps application. I was surprised at the complexity required to build a
good user experience for something that seems relatively simple—surely, there
are simplifications that could be made here and probably a few bugs in the React
code, as well.

Now, I was ready to wrap this component up into a context. Thankfully, that was
pretty easy!

```tsx
export type CanvasState {
  offset: Point
  buffer: Point
  scale: number
}

export const CanvasContext = React.createContext<CanvasState>({} as any)

export default function CanvasProvider(props: PropsWithChildren<unknown>) {
  // Insert here all of the hooks from the previous example!

  return (
    <CanvasContext.Provider
      value={{
        offset: adjustedOffset.current,
        scale,
        buffer
      }}
    >
      <div ref={ref} onMouseDown={startPan} style={{position: 'relative'}}>
        {props.children}
      </div>
    </CanvasContext.Provider>
  )
}
```

And to consume the context to get the grid effect in the prior example:

```tsx
export default function GridBackground() {
  const { offset, buffer, scale } = useContext(CanvasContext);

  return (
    <div
      style={{
        backgroundImage: "url(/grid.svg)",
        transform: `scale(${scale})`,
        backgroundPosition: `${-offset.x}px ${-offset.y}px`,
        position: "absolute",
        bottom: buffer.y,
        left: buffer.x,
        right: buffer.x,
        top: buffer.y,
      }}
    ></div>
  );
}
```

Now that we have our basic canvas container and context set up, there's a lot
more we can do just by consuming the desired state of the canvas view. In a
future blog post, I hope to show how I've also implemented a feature where cards
can be added to this canvas by command-clicking at the desired position.

Thanks for reading!

[^1]: You may be wondering why I'm manually attaching the `wheel` event instead of using a React `onWheel` listener. React has some surpising behavior and [some bugs](https://github.com/facebook/react/issues/14856) related to how it handles wheel events on components, so I am avoiding them by manually attaching an event listener. In this case I'm using a convenience hook called `useEventListener` that is just responsible for manually setting up and tearing down an event listener on a DOM node attached to the given ref.
