/// <reference types="vite/client" />

type WorkbenchMode = 'fixture' | 'real';

interface ImportMetaEnv {
  readonly VITE_WORKBENCH_MODE?: WorkbenchMode;
  readonly VITE_WORKBENCH_API_BASE_URL?: string;
  readonly VITE_WORKBENCH_COMPANY_ID?: string;
  readonly VITE_WORKBENCH_LIVE_UPDATES?: 'deferred' | 'sse';
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
