<script setup>
import { RouterLink } from 'vue-router'
</script>

<template>
  <footer class="border-t border-surface-border bg-surface-card/30 mt-auto">
    <div class="max-w-7xl mx-auto px-4 py-12">
      <div class="grid grid-cols-1 md:grid-cols-4 gap-8 mb-8">
        <!-- Brand Section -->
        <div class="col-span-1 md:col-span-2">
          <div class="flex items-center gap-2 mb-4">
            <div class="w-8 h-8 bg-brand rounded-lg flex items-center justify-center">
              <span class="text-white font-bold text-lg">P</span>
            </div>
            <span class="text-xl font-bold text-zinc-50">pbuf<span class="text-brand">.io</span></span>
          </div>
          <p class="text-zinc-400 text-sm max-w-md">
            A modern, open-source Protocol Buffers registry for managing and versioning your .proto files with ease.
          </p>
        </div>

        <!-- Resources -->
        <div>
          <h3 class="text-zinc-50 font-semibold mb-4">Resources</h3>
          <ul class="space-y-2">
            <li>
              <RouterLink to="/modules" class="text-zinc-400 hover:text-brand transition-colors text-sm">
                Browse Modules
              </RouterLink>
            </li>
            <li>
              <a
                href="https://github.com/pbufio/pbuf-cli"
                target="_blank"
                rel="noopener noreferrer"
                class="text-zinc-400 hover:text-brand transition-colors text-sm"
              >
                CLI Docs
              </a>
            </li>
          </ul>
        </div>

        <!-- Community -->
        <div>
          <h3 class="text-zinc-50 font-semibold mb-4">Community</h3>
          <ul class="space-y-2">
            <li>
              <a
                href="https://github.com/pbufio"
                target="_blank"
                rel="noopener noreferrer"
                class="text-zinc-400 hover:text-brand transition-colors text-sm"
              >
                GitHub
              </a>
            </li>
            <li>
              <a href="https://x.com/pbufio" target="_blank" rel="noopener noreferrer" class="text-zinc-400 hover:text-brand transition-colors text-sm">
                X (Twitter)
              </a>
            </li>
            <li>
              <a href="mailto:hello@pbuf.io" class="text-zinc-400 hover:text-brand transition-colors text-sm">
                Email
              </a>
            </li>
          </ul>
        </div>
      </div>

      <!-- Bottom Bar -->
      <div class="pt-8 border-t border-surface-border flex flex-col md:flex-row justify-between items-center gap-4">
        <p class="text-zinc-500 text-sm">
          © {{ new Date().getFullYear() }} pbuf.io. All rights reserved.
        </p>
        <div class="flex gap-6">
          <a href="https://x.com/pbufio" target="_blank" rel="noopener noreferrer" class="text-zinc-500 hover:text-brand transition-colors text-sm">
            X
          </a>
          <a href="mailto:hello@pbuf.io" class="text-zinc-500 hover:text-brand transition-colors text-sm">
            hello@pbuf.io
          </a>
        </div>
      </div>
    </div>
  </footer>
</template>
