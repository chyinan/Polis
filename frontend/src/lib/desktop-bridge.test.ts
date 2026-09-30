// pattern: Imperative Shell

import {afterEach, describe, expect, it, vi} from 'vitest';

const {invokeMock} = vi.hoisted(() => ({
  invokeMock: vi.fn<(command: string, args?: Record<string, unknown>) => Promise<unknown>>(),
}));

vi.mock('@tauri-apps/api/core', () => ({invoke: invokeMock}));

import {getWindowsStartupStatus, setWindowsStartupEnabled} from './desktop-bridge';

afterEach(() => {
  invokeMock.mockReset();
});

describe('Windows startup desktop bridge', () => {
  it('reads whether the current executable is registered for the current user', async () => {
    const status = {supported: true, enabled: false};
    invokeMock.mockResolvedValueOnce(status);

    await expect(getWindowsStartupStatus()).resolves.toEqual(status);
    expect(invokeMock).toHaveBeenCalledWith('windows_startup_status');
  });

  it("sends the user's explicit login-startup choice to Tauri", async () => {
    const status = {supported: true, enabled: true};
    invokeMock.mockResolvedValueOnce(status);

    await expect(setWindowsStartupEnabled(true)).resolves.toEqual(status);
    expect(invokeMock).toHaveBeenCalledWith('set_windows_startup_enabled', {enabled: true});
  });
});
