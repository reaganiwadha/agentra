export const auth = $state<{ token: string | null; user: any }>({ token: null, user: null });

export function init() {
	if (typeof localStorage !== 'undefined') {
		auth.token = localStorage.getItem('agentra_session');
	}
}

export function setSession(t: string, u: any) {
	auth.token = t;
	auth.user = u;
	localStorage.setItem('agentra_session', t);
}

export function setUser(u: any) {
	auth.user = u;
}

export function clear() {
	auth.token = null;
	auth.user = null;
	if (typeof localStorage !== 'undefined') {
		localStorage.removeItem('agentra_session');
	}
}
