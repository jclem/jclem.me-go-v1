import type { Config } from 'tailwindcss'
import plugin from 'tailwindcss/plugin'
import forms from '@tailwindcss/forms'

export default {
  content: ["internal/pages/*.md", "internal/posts/*.md", "internal/www/view/templates/**/*.tmpl", "internal/markdown/markdown.go"],
  safelist: [
    "code-example",
    "footnote",
    "footnotes",
    "reversefootnote",
    "img-figure",
    "vid-figure",
  ],
  theme: {
    extend: {
      colors: {
        border: "var(--color-border)",
        canvas: "var(--color-canvas)",
        card: "var(--color-card)",
        "card-text": "var(--color-card-text)",
        text: "var(--color-text)",
        "text-deemphasize": "var(--color-text-deemphasize)",
        "code-bg": "var(--color-code-bg)",
        highlight: "var(--color-highlight)",
      },

      fontFamily: {
        mono: ["Berkeley Mono", "monospace"],
        sans: ["Hanken Grotesk", "sans-serif"],
      },
    },
  },
  plugins: [
    forms,
    // Allows prefixing tailwind classes with LiveView classes to add rules
    // only when LiveView classes are applied, for example:
    //
    //     <div class="phx-click-loading:animate-ping">
    //
    plugin(({ addVariant }) =>
      addVariant("phx-no-feedback", [".phx-no-feedback&", ".phx-no-feedback &"])
    ),
    plugin(({ addVariant }) =>
      addVariant("phx-click-loading", [
        ".phx-click-loading&",
        ".phx-click-loading &",
      ])
    ),
    plugin(({ addVariant }) =>
      addVariant("phx-submit-loading", [
        ".phx-submit-loading&",
        ".phx-submit-loading &",
      ])
    ),
    plugin(({ addVariant }) =>
      addVariant("phx-change-loading", [
        ".phx-change-loading&",
        ".phx-change-loading &",
      ])
    ),
  ],
} satisfies Config

