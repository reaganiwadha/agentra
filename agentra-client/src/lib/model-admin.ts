import { api } from '$lib/api';
import type { AnalyzerType } from '$lib/model-setup';

export type ProviderModalMode = 'create' | 'edit';

export type ProviderInput = {
	name: string;
	provider_type: string;
	base_url: string;
	api_key?: string;
	is_active: boolean;
};

export async function fetchModelAdminData() {
	const [providersRes, analyzersRes, providerTypesRes] = await Promise.all([
		api.get('/admin/providers'),
		api.get('/admin/analyzers'),
		api.get('/admin/providers/types')
	]);

	if (!providersRes.ok || !analyzersRes.ok) {
		throw new Error('Failed to load model configuration options.');
	}

	return {
		providers: await providersRes.json(),
		analyzers: await analyzersRes.json(),
		providerTypes: providerTypesRes.ok ? await providerTypesRes.json() : []
	};
}

export async function saveProvider(mode: ProviderModalMode, providerId: string, body: ProviderInput) {
	const res =
		mode === 'create'
			? await api.post('/admin/providers', body)
			: await api.put(`/admin/providers/${providerId}`, body);

	if (!res.ok) {
		const data = await res.json().catch(() => ({}));
		throw new Error(data.error ?? 'Failed to save provider');
	}

	return res.json();
}

export async function testProviderGet(body: { provider_type: string; base_url: string; api_key?: string; test_path?: string }) {
	const res = await api.post('/admin/providers/test-get', body);
	const data = await res.json().catch(() => ({}));
	if (!res.ok) {
		throw new Error(data.error ?? 'Failed to run provider GET test');
	}
	return data;
}

export async function upsertAnalyzerByType(
	analyzers: any[],
	analyzerType: Extract<AnalyzerType, 'vision_tags' | 'embedding'>,
	providerId: string,
	modelName: string,
	name: string
) {
	const existing = analyzers.find((a: any) => a.analyzer_type === analyzerType);
	const body = {
		name,
		analyzer_type: analyzerType,
		provider_id: providerId,
		model_name: modelName,
		config_json: '{}',
		is_enabled: true
	};
	const res = existing
		? await api.put(`/admin/analyzers/${existing.id}`, body)
		: await api.post('/admin/analyzers', body);
	if (!res.ok) {
		const data = await res.json().catch(() => ({}));
		throw new Error(data.error ?? `Failed to save ${analyzerType} analyzer`);
	}
	return res.json();
}
