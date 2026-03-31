import { api } from './api';
import { auth } from './auth.svelte';

export type ActivityLog = {
	id: string;
	level: string;
	source: string;
	message: string;
	event_type: string;
	created_at: string;
	subject_type: string;
	subject_id: string | null;
	payload: Record<string, any> | null;
};

export const activity = $state({
	active: false,
	message: 'No recent backend activity',
	updatedAt: '',
	connected: false,
	transport: 'none' as 'stream' | 'poll' | 'none',
	logs: [] as ActivityLog[]
});

let _ctrl: AbortController | null = null;

function normalize(raw: any): ActivityLog | null {
	if (!raw || typeof raw !== 'object') return null;
	return {
		id: String(raw.id ?? ''),
		level: String(raw.level ?? 'info'),
		source: String(raw.source ?? ''),
		message: String(raw.message ?? ''),
		event_type: String(raw.event_type ?? ''),
		created_at: String(raw.created_at ?? ''),
		subject_type: String(raw.subject_type ?? ''),
		subject_id: typeof raw.subject_id === 'string' ? raw.subject_id : null,
		payload: raw.payload && typeof raw.payload === 'object' ? raw.payload : null
	};
}

function replace(logs: any[]) {
	activity.logs = logs.map(normalize).filter(Boolean) as ActivityLog[];
}

function prepend(raw: any) {
	const next = normalize(raw);
	if (!next) return;
	const idx = activity.logs.findIndex((l) => l.id === next.id);
	if (idx >= 0) {
		const arr = [...activity.logs];
		arr[idx] = next;
		activity.logs = arr;
		return;
	}
	activity.logs = [next, ...activity.logs].slice(0, 120);
}

function applyStatus(active: boolean, latest: any) {
	activity.active = active;
	if (latest?.message) activity.message = String(latest.message);
	else if (active) activity.message = 'Backend is actively processing...';
	else activity.message = 'Backend is idle.';
	activity.updatedAt = latest?.created_at ? String(latest.created_at) : '';
}

function parseSSE(block: string) {
	let eventName = 'message';
	const dataLines: string[] = [];
	for (const line of block.split('\n')) {
		if (!line || line.startsWith(':')) continue;
		if (line.startsWith('event:')) {
			eventName = line.slice(6).trim() || 'message';
			continue;
		}
		if (line.startsWith('data:')) dataLines.push(line.slice(5).trimStart());
	}
	if (eventName === 'heartbeat') return;
	const raw = dataLines.join('\n');
	if (!raw) return;
	let payload: any = raw;
	try {
		payload = JSON.parse(raw);
	} catch {}
	if (eventName === 'snapshot') {
		applyStatus(Boolean(payload?.active), payload?.latest ?? null);
		if (Array.isArray(payload?.logs)) replace(payload.logs);
		return;
	}
	if (eventName === 'log') {
		applyStatus(Boolean(payload?.active), payload?.log ?? null);
		prepend(payload?.log ?? null);
	}
}

async function poll() {
	const token = auth.token;
	if (!token) return;
	const res = await fetch(`${api.BASE}/activity/status`, {
		headers: { Authorization: `Bearer ${token}` }
	}).catch(() => null);
	if (!res || !res.ok) {
		activity.connected = false;
		activity.transport = 'none';
		return;
	}
	const body = await res.json().catch(() => null);
	if (!body) return;
	applyStatus(Boolean(body.active), body.latest ?? null);
	activity.connected = true;
	activity.transport = 'poll';
}

export function stop() {
	if (_ctrl) {
		_ctrl.abort();
		_ctrl = null;
	}
	activity.connected = false;
	activity.transport = 'none';
}

export async function start() {
	if (!auth.token) return;
	stop();
	await poll();
	const ctrl = new AbortController();
	_ctrl = ctrl;

	while (!ctrl.signal.aborted) {
		try {
			const res = await fetch(`${api.BASE}/activity/stream`, {
				headers: { Accept: 'text/event-stream', Authorization: `Bearer ${auth.token ?? ''}` },
				signal: ctrl.signal
			});
			if (!res.ok || !res.body) throw new Error(`stream failed (${res.status})`);
			activity.connected = true;
			activity.transport = 'stream';

			const reader = res.body.getReader();
			const dec = new TextDecoder();
			let buf = '';
			while (true) {
				const { done, value } = await reader.read();
				if (done) break;
				buf += dec.decode(value, { stream: true }).replace(/\r/g, '');
				let at = buf.indexOf('\n\n');
				while (at >= 0) {
					const block = buf.slice(0, at).trim();
					buf = buf.slice(at + 2);
					if (block) parseSSE(block);
					at = buf.indexOf('\n\n');
				}
			}
		} catch {
			await poll();
		}
		if (!ctrl.signal.aborted) {
			await new Promise((r) => setTimeout(r, activity.transport === 'poll' ? 5000 : 2000));
		}
	}
}
