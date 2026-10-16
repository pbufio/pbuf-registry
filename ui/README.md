# pbuf.io Registry UI

A modern, developer-first web interface for the pbuf.io Protocol Buffers registry. Built with Vue.js 3, Vite, and Tailwind CSS.

## Features

- 🎨 **Modern Dark Theme** - Developer-first aesthetic with high contrast and technical clarity
- 📦 **Module Browsing** - Browse and search all registered protobuf modules
- 🏷️ **Version Management** - View and manage module tags and versions
- 📄 **Proto File Viewer** - Inspect proto file contents and structure
- 🔍 **Metadata Explorer** - Browse parsed protobuf messages, services, and fields
- 🔗 **Dependency Tracking** - View module dependencies and their relationships
- 📱 **Fully Responsive** - Optimized for desktop, tablet, and mobile devices

## Tech Stack

- **Vue.js 3.4.21** - Progressive JavaScript framework with Composition API
- **Vite 5.2.8** - Next-generation frontend build tool with HMR
- **Tailwind CSS 3.4.3** - Utility-first CSS framework
- **Vue Router 4.3.0** - Official router for Vue.js SPA
- **Axios 1.6.8** - Promise-based HTTP client for API communication

## Design System

The UI follows a "Developer-First" design aesthetic:

- **Colors**: Deep dark theme (Zinc-950) with Blue-500 brand accent
- **Typography**: Inter for UI, JetBrains Mono for code
- **Components**: Minimalist with subtle shadows and borders
- **Interactions**: Smooth transitions and hover effects

## Getting Started

### Prerequisites

- **Node.js 18+** and **npm** (Node.js 20 recommended for production builds)
- **API Access** - Valid authentication token for pbuf.cloud API
- **Git** - For cloning the repository

### Quick Start

1. **Clone the repository:**
```bash
git clone <repository-url>
cd pbuf-registry-ui
```

2. **Install dependencies:**
```bash
npm install
```

3. **Configure environment variables:**
```bash
cp .env.example .env
```

Edit `.env` and configure:
- `API_BASE_URL` - Backend API endpoint (default: `http://pbuf.cloud`)
- `API_TOKEN` - Your **raw** pbuf registry token (optional in public mode)
- `PUBLIC_ENABLED` - Enable public (unauthenticated) browsing + optional token login

**Important:**
- When `API_TOKEN` is set, it is used server-side only by the Vite dev proxy and nginx in production. It is never exposed to the browser.
- If the browser sends an `Authorization` header (token login), the proxies **forward it** to the backend. If the browser does not send one, the proxies fall back to `API_TOKEN` (when configured).
- The UI always uses **Bearer tokens** (`Authorization: Bearer <token>`). Provide `API_TOKEN` as a raw token (without the `Bearer ` prefix).

If `PUBLIC_ENABLED` is set, you can also login via the UI at `/login`, which stores the token in `sessionStorage` and sends it on requests.

4. **Start the development server:**
```bash
npm run dev
```

5. **Open your browser:**
```
http://localhost:5173
```

### Available Scripts

| Command | Description |
|---------|-------------|
| `npm run dev` | Start Vite development server with HMR on port 5173 |
| `npm run build` | Build production bundle to `dist/` directory |
| `npm run preview` | Preview production build locally |

### Build for Production

Build the application for production deployment:

```bash
npm run build
```

The optimized static files will be output to the `dist/` directory, ready to be served by any static file server or CDN.

## Docker Deployment

The application includes a production-ready multi-stage Dockerfile with nginx as the web server.

### Build Docker Image

```bash
docker build -t pbuf-registry-ui:latest .
```

The build process:
1. Uses Node 20 Alpine to build the application with `npm ci` and `npm run build`
2. Creates production image with nginx Alpine
3. Copies built assets from build stage
4. Configures nginx with custom entrypoint for runtime environment variable substitution

### Run Docker Container

```bash
docker run -d \
  -p 8080:80 \
  -e API_BASE_URL=http://pbuf.cloud \
  -e API_TOKEN=your_raw_token_here \
  -e PUBLIC_ENABLED=true \
  --name pbuf-ui \
  pbuf-registry-ui:latest
```

The application will be available at `http://localhost:8080`.

### Docker Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `API_BASE_URL` | No | `http://pbuf.cloud` | Backend API endpoint to proxy requests to |
| `API_TOKEN` | No | (empty) | Raw registry token used as `Authorization: Bearer <token>` by the proxy |
| `PUBLIC_ENABLED` | No | (empty/false) | Enable public browsing + optional token login via UI |

### Public mode + token login

- Set `PUBLIC_ENABLED=true` to allow using the UI without a server-side token.
- Users can then open `/login` and paste a token; it will be stored in `sessionStorage` for the session.

**Note:** Environment variables are substituted at container startup by the `docker-entrypoint.sh` script using `envsubst`.

