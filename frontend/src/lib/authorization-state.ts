// pattern: Imperative Shell

export const AUTHORIZATION_INVALIDATED_EVENT = 'polis:authorization-invalidated';

const rejectedScopes = new Set<string>();

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
  window.dispatchEvent(new Event(AUTHORIZATION_INVALIDATED_EVENT));
}
