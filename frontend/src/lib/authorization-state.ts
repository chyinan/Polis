// pattern: Imperative Shell

export const AUTHORIZATION_INVALIDATED_EVENT = 'polis:authorization-invalidated';

const rejectedScopes = new Set<string>();
const authorizationChannelName = 'polis:authorization-state';
const authorizationStorageKey = 'polis:authorization-invalidated';

function notifyCurrentTab(): void {
  if (typeof window === 'undefined') return;
  window.dispatchEvent(new Event(AUTHORIZATION_INVALIDATED_EVENT));
}

let authorizationChannel: BroadcastChannel | null = null;
if (typeof window !== 'undefined' && typeof BroadcastChannel !== 'undefined') {
  try {
    authorizationChannel = new BroadcastChannel(authorizationChannelName);
    authorizationChannel.addEventListener('message', event => {
      if (event.data === AUTHORIZATION_INVALIDATED_EVENT) notifyCurrentTab();
    });
  } catch {
    authorizationChannel = null;
  }
}

if (typeof window !== 'undefined') {
  window.addEventListener('storage', event => {
    if (event.key === authorizationStorageKey && event.newValue !== null) notifyCurrentTab();
  });
}

export function observeProtectedResponse(scope: string, status: number): void {
  if (status === 401 || status === 403) {
    invalidateProtectedScope(scope);
    return;
  }
  if (status >= 200 && status < 300) rejectedScopes.delete(scope);
}

export function invalidateProtectedScope(scope: string): void {
  if (rejectedScopes.has(scope)) return;
  rejectedScopes.add(scope);
  if (rejectedScopes.size > 128) {
    const oldest = rejectedScopes.values().next().value;
    if (oldest !== undefined) rejectedScopes.delete(oldest);
  }
  dispatchAuthorizationInvalidated();
}

export function dispatchAuthorizationInvalidated(): void {
  if (typeof window === 'undefined') return;
  notifyCurrentTab();
  try {
    if (authorizationChannel !== null) {
      authorizationChannel.postMessage(AUTHORIZATION_INVALIDATED_EVENT);
      return;
    }
    window.localStorage.setItem(authorizationStorageKey, `${Date.now()}-${Math.random()}`);
  } catch {
    // A current-tab invalidation remains effective if cross-tab messaging is unavailable.
  }
}