### Docker Features

- **Health Check Endpoint:** `GET /health` returns `200 OK` for monitoring
- **Gzip Compression:** Enabled for text assets (HTML, CSS, JS, JSON)
- **Static Asset Caching:** 1-year cache for immutable assets
- **Security Headers:** X-Frame-Options, X-Content-Type-Options, X-XSS-Protection
- **SPA Routing:** All routes fallback to `index.html` for Vue Router
- **API Proxy:** `/api/*` requests proxied to backend with auth header injection

### Production Nginx Configuration

The nginx configuration includes:
- **Method Restrictions:** Only specific endpoints allow POST (metadata, dependencies, get, pull)
- **Proxy Pass:** Strips `/api` prefix and forwards to `API_BASE_URL`
- **Authorization Header:** Automatically added from `API_TOKEN` environment variable
- **Timeouts:** 60s for connect, send, and read operations

## Project Structure

```
pbuf-registry-ui/
├── src/
│   ├── assets/          # Static assets and global styles
│   ├── components/      # Reusable Vue components (P* prefix = shared components)
│   │   ├── PButton.vue
│   │   ├── PCodeBlock.vue
│   │   ├── PFeatureCard.vue
│   │   ├── PInput.vue
│   │   ├── PCard.vue
│   │   ├── PBadge.vue
│   │   ├── PTable.vue
│   │   ├── AppHeader.vue
│   │   └── AppFooter.vue
│   ├── views/           # Page components (Vue Router views)
│   │   ├── HomeView.vue           # Landing page
│   │   ├── ModulesView.vue        # Module list/browse
│   │   ├── ModuleDetailView.vue   # Single module details
│   │   └── ModuleVersionView.vue  # Module version viewer
│   ├── services/        # API service layer (Axios instances)
│   │   ├── registryApi.js   # Module registry operations
│   │   └── metadataApi.js   # Protobuf metadata parsing
│   ├── router/          # Vue Router configuration
│   │   └── index.js
│   ├── App.vue          # Root component
│   └── main.js          # Application entry point
├── index.html           # HTML entry point
├── vite.config.js       # Vite configuration with proxy setup
├── tailwind.config.js   # Tailwind CSS configuration
├── postcss.config.js    # PostCSS configuration
├── package.json         # Project dependencies and scripts
├── Dockerfile           # Multi-stage Docker build
├── docker-entrypoint.sh # Runtime environment variable substitution
├── nginx.conf           # Production nginx configuration template
└── .env.example         # Environment variable template

```

### Key Architecture Decisions

- **@ Alias:** `vite.config.js` configures `@` as alias for `src/` directory
- **API Services:** Centralized Axios instances in `services/` for clean separation
- **Component Prefix:** `P*` components are part of the shared component library
- **SPA Routing:** Vue Router handles client-side navigation
- **Proxy Pattern:** Both dev (Vite) and prod (nginx) use identical proxy configuration

## API Integration

The UI connects to the pbuf registry API endpoints:

### Registry API (`/v1/modules`)
- List all modules
- Get module details
- Register new modules
- Push module versions
- Pull module versions
- Delete modules and tags
- Get module dependencies

### Metadata API (`/v1/metadata`)
- Get parsed protobuf metadata
- View messages, services, fields, and enums

## Component Library

### Core Components

- **PButton** - Primary and secondary action buttons
- **PCodeBlock** - Terminal-style code display with copy functionality
- **PFeatureCard** - Feature highlight cards with icons
- **PInput** - Form input with dark theme styling
- **PCard** - Generic card container
- **PBadge** - Tags and version labels
- **PTable** - Data table with custom cell rendering

### Layout Components

- **AppHeader** - Navigation header with logo and menu
- **AppFooter** - Footer with links and branding

## Development

### Recommended IDE Setup

