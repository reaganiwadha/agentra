import { PUBLIC_API_URL } from '$env/static/public';

const BASE = PUBLIC_API_URL || 'http://localhost:8080';

function token(): string {
	return (typeof localStorage !== 'undefined' && localStorage.getItem('agentra_session')) || '';
}

function headers(json = true): Record<string, string> {
	const h: Record<string, string> = {};
	const t = token();
	if (t) h['Authorization'] = `Bearer ${t}`;
	if (json) h['Content-Type'] = 'application/json';
	return h;
}

export const api = {
	BASE,
	token,
	get: (path: string) => fetch(BASE + path, { headers: headers(false) }),
	post: (path: string, body?: unknown) =>
		fetch(BASE + path, {
			method: 'POST',
			headers: headers(),
			body: body !== undefined ? JSON.stringify(body) : undefined
		}),
	put: (path: string, body?: unknown) =>
		fetch(BASE + path, {
			method: 'PUT',
			headers: headers(),
			body: body !== undefined ? JSON.stringify(body) : undefined
		}),
	postForm: (path: string, body: FormData) =>
		fetch(BASE + path, {
			method: 'POST',
			headers: headers(false),
			body
		}),
	del: (path: string) => fetch(BASE + path, { method: 'DELETE', headers: headers(false) })
};
