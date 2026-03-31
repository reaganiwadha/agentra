<script lang="ts">
	import Modal from '$lib/components/Modal.svelte';
	import { saveProvider, testProviderGet, type ProviderModalMode } from '$lib/model-admin';

	type Props = {
		open?: boolean;
		mode?: ProviderModalMode;
		provider?: any;
		providerTypes?: any[];
		onClose?: () => void;
		onSaved?: (provider: any) => void;
	};

	let {
		open = false,
		mode = 'create',
		provider = null,
		providerTypes = [],
		onClose = () => {},
		onSaved = () => {}
	}: Props = $props();

	let submitting = $state(false);
	let error = $state('');
	let providerId = $state('');
	let providerName = $state('');
	let providerType = $state('openai_compat');
	let providerBaseUrl = $state('');
	let providerApiKey = $state('');
	let providerIsActive = $state(true);
	let selectedBasePreset = $state('custom');
	let testPath = $state('/models');
	let testing = $state(false);
	let testResult = $state<any>(null);
	let wasOpen = $state(false);

	const providerTypesMeta = $derived(Object.fromEntries(providerTypes.map((t: any) => [t.type, t])));
	const selectedProviderMeta = $derived(providerTypesMeta[providerType] ?? null);
	const baseUrlPresets = [
		{
			id: 'openai',
			label: 'OpenAI',
			providerType: 'openai_compat',
			baseUrl: 'https://api.openai.com/v1',
			iconUrl: 'https://cdn.simpleicons.org/openai/111827'
		},
		{
			id: 'openrouter',
			label: 'OpenRouter',
			providerType: 'openai_compat',
			baseUrl: 'https://openrouter.ai/api/v1',
			iconUrl: 'https://cdn.simpleicons.org/openrouter/111827'
		},
		{
			id: 'ollama',
			label: 'Ollama',
			providerType: 'openai_compat',
			baseUrl: 'http://localhost:11434/v1',
			iconUrl: 'https://cdn.simpleicons.org/ollama/111827'
		},
		{
			id: 'deepgram',
			label: 'Deepgram',
			providerType: 'deepgram',
			baseUrl: 'https://api.deepgram.com',
			iconUrl: 'https://cdn.simpleicons.org/deepgram/111827'
		}
	];
	const availableBasePresets = $derived(
		baseUrlPresets.filter((p) => p.providerType === providerType || p.id === 'custom')
	);
	const selectedBasePresetMeta = $derived(
		baseUrlPresets.find((p) => p.id === selectedBasePreset) ?? null
	);

	$effect(() => {
		if (!open) {
			wasOpen = false;
			return;
		}
		if (wasOpen) return;
		wasOpen = true;

		error = '';
		submitting = false;
		testResult = null;
		testing = false;
		testPath = '/models';

		if (mode === 'edit' && provider) {
			providerId = provider.id;
			providerName = provider.name ?? '';
			providerType = provider.provider_type ?? 'openai_compat';
			providerBaseUrl = provider.base_url ?? '';
			selectedBasePreset = presetForBaseUrl(provider.provider_type ?? 'openai_compat', provider.base_url ?? '');
			providerApiKey = '';
			providerIsActive = Boolean(provider.is_active);
			return;
		}

		providerId = '';
		providerName = '';
		providerType = 'openai_compat';
		providerBaseUrl = providerTypesMeta['openai_compat']?.default_base_url ?? '';
		selectedBasePreset = 'openai';
		providerApiKey = '';
		providerIsActive = true;
	});

	function onProviderTypeChange() {
		const meta = providerTypesMeta[providerType];
		if (mode === 'create' && meta?.default_base_url) {
			providerBaseUrl = meta.default_base_url;
		}
		if (providerType === 'deepgram') {
			selectedBasePreset = 'deepgram';
			providerBaseUrl = 'https://api.deepgram.com';
		}
		if (providerType === 'openai_compat' && selectedBasePreset === 'deepgram') {
			selectedBasePreset = 'openai';
			providerBaseUrl = 'https://api.openai.com/v1';
		}
	}

	async function runTestGet() {
		testing = true;
		testResult = null;
		try {
			testResult = await testProviderGet({
				provider_type: providerType,
				base_url: providerBaseUrl,
				api_key: providerApiKey || undefined,
				test_path: testPath || '/models'
			});
		} catch (err: any) {
			testResult = { ok: false, error: err?.message ?? 'GET test failed' };
		} finally {
			testing = false;
		}
	}

	function onBasePresetChange() {
		const preset = baseUrlPresets.find((p) => p.id === selectedBasePreset);
		if (!preset) return;
		if (preset.id === 'custom') return;
		providerBaseUrl = preset.baseUrl;
	}

	function onBaseUrlInput() {
		selectedBasePreset = presetForBaseUrl(providerType, providerBaseUrl);
	}

	function presetForBaseUrl(type: string, baseUrl: string) {
		const normalized = String(baseUrl ?? '').trim().replace(/\/+$/, '');
		const hit = baseUrlPresets.find(
			(p) => p.providerType === type && p.baseUrl.replace(/\/+$/, '') === normalized
		);
		return hit?.id ?? 'custom';
	}

	async function submit() {
		submitting = true;
		error = '';
		try {
			const saved = await saveProvider(mode, providerId, {
				name: providerName,
				provider_type: providerType,
				base_url: providerBaseUrl,
				api_key: providerApiKey || undefined,
				is_active: providerIsActive
			});
			onSaved(saved);
			onClose();
		} catch (err: any) {
			error = err?.message ?? 'Failed to save provider';
		} finally {
			submitting = false;
		}
	}