- [VSCode](https://code.visualstudio.com/)
- [Volar](https://marketplace.visualstudio.com/items?itemName=Vue.volar) extension

### Code Style

The project follows Vue.js 3 best practices:
- Composition API with `<script setup>`
- Single File Components (SFC)
- Tailwind CSS utility classes
- Responsive design patterns

## Architecture Overview

### Proxy Server Architecture

**Security Design:** The frontend never directly connects to pbuf.cloud. All API calls are proxied through a backend server to avoid exposing authentication tokens in the browser.

```
[Browser] ---> [Proxy Server] ---> [pbuf.cloud API]
           /api/*          + Auth Header
```

#### Development Mode (Vite Proxy)

When running `npm run dev`, Vite's built-in proxy server handles API requests:

1. Frontend makes requests to `/api/*` (e.g., `/api/v1/modules`)
2. Vite proxy intercepts these requests
3. Strips `/api` prefix: `/api/v1/modules` → `/v1/modules`
4. Adds `Authorization` header using `API_TOKEN` from `.env`
5. Forwards to `API_BASE_URL` (default: `http://pbuf.cloud`)
6. Returns the response to the frontend

**Configuration:** Edit `.env` file (copy from `.env.example`):
```bash
API_BASE_URL=http://pbuf.cloud
API_TOKEN=your_token_here
```

**Implementation:** See `vite.config.js` proxy configuration with `proxyReq` event handler.

#### Production Mode (Nginx Proxy)

When running in Docker, nginx acts as the proxy server with identical behavior:

1. Browser requests `/api/*`
2. Nginx intercepts and strips `/api` prefix
3. Adds `Authorization` header from `API_TOKEN` environment variable
4. Forwards to `API_BASE_URL`
5. Returns response to browser

**Configuration:** Set environment variables when running Docker container:
```bash
docker run -e API_BASE_URL=... -e API_TOKEN=... pbuf-registry-ui
```

**Implementation:** See `nginx.conf` and `docker-entrypoint.sh` for runtime substitution.

### Environment Variables

| Variable | Context | Required | Default | Description |
|----------|---------|----------|---------|-------------|
| `API_BASE_URL` | Dev & Prod | No | `http://pbuf.cloud` | Backend API endpoint for proxy target |
| `API_TOKEN` | Dev & Prod | Yes | (none) | Authentication token for pbuf.cloud API (server-side only) |

**Important Security Notes:**
- Environment variables are used **server-side only** (Vite proxy in dev, nginx in prod)
- `API_TOKEN` is **never exposed** to the browser or included in the built JavaScript bundle
- No `VITE_*` prefixed variables are needed (Vite variables would be embedded in the bundle)

## Troubleshooting

### Common Issues

#### API Authentication Errors (401/403)

**Problem:** Browser console shows 401 or 403 errors when accessing `/api/*` endpoints.

**Solution:**
1. Verify `API_TOKEN` is set correctly in `.env` file
2. Ensure `.env` file is in the project root directory
3. Restart the dev server after changing `.env` (`npm run dev`)
4. Check that the token is valid and not expired

#### Proxy Connection Errors (ECONNREFUSED)

**Problem:** Dev server shows connection refused errors or cannot reach backend.

**Solution:**
1. Verify `API_BASE_URL` is correct in `.env` (default: `http://pbuf.cloud`)
2. Check if the backend API is accessible from your machine
3. Try using the full URL with port if needed: `http://pbuf.cloud:8080`
4. Check firewall or network restrictions

#### Docker Container Won't Start

**Problem:** Container exits immediately or nginx fails to start.

**Solution:**
1. Check container logs: `docker logs pbuf-ui`
2. Verify environment variables are passed correctly
3. Test nginx config: `docker run --rm pbuf-registry-ui nginx -t`
4. Ensure `API_TOKEN` doesn't contain special characters that need escaping

#### Blank Page After Build

**Problem:** Production build shows blank page or routing doesn't work.

**Solution:**
1. Check browser console for errors
2. Ensure your web server is configured for SPA routing (fallback to `index.html`)
3. Verify all assets are being served correctly
4. Check that `dist/` directory was built successfully

### Development Tips

- **Hot Module Replacement (HMR):** Vite provides instant feedback on code changes
- **Vue DevTools:** Install the Vue.js DevTools browser extension for component inspection
- **API Debugging:** Check Network tab in browser DevTools to inspect `/api/*` requests
- **Environment Check:** Run `console.log(import.meta.env)` to debug environment variables (note: only `VITE_*` vars are exposed)

## Contributing

### Development Workflow

1. **Fork & Clone:** Fork the repository and clone your fork
2. **Create Branch:** Create a feature branch from `main`
3. **Install Dependencies:** Run `npm install`
4. **Make Changes:** Follow the existing code style and patterns
5. **Test Locally:** Ensure dev server runs without errors
6. **Commit:** Write clear, descriptive commit messages
7. **Push & PR:** Push to your fork and open a pull request

### Code Style Guidelines

- Use **Composition API** with `<script setup>` syntax
- Follow **Vue 3 best practices** and component naming conventions
- Use **Tailwind utility classes** instead of custom CSS when possible
- Keep components **small and focused** (single responsibility)
- Add **comments** for complex logic or non-obvious behavior
- Maintain **consistent formatting** (prettier/eslint compatible)

### Project Conventions

- **Component Naming:** Use `PascalCase` for component files (e.g., `PButton.vue`)
- **Component Prefix:** Shared components use `P` prefix (e.g., `PCard`, `PButton`)
- **View Naming:** Page views use `*View.vue` suffix (e.g., `HomeView.vue`)
- **Import Alias:** Use `@/` for imports from `src/` directory
- **API Services:** Centralize API calls in `services/` directory

## License

This project is part of the pbuf.io ecosystem.

## Contributing

Contributions are welcome! Please feel free to submit pull requests.
