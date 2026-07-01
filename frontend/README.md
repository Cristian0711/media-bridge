# Media Bridge — Frontend

SvelteKit app with Tailwind CSS and shadcn-svelte.

## Dev

```bash
npm install
npm run dev
```

Open http://localhost:5173

## Add to iOS Home Screen

1. Open the site in **Safari** (required on iOS).
2. Tap **Share** (square with arrow).
3. Tap **Add to Home Screen**.
4. Confirm — the app opens full-screen like a native app.

## Stack

- **SvelteKit** — routing and layout
- **Tailwind CSS v4** — styling (design tokens from `design/tabbars.jsx`)
- **shadcn-svelte** — `Button` primitive (`src/lib/components/ui/button`)
- **lucide-svelte** — tab icons

## Navigation

Floating liquid-glass tab bar (Home / Search / Settings) on all routes. Route shells only — no page content yet.