</script>

<Modal
	open={open}
	title={mode === 'create' ? 'Add Model Provider' : 'Edit Model Provider'}
	description="Endpoint and credentials used by analyzers."
	widthClass="max-w-xl"
	onClose={onClose}
>
	<div class="space-y-4">
		{#if error}
			<div class="p-3 rounded bg-red-50 text-red-700 text-sm">{error}</div>
		{/if}
		<div>
			<label class="block text-xs font-medium mb-1">Provider Type</label>
			<select bind:value={providerType} class="input-base" onchange={onProviderTypeChange}>
				{#each providerTypes as t}
					<option value={t.type}>{t.label}</option>
				{/each}
			</select>
		</div>
		<div>
			<label class="block text-xs font-medium mb-1">Name</label>
			<input type="text" required bind:value={providerName} class="input-base" />
		</div>
		{#if !selectedProviderMeta?.base_url_fixed}
			<div>
				<label class="block text-xs font-medium mb-1">Base URL Preset</label>
				<div class="flex items-center gap-2">
					<select bind:value={selectedBasePreset} class="input-base" onchange={onBasePresetChange}>
						{#if providerType === 'openai_compat'}
							<option value="openai">OpenAI</option>
							<option value="openrouter">OpenRouter</option>
							<option value="ollama">Ollama</option>
						{/if}
						{#if providerType === 'deepgram'}
							<option value="deepgram">Deepgram</option>
						{/if}
						<option value="custom">Custom</option>
					</select>
					{#if selectedBasePresetMeta?.iconUrl && selectedBasePreset !== 'custom'}
						<img
							src={selectedBasePresetMeta.iconUrl}
							alt={selectedBasePresetMeta.label}
							class="h-7 w-7 rounded border border-slate-200 bg-white p-1"
						/>
					{/if}
				</div>
			</div>
			<div>
				<label class="block text-xs font-medium mb-1">Base URL</label>
				<input
					type="text"
					required
					bind:value={providerBaseUrl}
					oninput={onBaseUrlInput}
					placeholder="http://localhost:11434/v1"
					class="input-base"
				/>
			</div>
		{:else}
			<div class="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-500">
				Base URL is managed by Agentra for this provider type.
			</div>
		{/if}
		<div>
			<label class="block text-xs font-medium mb-1">
				API Key
				{#if selectedProviderMeta?.api_key_required}<span class="text-rose-500">*</span>{/if}
			</label>
			<input
				type="password"
				bind:value={providerApiKey}
				required={selectedProviderMeta?.api_key_required && mode === 'create'}
				placeholder={mode === 'edit' ? 'Leave empty to keep existing key' : selectedProviderMeta?.api_key_required ? 'Required' : 'Optional'}
				class="input-base"
			/>
		</div>
		<div class="rounded-lg border border-slate-200 bg-slate-50 p-3 space-y-2">
			<div class="text-xs font-medium text-slate-700">GET / cURL test</div>
			<div class="flex gap-2">
				<input type="text" bind:value={testPath} class="input-base" placeholder="/models" />
				<button type="button" class="btn-secondary text-sm whitespace-nowrap" onclick={runTestGet} disabled={testing || !providerBaseUrl}>
					{testing ? 'Testing...' : 'Run GET test'}
				</button>
			</div>
			{#if testResult}
				<div class={`text-xs rounded p-2 ${testResult.ok ? 'bg-emerald-50 text-emerald-700' : 'bg-red-50 text-red-700'}`}>
					{#if testResult.status}
						<div>Status: {testResult.status}</div>
					{/if}
					{#if testResult.error}
						<div>Error: {testResult.error}</div>
					{/if}
					{#if testResult.curl}
						<div class="mt-1 text-slate-700">
							<div class="font-medium">cURL</div>
							<code class="block mt-1 whitespace-pre-wrap break-all">{testResult.curl}</code>
						</div>
					{/if}
				</div>
			{/if}
		</div>
		<label class="text-sm flex items-center gap-2"><input type="checkbox" bind:checked={providerIsActive} /> Active</label>
		<div class="flex justify-end gap-2">
			<button type="button" class="btn-secondary text-sm" onclick={onClose}>Cancel</button>
			<button type="button" class="btn-primary text-sm disabled:opacity-60" disabled={submitting} onclick={submit}>
				{submitting ? 'Saving...' : mode === 'create' ? 'Create Provider' : 'Save Provider'}
			</button>
		</div>
	</div>
</Modal>
