import {defineConfig, loadEnv} from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig(({mode}) => {
  const env = loadEnv(mode, '.', '');
  const runtimeEnv = (globalThis as unknown as {process?: {env?: Record<string, string | undefined>}}).process?.env;
  const backendUrl = env.VITE_WORKBENCH_BACKEND_URL ?? runtimeEnv?.VITE_WORKBENCH_BACKEND_URL ?? 'http://127.0.0.1:8080';
  return {
    plugins: [react()],
    server: {
      port: 4173,
      proxy: {
        '/api': {
          target: backendUrl,
        },
      },
    },
  };
});
