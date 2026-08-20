import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const backendPort = env.VITE_BACKEND_PORT || '8003'

  return {
    plugins: [vue()],
    server: {
      allowedHosts: ['inkpot.cn'],
      host: '0.0.0.0',
      port: 5175,
      proxy: {
        '/api': {
          target: `http://localhost:${backendPort}`,
          changeOrigin: true,
        },
      },
    },
  }
})
