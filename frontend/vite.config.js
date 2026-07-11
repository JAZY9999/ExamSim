import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Le proxy redirige les appels /api et /ws vers le backend Go en développement,
// ce qui évite les soucis de CORS et reproduit le comportement de production.
export default defineConfig({
  plugins: [react()],
  server: {
    host: true,
    port: 5173,
    proxy: {
      '/api': {
        target: process.env.VITE_API_TARGET || 'http://localhost:8080',
        changeOrigin: true,
      },
      '/ws': {
        target: process.env.VITE_API_TARGET || 'http://localhost:8080',
        changeOrigin: true,
        ws: true,
      },
    },
  },
})
