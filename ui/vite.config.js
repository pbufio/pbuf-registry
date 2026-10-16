import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'
import { selectProxyAuthorizationHeader } from './src/utils/proxyAuthUtils.js'

const runtimeConfigDevPlugin = (env) => {
  return {
    name: 'pbuf-runtime-config-dev',
    apply: 'serve',
    configureServer(server) {
      server.middlewares.use('/config.js', (req, res) => {
        const cfg = {
          PUBLIC_ENABLED: env.PUBLIC_ENABLED ?? env.VITE_PUBLIC_ENABLED ?? '',
          OIDC_ENABLED: env.OIDC_ENABLED ?? env.VITE_OIDC_ENABLED ?? '',
          OIDC_ISSUER: env.OIDC_ISSUER ?? env.VITE_OIDC_ISSUER ?? '',
          OIDC_CLIENT_ID: env.OIDC_CLIENT_ID ?? env.VITE_OIDC_CLIENT_ID ?? '',
          OIDC_REDIRECT_URI: env.OIDC_REDIRECT_URI ?? env.VITE_OIDC_REDIRECT_URI ?? '',
          OIDC_SCOPES: env.OIDC_SCOPES ?? env.VITE_OIDC_SCOPES ?? ''
        }

        res.statusCode = 200
        res.setHeader('Content-Type', 'application/javascript; charset=utf-8')
        res.setHeader('Cache-Control', 'no-store')
        res.end(`window.__APP_CONFIG__ = ${JSON.stringify(cfg, null, 2)};\n`)
      })
    }
  }
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')

  const publicEnabled = env.PUBLIC_ENABLED ?? env.VITE_PUBLIC_ENABLED ?? ''

  return {
    plugins: [runtimeConfigDevPlugin(env), vue()],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url))
      }
    },
    server: {
      proxy: {
        '/api': {
          target: env.API_BASE_URL,
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/api/, ''),
          configure: (proxy, options) => {
            proxy.on('proxyReq', (proxyReq, req, res) => {
              const headerValue = selectProxyAuthorizationHeader({
                incomingAuthorization: req.headers?.authorization,
                apiToken: env.API_TOKEN,
                publicEnabled,
                method: req.method,
                url: req.url
              })

              if (headerValue) {
                proxyReq.setHeader('Authorization', headerValue)
              }
            })
          }
        }
      }
    }
  }
})
