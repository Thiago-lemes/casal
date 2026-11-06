/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'
import { fileURLToPath } from 'node:url'

export default defineConfig({
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: {
    // porta fixa: o F5 abre o navegador nela; se estiver ocupada, falha em vez de subir em outra
    port: 5173,
    strictPort: true,
    // mesma origem em desenvolvimento: o cookie de sessão funciona sem CORS
    proxy: { '/v1': 'http://localhost:8080', '/admin/v1': 'http://localhost:8080', '/healthz': 'http://localhost:8080' },
  },
  plugins: [
    react(),
    VitePWA({
      strategies: 'injectManifest',
      srcDir: 'src/shared/pwa',
      filename: 'sw.ts',
      registerType: 'prompt',
      injectRegister: false,
      manifest: {
        name: 'Zoog',
        short_name: 'Zoog',
        start_url: '/',
        display: 'standalone',
        background_color: '#15171B',
        theme_color: '#12204A',
        icons: [
          { src: '/icon-192.png', sizes: '192x192', type: 'image/png' },
          { src: '/icon-512.png', sizes: '512x512', type: 'image/png' },
        ],
      },
      devOptions: { enabled: true, type: 'module' },
    }),
  ],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
  },
})
