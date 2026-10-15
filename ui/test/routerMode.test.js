import test from 'node:test'
import assert from 'node:assert/strict'

import { getAuthRedirect } from '../src/router/authGuard.js'
import { buildRootRoute } from '../src/router/routeUtils.js'

test('getAuthRedirect: allows all routes in public mode', () => {
  const redirect = getAuthRedirect({
    publicEnabled: true,
    to: { path: '/modules', fullPath: '/modules' },
    isLoggedIn: false
  })

  assert.equal(redirect, null)
})

test('getAuthRedirect: allows /login in private mode', () => {
  const redirect = getAuthRedirect({
    publicEnabled: false,
    to: { path: '/login', fullPath: '/login' },
    isLoggedIn: false
  })

  assert.equal(redirect, null)
})

test('getAuthRedirect: redirects to /login with next in private mode when not logged in', () => {
  const redirect = getAuthRedirect({
    publicEnabled: false,
    to: { path: '/modules', fullPath: '/modules?page=1' },
    isLoggedIn: false
  })

  assert.deepEqual(redirect, { path: '/login', query: { next: '/modules?page=1' } })
})

test('getAuthRedirect: does not redirect when logged in (private mode)', () => {
  const redirect = getAuthRedirect({
    publicEnabled: false,
    to: { path: '/modules', fullPath: '/modules' },
    isLoggedIn: true
  })

  assert.equal(redirect, null)
})

test('buildRootRoute: uses HomeView as root route in public mode', () => {
  const HomeView = { name: 'HomeView' }
  const route = buildRootRoute({ publicEnabled: true, HomeView })

  assert.equal(route.path, '/')
  assert.equal(route.name, 'home')
  assert.equal(route.component, HomeView)
})

test('buildRootRoute: redirects / to /modules in private mode', () => {
  const route = buildRootRoute({ publicEnabled: false, HomeView: {} })
  assert.deepEqual(route, { path: '/', redirect: '/modules' })
})
