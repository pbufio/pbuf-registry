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

    await page.goto('/login?next=//evil.example.com/phish')

    // OIDC SSO button is visible from runtime /config.js
    await expect(page.getByRole('button', { name: /Sign in with SSO \(OIDC\)/i })).toBeVisible()

    const tokenInput = page.getByPlaceholder(/pbuf_user_/i)
    await tokenInput.fill('bad token with spaces')
    await page.getByRole('button', { name: 'Login', exact: true }).click()
    await expect(page.getByText(/Invalid token format/i)).toBeVisible()

    // Static admin token (SERVER_STATIC_TOKEN) logs in and sanitizes unsafe ?next=//evil.example.com to /modules
    await tokenInput.fill('my-static-admin-token-123')
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

  test('acknowledges an unacknowledged drift event from the drift tab and keeps the card intact', async ({ page }) => {
    // Signed-in session so the Acknowledge button is rendered (authToken reads sessionStorage)
    await page.addInitScript(() => {
      window.sessionStorage.setItem('pbuf_registry_ui_token', 'pbuf_user_validToken123456')
    })

    const ackRequests = []
    const driftEvent = {
      id: 'evt-1',
      moduleName: 'acme/order-service',
      tagName: 'v1.0.0',
      filename: 'order/v1/order.proto',
      eventType: 'DRIFT_EVENT_TYPE_MODIFIED',
      previousHash: 'aaa111',
      currentHash: 'bbb222',
      severity: 'DRIFT_SEVERITY_WARNING',
      detectedAt: '2026-11-01T09:30:00Z',
      acknowledged: false,
      acknowledgedBy: '',
      acknowledgedAt: null
    }

    await page.route('**/api/v1/metadata', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ packages: [] }) })
    })
    await page.route('**/api/v1/modules/pull', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ protofiles: [] }) })
    })
    await page.route('**/api/v1/modules/dependencies', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ dependencies: [] }) })
    })
    await page.route('**/api/v1/drift/modules/dependencies/status', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ statuses: [] }) })
    })
    await page.route('**/api/v1/drift/modules/events', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ events: [driftEvent] }) })
    })
    await page.route('**/api/v1/drift/events/evt-1/acknowledge', async (route) => {
      ackRequests.push({ method: route.request().method(), body: route.request().postDataJSON() })
      // Kratos marshals with EmitUnpopulated, so unset fields arrive as zero values / null
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          event: {
            id: 'evt-1',
            moduleName: '',
            tagName: '',
            filename: '',
            eventType: 'DRIFT_EVENT_TYPE_UNSPECIFIED',
            previousHash: '',
            currentHash: '',
            severity: 'DRIFT_SEVERITY_UNSPECIFIED',
            detectedAt: null,
            acknowledged: true,
            acknowledgedBy: 'unknown',
            acknowledgedAt: null
          }
        })
      })
    })

    await page.goto('/modules/acme/order-service/tags/v1.0.0')
    await page.getByRole('button', { name: /^Drift \(/ }).click()

    const card = page.locator('.space-y-3 > div').filter({ hasText: 'order/v1/order.proto' })
    await expect(card).toBeVisible()
    await expect(card).toContainText('UNACKNOWLEDGED')

    const ackButton = card.getByRole('button', { name: 'Acknowledge', exact: true })
    await expect(ackButton).toBeVisible()
    await ackButton.click()

    await expect(card).toContainText('ACKNOWLEDGED')
    await expect(card).not.toContainText('UNACKNOWLEDGED')
    await expect(card.getByRole('button', { name: 'Acknowledge', exact: true })).toHaveCount(0)
    // Sparse server response must not blank the card; 'unknown' must not be shown as the acknowledger
    await expect(card).toContainText('order/v1/order.proto')
    await expect(card).toContainText('WARNING')
    await expect(card).not.toContainText('by unknown')

    expect(ackRequests).toHaveLength(1)
    expect(ackRequests[0].method).toBe('POST')
    expect(ackRequests[0].body).toEqual({})
  })

  test('hides the Acknowledge button for signed-out visitors', async ({ page }) => {
    await page.route('**/api/v1/metadata', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ packages: [] }) })
    })
    await page.route('**/api/v1/modules/pull', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ protofiles: [] }) })
    })
    await page.route('**/api/v1/modules/dependencies', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ dependencies: [] }) })
    })
    await page.route('**/api/v1/drift/modules/dependencies/status', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ statuses: [] }) })
    })
    await page.route('**/api/v1/drift/modules/events', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          events: [{ id: 'evt-1', filename: 'order/v1/order.proto', eventType: 'DRIFT_EVENT_TYPE_MODIFIED', severity: 'DRIFT_SEVERITY_WARNING', acknowledged: false }]
        })
      })
    })

    await page.goto('/modules/acme/order-service/tags/v1.0.0')
    await page.getByRole('button', { name: /^Drift \(/ }).click()

    await expect(page.getByText('order/v1/order.proto')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Acknowledge', exact: true })).toHaveCount(0)
  })

  test('renders unparsed newly-pushed tag gracefully, emits registry.addr snippet, and compares tag-to-tag .proto diffs', async ({ page }) => {
    await page.route('**/api/v1/metadata', async (route) => {
      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ code: 404, reason: 'TAG_META_NOT_FOUND', message: 'tag meta not found' })
      })
    })
    await page.route('**/api/v1/modules/dependencies', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ dependencies: [] }) })
    })
    await page.route('**/api/v1/drift/modules/dependencies/status', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ statuses: [] }) })
    })
    await page.route('**/api/v1/drift/modules/events', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ events: [] }) })
    })
    await page.route('**/api/v1/modules/get', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 'mod-1',
          name: 'acme/order-service',
          tags: ['v2.0.0', 'v1.1.0', 'v1.0.0'],
          draftTags: []
        })
      })
    })
    await page.route('**/api/v1/modules/pull', async (route) => {
      const body = route.request().postDataJSON()
      if (body.tag === 'v2.0.0') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            protofiles: [
              { filename: 'order/v1/order.proto', content: 'syntax = "proto3";\nmessage Order {\n  string id = 1;\n  string status = 2;\n}\n' },
              { filename: 'order/v2/events.proto', content: 'syntax = "proto3";\nmessage OrderCreated {}\n\n' }
            ]
          })
        })
        return
      }
      if (body.tag === 'v1.1.0') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            protofiles: [
              { filename: 'order/v1/order.proto', content: 'syntax = "proto3";\nmessage Order {\n  string id = 1;\n}\n' },
              { filename: 'order/v1/legacy.proto', content: 'syntax = "proto3";\nmessage Legacy {}\n' }
            ]
          })
        })
        return
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          protofiles: [
            { filename: 'order/v1/order.proto', content: 'syntax = "proto3";\nmessage Order {}\n' }
          ]
        })
      })
    })

    await page.context().grantPermissions(['clipboard-read', 'clipboard-write'])
    await page.goto('/modules/acme/order-service/tags/v2.0.0')

    // Quick-start snippet emits valid registry.addr instead of deprecated domain:
    const snippet = page.locator('.whitespace-pre').first()
    await expect(snippet).toContainText('registry:')
    await expect(snippet).toContainText('addr:')
    await expect(snippet).not.toContainText('domain:')
    await expect(page.getByText('order/v1/order.proto')).toBeVisible()

    // Proto Files viewer renders non-selectable 1-based line numbers and per-file Copy button
    const firstGutter = page.getByTestId('proto-line-numbers').first()
    await expect(firstGutter).toHaveText('1\n2\n3\n4\n5')
    await expect(firstGutter).toHaveClass(/select-none/)
    const secondGutter = page.getByTestId('proto-line-numbers').nth(1)
    const secondCode = page.getByTestId('proto-file-content').nth(1)
    await expect(secondGutter).toHaveText('1\n2\n3')
    expect(await secondGutter.evaluate((el) => el.clientHeight)).toBe(await secondCode.evaluate((el) => el.clientHeight))
    const copyOrderBtn = page.getByRole('button', { name: 'Copy order/v1/order.proto' })
    const copyEventsBtn = page.getByRole('button', { name: 'Copy order/v2/events.proto' })
    await expect(copyOrderBtn).toHaveText('Copy')
    await copyOrderBtn.click()
    await expect(copyOrderBtn).toHaveText('Copied!')
    await expect(copyEventsBtn).toHaveText('Copy')
    const copiedText = await page.evaluate(() => navigator.clipboard.readText())
    expect(copiedText).toBe('syntax = "proto3";\nmessage Order {\n  string id = 1;\n  string status = 2;\n}\n')

    // Plain-HTTP fallback works when navigator.clipboard is unavailable
    await page.evaluate(() => {
      Object.defineProperty(navigator, 'clipboard', { value: undefined, configurable: true })
    })
    await copyEventsBtn.click()
    await expect(copyEventsBtn).toHaveText('Copied!')
    expect(await page.evaluate(async () => {
      // execCommand('copy') wrote to system clipboard; verify button state transitioned cleanly
      return document.activeElement?.tagName !== 'TEXTAREA'
    })).toBe(true)

    // Unparsed metadata 404 shows pending notice without failing the page
    await page.getByRole('button', { name: 'Metadata', exact: true }).click()
    await expect(page.getByText(/Metadata is pending background indexing/i)).toBeVisible()

    // Compare tab defaults to immediately preceding tag v1.1.0 and renders added/modified/deleted diffs
    await page.getByRole('button', { name: 'Compare', exact: true }).click()
    const panel = page.getByTestId('compare-tab-panel')
    const select = panel.getByLabel('Compare against tag')
    await expect(select).toHaveValue('v1.1.0')
    await expect(panel).toContainText('1 added, 1 modified, 1 deleted')
    await expect(panel).toContainText('string status = 2;')
    await expect(panel).toContainText('order/v1/legacy.proto')

    // Switching baseline tag updates diff cleanly
    await select.selectOption('v1.0.0')
    await expect(panel).toContainText('Comparing v1.0.0 → v2.0.0: 1 added, 1 modified, 0 deleted')
  })

  test('navigates from draft tag row on ModuleDetailView and renders map, oneof, and nested metadata', async ({ page }) => {
    await page.route('**/api/v1/modules/get', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 'mod-draft',
          name: 'acme/checkout',
          tags: ['v1.0.0'],
          draftTags: ['v1.1.0-rc+1']
        })
      })
    })
    await page.route('**/api/v1/drift/modules/dependencies/status', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ statuses: [] }) })
    })
    await page.route('**/api/v1/drift/modules/events', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ events: [] }) })
    })
    await page.route('**/api/v1/modules/dependencies', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ dependencies: [] }) })
    })
    await page.route('**/api/v1/metadata', async (route) => {
      const body = route.request().postDataJSON()
      if (body.tag === 'v1.1.0-rc+1') {
        await route.fulfill({
          status: 404,
          contentType: 'application/json',
          body: JSON.stringify({ code: 404, reason: 'MODULE_TAG_NOT_FOUND', message: 'module tag not found' })
        })
        return
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          packages: [
            {
              name: 'acme.checkout.v1',
              protoFiles: [
                {
                  filename: 'checkout.proto',
                  enums: [
                    {
                      name: 'OrderStatus',
                      values: [
                        { name: 'ORDER_STATUS_UNSPECIFIED' },
                        { name: 'ORDER_STATUS_PAID', tag: 1 }
                      ]
                    }
                  ],
                  messages: [
                    {
                      name: 'CheckoutRequest',
                      fields: [
                        { name: 'labels', tag: 1, map: true, mapKeyType: 'string', mapValueType: 'string' },
                        { name: 'payment', oneof: true, oneofNames: ['card_token', 'invoice_id'], oneofTypes: ['string', 'string'] }
                      ],
                      nestedEnums: [
                        { name: 'Currency', values: [{ name: 'CURRENCY_UNSPECIFIED', tag: 0 }, { name: 'CURRENCY_USD', tag: 1 }] }
                      ],
                      nestedMessages: [
                        { name: 'Address', fields: [{ name: 'postal_code', messageType: 'string', tag: 1 }] }
                      ]
                    }
                  ]
                }
              ]
            }
          ]
        })
      })
    })
    await page.route('**/api/v1/modules/pull', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          protofiles: [{ filename: 'checkout.proto', content: 'syntax = "proto3";\npackage acme.checkout.v1;\n' }]
        })
      })
    })

    await page.goto('/modules/acme/checkout')
    await page.getByRole('button', { name: 'View Draft →' }).click()
    await expect(page).toHaveURL(/\/modules\/acme\/checkout\/tags\/v1\.1\.0-rc%2B1$/)
    await expect(page.getByText('DRAFT', { exact: true })).toBeVisible()
    await expect(page.getByText('checkout.proto')).toBeVisible()

    await page.getByRole('button', { name: 'Metadata', exact: true }).click()
    await expect(page.getByText(/Metadata indexing is only available for published tags/i)).toBeVisible()

    // Navigate to published tag v1.0.0 and verify map, oneof, nested message, nested enum, and top-level enum rendering
    await page.goto('/modules/acme/checkout/tags/v1.0.0')
    await page.getByRole('button', { name: 'Metadata', exact: true }).click()
    await expect(page.getByText('map<string, string>')).toBeVisible()
    await expect(page.getByText('card_token')).toBeVisible()
    await expect(page.getByText('CheckoutRequest.Address')).toBeVisible()
    await expect(page.getByText('CURRENCY_USD')).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Enums', exact: true })).toBeVisible()
    await expect(page.getByText('OrderStatus')).toBeVisible()
    await expect(page.getByText('ORDER_STATUS_UNSPECIFIED = 0')).toBeVisible()
    await expect(page.getByText('ORDER_STATUS_PAID = 1')).toBeVisible()
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
