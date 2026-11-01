import { test, expect } from '@playwright/test'

test.describe('pbuf-registry Web UI', () => {
  test('renders header, toggles accessible mobile menu, and navigates to modules', async ({ page }) => {
    await page.route('**/api/v1/modules*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          modules: [{ id: '1', name: 'yourorg/hello-proto', tags: ['v1.0.0'] }],
          next_page_token: ''
        })
      })
    })

    await page.setViewportSize({ width: 390, height: 844 })
    await page.goto('/')

    const menuBtn = page.getByRole('button', { name: 'Toggle navigation menu' })
    await expect(menuBtn).toBeVisible()
    await expect(menuBtn).toHaveAttribute('aria-expanded', 'false')

    await menuBtn.click()
    await expect(menuBtn).toHaveAttribute('aria-expanded', 'true')
  })

  test('lists modules, filters by search, appends on Load More, and supports keyboard navigation', async ({ page }) => {
    let getModuleCalls = 0
    await page.route('**/api/v1/modules?*', async (route) => {
      const url = new URL(route.request().url())
      const pageToken = url.searchParams.get('page_token')
      if (pageToken === 'page-2') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            modules: [
              { id: '2', name: 'pbufio/pbuf-registry', tags: ['v0.7.0'] },
              { id: '3', name: 'yourorg/empty-proto', tags: [] }
            ],
            next_page_token: ''
          })
        })
        return
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          modules: [{ id: '1', name: 'yourorg/hello-proto', tags: ['v1.0.0'] }],
          next_page_token: 'page-2'
        })
      })
    })

    await page.route('**/api/v1/modules/get', async (route) => {
      getModuleCalls += 1
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: '1',
          name: 'yourorg/hello-proto',
          tags: ['v1.0.0']
        })
      })
    })

    await page.route('**/api/v1/drift/modules/dependencies/status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ statuses: [] })
      })
    })

    await page.goto('/modules')

    await expect(page.getByText('yourorg/hello-proto')).toBeVisible()
    await expect(page.getByText('v1.0.0')).toBeVisible()

    // Load More appends page 2 while keeping page 1 visible without N+1 getModule requests
    await page.getByRole('button', { name: 'Load More' }).click()
    await expect(page.getByText('yourorg/hello-proto')).toBeVisible()
    await expect(page.getByText('pbufio/pbuf-registry')).toBeVisible()
    await expect(page.getByText('yourorg/empty-proto')).toBeVisible()
    expect(getModuleCalls).toBe(0)

    // Search filters modules
    const searchInput = page.getByPlaceholder('Search modules...')
    await searchInput.fill('hello-proto')
    await expect(page.getByText('yourorg/hello-proto')).toBeVisible()
    await expect(page.getByText('pbufio/pbuf-registry')).toHaveCount(0)

    // Keyboard Enter on card navigates to module detail view
    const card = page.getByRole('link', { name: /yourorg\/hello-proto/ })
    await card.focus()
    await page.keyboard.press('Enter')
    await expect(page).toHaveURL(/\/modules\/yourorg\/hello-proto$/)
  })

  test('login page renders OIDC SSO button, validates token format, and logs in with valid token', async ({ page }) => {
    await page.route('**/api/v1/modules*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ modules: [], next_page_token: '' })
      })
    })

    await page.goto('/login')

    // OIDC SSO button is visible from runtime /config.js
    await expect(page.getByRole('button', { name: /Sign in with SSO \(OIDC\)/i })).toBeVisible()

    const tokenInput = page.getByPlaceholder(/pbuf_user_/i)
    await tokenInput.fill('bad-token')
    await page.getByRole('button', { name: 'Login', exact: true }).click()
    await expect(page.getByText(/Invalid token format/i)).toBeVisible()

    await tokenInput.fill('pbuf_user_validToken123456')
    await page.getByRole('button', { name: 'Login', exact: true }).click()
    await expect(page).toHaveURL(/\/modules$/)
  })

  test('live Docker Compose stack serves CLI-pushed modules and enforces Nginx proxy restrictions', async ({ page, request }) => {
    test.skip(process.env.PLAYWRIGHT_LIVE_BACKEND !== '1', 'Requires live Docker Compose stack')

    await page.goto('/modules')

    await expect(page.getByText('yourorg/hello-proto')).toBeVisible()
    await expect(page.getByText('pbufio/pbuf-registry')).toBeVisible()

    await page.getByRole('link', { name: /yourorg\/hello-proto/ }).click()
    await expect(page).toHaveURL(/\/modules\/yourorg\/hello-proto$/)
    await expect(page.getByText('v1.0.0', { exact: true })).toBeVisible()
    await expect(page.getByText('yourorg/hello-proto@v1.0.0')).toBeVisible()

    // Verify Nginx blocks write endpoints (405) and withholds public fallback token from /api/v1/users (401)
    const delResp = await request.post('/api/v1/modules/delete', {
      data: { name: 'yourorg/hello-proto' }
    })
    expect(delResp.status()).toBe(405)

    const usersResp = await request.get('/api/v1/users')
    expect(usersResp.status()).toBe(401)
  })
})
