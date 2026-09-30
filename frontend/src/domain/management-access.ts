// pattern: Functional Core

export type ManagementAccessMode = 'local_management' | 'remote_management_unqualified';

export type ManagementAccessPresentation = Readonly<{
  mode: ManagementAccessMode;
  label: string;
}>;

function isLoopbackHost(hostname: string): boolean {
	const normalized = hostname.toLowerCase();
	return normalized === 'localhost' || normalized === '127.0.0.1' || normalized === '::1' || normalized === '[::1]';
}

export function managementAccessForOrigins(pageOrigin: string, apiBaseUrl: string): ManagementAccessPresentation {
	try {
		const page = new URL(pageOrigin);
		const api = new URL(apiBaseUrl, page);
		if (isLoopbackHost(page.hostname) && isLoopbackHost(api.hostname)) {
			return {mode: 'local_management', label: '本机管理'};
		}
	} catch {
		return {mode: 'remote_management_unqualified', label: '远程管理 · 未验证'};
	}
	return {mode: 'remote_management_unqualified', label: '远程管理 · 未验证'};
}
