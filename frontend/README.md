# Frontend — React SPA

Clean, professional UI for the URL shortener. Built with React 18, TypeScript, Vite, and Tailwind CSS.

## Structure

```
frontend/
├── src/
│   ├── App.tsx               Root component — layout + state
│   ├── api.ts                Typed fetch wrapper for all backend calls
│   ├── types.ts              TypeScript interfaces matching Go API shapes
│   ├── history.ts            localStorage helper (stores last 10 URLs)
│   └── components/
│       ├── ShortenForm.tsx   URL input, custom alias, expiry options
│       ├── ResultCard.tsx    Short URL result with copy button
│       ├── AnalyticsModal.tsx  Click count + timestamps modal
│       ├── HistoryPanel.tsx  Recent URLs from localStorage
│       ├── CopyButton.tsx    Clipboard copy with visual feedback
│       └── HealthBadge.tsx   Live system status in navbar
├── nginx.conf                Reverse proxy — routes /api, /health, /{code} to backend
├── Dockerfile                Node build → Nginx serve (two-stage)
└── package.json
```

## Running locally

```bash
npm install
npm run dev     # http://localhost:3000
```

Vite proxies `/api` and `/health` to `http://localhost:8080` — the Go backend must be running.

## Building for production

```bash
npm run build   # outputs to dist/
```

In Docker, Nginx serves the `dist/` folder and proxies backend requests to the `app` container by service name.

## Features

| Feature | Notes |
|---|---|
| Shorten any URL | With optional custom alias (3–50 chars) and expiry |
| Copy to clipboard | Falls back gracefully on non-HTTPS |
| Analytics modal | Polls `/api/v1/analytics/{code}` — shows click count, dates |
| History panel | Last 10 URLs in localStorage — survives page refresh |
| Health badge | Polls `/health` every 30s — green/red dot in navbar |
| Dark theme | Slate-950 background, Tailwind utility classes throughout |
