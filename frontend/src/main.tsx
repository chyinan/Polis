// pattern: Imperative Shell

import {StrictMode} from 'react';
import {createRoot} from 'react-dom/client';
import {App} from './app/App';
import './styles/tokens.css';
import './styles/globals.css';

const root = document.getElementById('root');
if (root === null) {
  throw new Error('failed to mount Polis workbench: root element is missing');
}

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
