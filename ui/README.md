# pbuf.io Registry Web UI (`ui/`)

A modern, developer-first web interface for [`pbuf-registry`](https://github.com/pbufio/pbuf-registry). Built with Vue 3, Vite, and Tailwind CSS.

## Features

- 🎨 **Modern Dark Theme** — Developer-first aesthetic with high contrast and technical clarity
- 📦 **Module Browsing** — Browse and search all registered protobuf modules
- 🏷️ **Version Management** — View module tags, draft tags, and drift badges
- 📄 **Proto File Viewer** — Inspect `.proto` file contents and structure
- 🔍 **Metadata Explorer** — Browse parsed protobuf messages, services, enums, and fields
- 🔗 **Dependency & Drift Tracking** — View module dependencies, breaking change alerts, and upgrade recommendations
- 🔐 **Flexible Authentication** — Supports public read-only browsing, server-side proxy token, browser token login, and OIDC/OAuth2 SSO (Authorization Code + PKCE)
- 📱 **Fully Responsive** — Optimized for desktop, tablet, and mobile devices

## Tech Stack

- **Vue 3** — Composition API (`<script setup>`)
- **Vite 5** — Frontend build tool with HMR and dev proxy
- **Tailwind CSS 3** — Utility-first CSS framework
- **Vue Router 4** — SPA routing
- **Axios** — HTTP client for API communication
- **Node.js Test Runner & Playwright** — Unit and end-to-end browser testing

## Getting Started (Local Development)

### Prerequisites

- **Node.js 20+** and **npm**
- Running [`pbuf-registry`](../README.md) instance (or remote endpoint)

### Quick Start

1. **Navigate to the `ui/` directory in the monorepo:**
   ```bash
   cd ui
   npm install
   ```

2. **Configure environment variables:**
   ```bash
   cp .env.example .env
   ```

   Edit `.env`:
   - `API_BASE_URL` — Backend HTTP API endpoint (e.g., `http://localhost:8080` or `https://pbuf.cloud`)
   - `API_TOKEN` — Optional server-side registry token injected by the proxy
   - `PUBLIC_ENABLED` — Set to `true` to enable unauthenticated public browsing + `/login` page
   - `OIDC_ENABLED` — Set to `true` to enable "Sign in with SSO (OIDC)" on `/login`
   - `OIDC_ISSUER_URL` — OIDC identity provider issuer URL
   - `OIDC_CLIENT_ID` — OIDC public client ID
   - `OIDC_REDIRECT_URI` — Optional explicit redirect URI (defaults to `<origin>/login`)
   - `OIDC_SCOPES` — OIDC scopes (default: `openid profile email`)

3. **Start the development server:**
   ```bash
   npm run dev
   ```
   Open `http://localhost:5173`.

### Available Scripts

| Command | Description |
|---|---|
| `npm run dev` | Start Vite development server with HMR on port 5173 |
| `npm run build` | Build production bundle to `dist/` (also `make build-ui` from repo root) |
| `npm run preview` | Preview production build locally |
| `npm test` | Run unit tests with Node.js test runner (also `make test-ui` from repo root) |
| `npm run test:e2e` | Run Playwright browser E2E tests (also `make test-ui-e2e` from repo root) |

## Docker Deployment

### Using Pre-Built Image from GHCR

```bash
docker run -d \
  -p 8080:80 \
  -e API_BASE_URL=http://your-registry:8080 \
  -e API_TOKEN=your_raw_token_here \
  -e PUBLIC_ENABLED=true \
  --name pbuf-ui \
  ghcr.io/pbufio/pbuf-registry-ui:latest
```

### Building Locally

From the repository root:

```bash
make build-docker-ui
```

Or directly inside `ui/`:

```bash
docker build -t ghcr.io/pbufio/pbuf-registry-ui:latest .
```

### Runtime Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `API_BASE_URL` | No | `http://pbuf.cloud` | Backend HTTP API endpoint to proxy `/api/*` requests to |
| `API_TOKEN` | No | `""` | Server-side fallback token injected as `Authorization: Bearer <token>` on public read endpoints |
| `PUBLIC_ENABLED` | No | `false` | Allow unauthenticated browsing in the UI and interactive login at `/login` |
| `OIDC_ENABLED` | No | `false` | Enable OIDC/OAuth2 Authorization Code + PKCE SSO login |
| `OIDC_ISSUER_URL` | No | `""` | OIDC Provider Issuer URL |
| `OIDC_CLIENT_ID` | No | `""` | OIDC Client ID |
| `OIDC_REDIRECT_URI` | No | `""` | Custom OIDC redirect URI (defaults to `<window.location.origin>/login`) |
| `OIDC_SCOPES` | No | `openid profile email` | Space-delimited OIDC scopes |

### Proxy & Security Architecture

```text
[Browser] ---> [Vite Dev Proxy / Nginx Prod Proxy] ---> [pbuf-registry HTTP API (:8080)]
           /api/*                                   /v1/* + Authorization Header
```

- All `/api/*` requests are proxied to `API_BASE_URL` with `/api` stripped.
- If the browser provides an `Authorization` header (from token login or OIDC SSO stored in `sessionStorage`), the proxy forwards the user's header directly.
- If the browser does not send an `Authorization` header and `API_TOKEN` is configured on the server, the proxy injects `Authorization: Bearer ${API_TOKEN}` **only** for public read-only endpoints (`ListModules`, `GetModule`, `PullModule`, `GetModuleDependencies`, `GetMetadata`, `GetModuleDriftEvents`, `GetModuleDependencyDriftStatus`).
- `API_TOKEN` is never exposed to the browser or embedded in the frontend bundle.
- In production (`nginx.conf`), HTTP methods are restricted to `GET`, `HEAD`, `OPTIONS`, and the specific read-only `POST` endpoints used by the registry API.
