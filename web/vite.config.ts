import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// The browser only talks to the Vite dev server (same origin), which forwards these
// path prefixes to the FastAPI backend. No CORS, no backend change.
// In compose the dev container sets BACKEND_URL=http://backend:8000.
const backend = process.env.BACKEND_URL ?? 'http://localhost:8000'

export default defineConfig({
  plugins: [react()],
  server: {
    host: true, // listen on 0.0.0.0 so a container's published port can reach it
    proxy: {
      '/vehicles': backend,
      '/commands': backend,
    },
  },
})
