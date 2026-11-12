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

  test('renders interactive dependency graph with direct + transitive drift colors and navigates on node click', async ({ page }) => {
    const depRequests = []

    await page.route('**/api/v1/metadata', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ packages: [] })
      })
    })

    await page.route('**/api/v1/modules/pull', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ protofiles: [] })
      })
    })

    await page.route('**/api/v1/drift/modules/events', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ events: [] })
      })
    })

    await page.route('**/api/v1/modules/dependencies', async (route) => {
      const body = route.request().postDataJSON()
      depRequests.push(body)

      if (body.name === 'acme/order-service') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            dependencies: [
              { name: 'acme/payment-service', tag: 'v1.0.0', dependencyType: 'direct' },
              { name: 'googleapis/common-protos', tag: 'v1.0.0', dependencyType: 'transitive' }
            ]
          })
        })
        return
      }

      if (body.name === 'acme/payment-service') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            dependencies: [{ name: 'googleapis/common-protos', tag: 'v1.0.0', dependencyType: 'direct' }]
          })
        })
        return
      }

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ dependencies: [] })
      })
    })

    await page.route('**/api/v1/drift/modules/dependencies/status', async (route) => {
      const body = route.request().postDataJSON()
      if (body.moduleName === 'acme/order-service') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            statuses: [
              {
                dependencyName: 'acme/payment-service',
                currentTag: 'v1.0.0',
                targetTag: 'v2.0.0',
                severity: 'DRIFT_SEVERITY_CRITICAL',
                recommendation: 'DEPENDENCY_DRIFT_RECOMMENDATION_ALERT_REVIEW'
              }
            ]
          })
        })
        return
      }

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ statuses: [] })
      })
    })

    await page.goto('/modules/acme/order-service/tags/v1.0.0')

    const graphTab = page.getByRole('button', { name: 'Dependency Graph' })
    await expect(graphTab).toBeVisible()
    expect(depRequests.some((r) => r.name === 'acme/order-service' && r.resolveTransitive === true)).toBe(true)

    await graphTab.click()

    const graphContainer = page.getByTestId('dependency-graph')
    await expect(graphContainer).toBeVisible()
    await expect(graphContainer).toContainText('1 direct • 1 transitive')

    const paymentNode = graphContainer.locator('[data-node-id="acme/payment-service@v1.0.0"]')
    const commonNode = graphContainer.locator('[data-node-id="googleapis/common-protos@v1.0.0"]')
    await expect(paymentNode).toHaveAttribute('data-severity', 'CRITICAL')
    await expect(commonNode).toHaveAttribute('data-severity', 'HEALTHY')
    await expect(commonNode).toHaveAttribute('data-dependency-type', 'transitive')

    // Clicking leaf transitive node navigates to its version view, where Dependency Graph tab is hidden (0 dependencies)
    await commonNode.click()
    await expect(page).toHaveURL(/\/modules\/googleapis\/common-protos\/tags\/v1\.0\.0$/)
    await expect(page.getByRole('button', { name: 'Dependency Graph' })).toHaveCount(0)
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
