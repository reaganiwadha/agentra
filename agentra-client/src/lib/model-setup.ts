export type ProviderType = 'openai_compat' | 'deepgram' | 'other';
export type AnalyzerType = 'transcription' | 'vision_tags' | 'embedding';
export type ProviderModelMode = 'required' | 'optional' | 'disabled';

export const providerRules: Record<
	ProviderType,
	{ forcedAnalyzerType?: AnalyzerType; modelMode: ProviderModelMode }
> = {
	openai_compat: { modelMode: 'required' },
	deepgram: { forcedAnalyzerType: 'transcription', modelMode: 'optional' },
	other: { modelMode: 'required' }
};

export function getProviderRule(providerType: string | undefined) {
	const key = (providerType ?? 'other') as ProviderType;
	return providerRules[key] ?? providerRules.other;
}

export function providerSupportsAnalyzer(providerType: string | undefined, analyzerType: AnalyzerType) {
	const rule = getProviderRule(providerType);
	if (rule.forcedAnalyzerType) return rule.forcedAnalyzerType === analyzerType;
	return true;
}

export function getAnalyzerModelHint(providerType: string | undefined) {
	const key = (providerType ?? 'other') as ProviderType;
	const rule = getProviderRule(key);
	if (key === 'deepgram') return 'Optional for Deepgram. Empty uses backend default model (nova-3).';
	return rule.modelMode === 'required' ? 'Required by this provider.' : 'Optional for this provider.';
}
