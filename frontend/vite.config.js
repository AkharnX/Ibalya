import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// En dev (npm run dev), les appels /api sont relayés vers le backend Go.
// En prod, `npm run build` produit dist/ que le backend sert directement.
export default defineConfig({
  // L'application est servie sous /app : les ressources doivent s'y référer.
  base: '/app/',
  plugins: [react()],
  server: {
    proxy: { '/api': 'http://127.0.0.1:9999' },
  },
  build: { outDir: 'dist' },
})
