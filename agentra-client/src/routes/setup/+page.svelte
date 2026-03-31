<script lang="ts">
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { auth, init, setSession } from '$lib/auth.svelte';
	import ProviderModal from '$lib/components/ProviderModal.svelte';
	import { fetchModelAdminData, upsertAnalyzerByType, type ProviderModalMode } from '$lib/model-admin';
	import { getProviderRule, providerSupportsAnalyzer } from '$lib/model-setup';
	import { onMount } from 'svelte';

	type StorageOption = {
		type: string;
		label: string;
		description: string;
		default_endpoint: string;
		default_access_key: string;
		default_secret_key: string;
		default_bucket: string;
		default_base_path: string;
		default_output_base_path: string;
	};

	let availableStorages = $state<StorageOption[]>([]);
	let recommendedStorage = $state('');
	let step = $state(1);
	let message = $state('');
	let messageOk = $state(false);
	let loading = $state(false);
	let setupProvisioned = $state(false);

	// Step 1 fields
	let setupToken = $state('');
	let orgName = $state('');
	let email = $state('');
	let password = $state('');
	let confirmPassword = $state('');

	// Step 2 fields
	let storageType = $state('minio');
	let endpoint = $state('');
	let accessKey = $state('');
	let secretKey = $state('');
	let bucket = $state('');
	let basePath = $state('');
	let outputBasePath = $state('');

	// Step 3 fields
	let providers = $state<any[]>([]);
	let analyzers = $state<any[]>([]);
	let providerTypes = $state<any[]>([]);
	let visionProviderId = $state('');
	let visionModelName = $state('');
	let embeddingProviderId = $state('');
	let embeddingModelName = $state('');
	let modelSaving = $state(false);
	let modelError = $state('');
	let providerModalOpen = $state(false);
	let providerModalMode = $state<ProviderModalMode>('create');

	const defaultStorage = $derived(
		availableStorages.find((s) => s.type === recommendedStorage) ?? availableStorages[0] ?? null
	);

	const providerTypesMeta = $derived(
		Object.fromEntries(providerTypes.map((t: any) => [t.type, t]))
	);

	const visionProviderOptions = $derived(
		providers.filter((p: any) => providerSupportsAnalyzer(p.provider_type, 'vision_tags'))
	);

	const embeddingProviderOptions = $derived(
		providers.filter((p: any) => providerSupportsAnalyzer(p.provider_type, 'embedding'))
	);
	const canFinishStep3 = $derived(() => {
		if (providers.length === 0) return false;
		if (!visionProviderId || !embeddingProviderId) return false;
		if (isModelRequiredForProvider(visionProviderId) && !visionModelName.trim()) return false;
		if (isModelRequiredForProvider(embeddingProviderId) && !embeddingModelName.trim()) return false;
		return true;
	});

	onMount(async () => {
		init();
		const res = await fetch(api.BASE + '/setup/options').catch(() => null);
		if (!res?.ok) {
			goto(auth.token ? '/dashboard' : '/login');
			return;
		}
		const data = await res.json();
		if (!data.needs_setup) {
			goto(auth.token ? '/dashboard' : '/login');
			return;
		}
		availableStorages = data.available_storages ?? [];
		recommendedStorage = data.recommended_storage ?? '';

		const def = availableStorages.find((s) => s.type === recommendedStorage) ?? availableStorages[0];
		if (def) {
			storageType = def.type;
			endpoint = def.default_endpoint ?? '';
			accessKey = def.default_access_key ?? '';
			secretKey = def.default_secret_key ?? '';
			bucket = def.default_bucket ?? '';
			basePath = def.default_base_path ?? '';
			outputBasePath = def.default_output_base_path ?? '';
		}
	});

	async function validateIdentity() {
		if (password !== confirmPassword) {
			message = 'Passwords do not match.';
			messageOk = false;
			return;
		}
		loading = true;
		message = '';
		const res = await api.post('/setup/validate', {
			token: setupToken,
			organization_name: orgName,
			email,
			password
		});
		loading = false;
		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			message = body.error ?? 'Setup dry run failed';
			messageOk = false;
			return;
		}
		message = 'Identity is valid.';
		messageOk = true;
		step = 2;
	}

	async function validateStorage() {
		loading = true;
		message = '';
		const res = await api.post('/setup/storage/validate', storagePayload());
		const body = await res.json().catch(() => ({}));
		loading = false;
		if (!res.ok) {
			message = body.error ?? 'Storage dry run failed';
			messageOk = false;
			return;
		}
		messageOk = body.healthy === true;
		message =
			body.healthy === true
				? 'Storage dry run passed.'
				: (body.message ?? 'Storage dry run failed');
	}

	async function continueToModelSetup() {
		if (password !== confirmPassword) {
			message = 'Passwords do not match.';
			messageOk = false;
			step = 1;
			return;
		}
		loading = true;
		message = '';

		if (!setupProvisioned) {
			const identityRes = await api.post('/setup/validate', {
				token: setupToken,
				organization_name: orgName,
				email,
				password
			});
			if (!identityRes.ok) {
				const body = await identityRes.json().catch(() => ({}));
				message = body.error ?? 'Identity is no longer valid';
				messageOk = false;
				step = 1;
				loading = false;
				return;
			}

			const res = await api.post('/setup', {
				token: setupToken,
				organization_name: orgName,
				email,
				password,
				...storageConfigPayload()
			});
			if (!res.ok) {
				const body = await res.json().catch(() => ({}));
				message = body.error ?? 'Setup failed';
				messageOk = false;
				loading = false;
				return;
			}

			const loginRes = await api.post('/login', { email, password });
			if (!loginRes.ok) {
				const body = await loginRes.json().catch(() => ({}));
				message = body.error ?? 'Setup finished, but automatic sign-in failed';
				messageOk = false;
				loading = false;
				return;
			}

			const loginData = await loginRes.json();
			const meRes = await fetch(api.BASE + '/me', {
				headers: { Authorization: `Bearer ${loginData.token}` }
			});
			const me = meRes.ok ? await meRes.json() : { email, role: 'admin' };
			setSession(loginData.token, me);
			setupProvisioned = true;
		}

		const loaded = await loadModelSetupData();
		if (!loaded) {
			loading = false;
			return;
		}

		message = 'Core setup complete. Configure vision + embedding before finishing.';
		messageOk = true;
		step = 3;
		loading = false;
	}

	async function loadModelSetupData() {
		modelError = '';
		try {
			const data = await fetchModelAdminData();
			providers = data.providers;
			analyzers = data.analyzers;
			providerTypes = data.providerTypes;
			seedModelSelections();
			return true;
		} catch (err: any) {
			modelError = 'Failed to load model configuration options.';
			return false;
		}
	}

	function seedModelSelections() {
		const vision = analyzers.find((a: any) => a.analyzer_type === 'vision_tags');
		const embedding = analyzers.find((a: any) => a.analyzer_type === 'embedding');
		visionProviderId = vision?.provider_id ?? visionProviderOptions[0]?.id ?? '';
		visionModelName = vision?.model_name ?? '';
		embeddingProviderId = embedding?.provider_id ?? embeddingProviderOptions[0]?.id ?? '';
		embeddingModelName = embedding?.model_name ?? '';
	}

	function storagePayload() {
		return {
			token: setupToken,
			organization_name: orgName,
			email,
			password,
			storage_type: storageType,
			endpoint,
			access_key: accessKey,
			secret_key: secretKey,
			bucket,
			base_path: basePath,
			output_base_path: outputBasePath
		};
	}

	function storageConfigPayload() {
		return {
			storage_type: storageType,
			endpoint,
			access_key: accessKey,
			secret_key: secretKey,
			bucket,
			base_path: basePath,
			output_base_path: outputBasePath
		};
	}

	function providerLabel(provider: any) {
		const typeLabel = providerTypesMeta[provider.provider_type]?.label ?? provider.provider_type;
		return `${provider.name} (${typeLabel})`;
	}

	function analyzerLabel(analyzerType: 'vision_tags' | 'embedding') {
		return analyzerType === 'vision_tags' ? 'Vision Tags' : 'Embedding';
	}

	function analyzerName(providerId: string, analyzerType: 'vision_tags' | 'embedding') {
		const provider = providers.find((p: any) => p.id === providerId);
		const providerName = provider?.name ? String(provider.name).trim() : 'Unnamed Provider';
		return `${providerName} ${analyzerLabel(analyzerType)} Analyzer`;
	}

	function isModelRequiredForProvider(providerId: string) {
		const provider = providers.find((p: any) => p.id === providerId);
		if (!provider) return true;
		return getProviderRule(provider.provider_type).modelMode === 'required';
	}

	async function upsertAnalyzer(analyzerType: 'vision_tags' | 'embedding', providerId: string, modelName: string) {
		await upsertAnalyzerByType(
			analyzers,
			analyzerType,
			providerId,
			modelName,
			analyzerName(providerId, analyzerType)
		);
	}

	function openCreateProvider() {
		providerModalMode = 'create';
		providerModalOpen = true;
	}

	function onProviderSaved(saved: any) {
		const exists = providers.some((p: any) => p.id === saved.id);
		if (exists) {
			providers = providers.map((p: any) => (p.id === saved.id ? saved : p));
		} else {
			providers = [...providers, saved];
		}
		if (!visionProviderId && providerSupportsAnalyzer(saved.provider_type, 'vision_tags')) {
			visionProviderId = saved.id;
		}
		if (!embeddingProviderId && providerSupportsAnalyzer(saved.provider_type, 'embedding')) {
			embeddingProviderId = saved.id;
		}
	}

	async function finishSetup() {
		modelSaving = true;
		modelError = '';

		if (providers.length === 0) {
			modelSaving = false;
			goto('/dashboard');
			return;
		}
		if (!visionProviderId || !embeddingProviderId) {
			modelError = 'Choose providers for both Vision and Embedding.';
			modelSaving = false;
			return;
		}
		if (isModelRequiredForProvider(visionProviderId) && !visionModelName.trim()) {
			modelError = 'Vision model name is required for the selected provider.';
			modelSaving = false;
			return;
		}
		if (isModelRequiredForProvider(embeddingProviderId) && !embeddingModelName.trim()) {
			modelError = 'Embedding model name is required for the selected provider.';
			modelSaving = false;
			return;
		}

		try {
			await upsertAnalyzer('vision_tags', visionProviderId, visionModelName.trim());
			await upsertAnalyzer('embedding', embeddingProviderId, embeddingModelName.trim());
			goto('/dashboard');
		} catch (err: any) {
			modelError = err?.message ?? 'Failed to save model setup.';
		} finally {
			modelSaving = false;
		}
	}
</script>

<svelte:head><title>Setup</title></svelte:head>

<div class="min-h-screen flex items-center justify-center px-6 py-12">
	<div class="surface-card w-full max-w-xl p-8">
		<div class="mb-6">
			<h1 class="text-2xl font-semibold">Setup</h1>
			<p class="text-sm text-slate-500 mt-1">
				Configure identity, storage, and model analyzers.
			</p>
		</div>

		<div class="mb-6">
			<div class="flex items-center gap-2" aria-label="Setup progress">
				<div class={`h-2 flex-1 rounded-full ${step >= 1 ? 'bg-slate-900' : 'bg-slate-200'}`}></div>
				<div class={`h-2 flex-1 rounded-full ${step >= 2 ? 'bg-slate-900' : 'bg-slate-200'}`}></div>
				<div class={`h-2 flex-1 rounded-full ${step >= 3 ? 'bg-slate-900' : 'bg-slate-200'}`}></div>
			</div>
			<div class="mt-2 text-xs uppercase tracking-[0.12em] text-slate-500">Step {step} of 3</div>
		</div>

		{#if message}
			<div class={`mb-4 p-3 rounded text-sm ${messageOk ? 'bg-sky-50 text-sky-700' : 'bg-red-50 text-red-700'}`}>
				{message}
			</div>
		{/if}

		{#if step === 1}
			<div class="space-y-4">
				<div>
					<label class="block text-sm font-medium mb-1" for="token">Setup token</label>
					<input id="token" type="text" required bind:value={setupToken} class="input-base" />
				</div>
				<div>
					<label class="block text-sm font-medium mb-1" for="organization_name">Organization name</label>
					<input id="organization_name" type="text" required bind:value={orgName} class="input-base" />
				</div>
				<div>
					<label class="block text-sm font-medium mb-1" for="email">Admin email</label>
					<input id="email" type="email" required bind:value={email} class="input-base" />
				</div>
				<div>
					<label class="block text-sm font-medium mb-1" for="password">Password</label>
					<input id="password" type="password" required minlength="8" bind:value={password} class="input-base" />
				</div>
				<div>
					<label class="block text-sm font-medium mb-1" for="confirm_password">Confirm password</label>
					<input
						id="confirm_password"
						type="password"
						required
						minlength="8"
						bind:value={confirmPassword}
						class="input-base"
					/>
				</div>
				<button type="button" class="w-full btn-primary" onclick={validateIdentity} disabled={loading}>
					{loading ? 'Validating...' : 'Continue'}
				</button>
			</div>
		{/if}

		{#if step === 2}
			<div class="space-y-4">
				{#if defaultStorage}
					<div class="surface-soft p-4 text-sm text-slate-700">
						<div class="font-semibold text-slate-900">{defaultStorage.label}</div>
					</div>
				{/if}

				<div>
					<label class="block text-sm font-medium mb-1" for="storage_type">Storage type</label>
					<select id="storage_type" bind:value={storageType} class="input-base">
						{#each availableStorages as s}
							<option value={s.type}>{s.label}</option>
						{/each}
					</select>
				</div>
				<div>
					<label class="block text-sm font-medium mb-1" for="endpoint">Endpoint</label>
					<input id="endpoint" type="text" required bind:value={endpoint} class="input-base" />
				</div>
				<div class="grid sm:grid-cols-2 gap-3">
					<div>
						<label class="block text-sm font-medium mb-1" for="access_key">Access key</label>
						<input id="access_key" type="text" required bind:value={accessKey} class="input-base" />
					</div>
					<div>
						<label class="block text-sm font-medium mb-1" for="secret_key">Secret key</label>
						<input id="secret_key" type="password" required bind:value={secretKey} class="input-base" />
					</div>
				</div>
				<div>
					<label class="block text-sm font-medium mb-1" for="bucket">Bucket</label>
					<input id="bucket" type="text" required bind:value={bucket} class="input-base" />
				</div>
				<div class="grid sm:grid-cols-2 gap-3">
					<div>
						<label class="block text-sm font-medium mb-1" for="base_path">Media base path</label>
						<input id="base_path" type="text" bind:value={basePath} class="input-base" />
					</div>
					<div>
						<label class="block text-sm font-medium mb-1" for="output_base_path">Output base path</label>
						<input id="output_base_path" type="text" bind:value={outputBasePath} class="input-base" />
					</div>
				</div>
				<div class="flex gap-3">
					<button type="button" class="w-1/2 btn-secondary" onclick={validateStorage} disabled={loading}>
						{loading ? 'Checking...' : 'Run storage dry run'}
					</button>
					<button type="button" class="w-1/2 btn-primary" onclick={continueToModelSetup} disabled={loading}>
						{loading ? 'Continuing...' : 'Continue'}
					</button>
				</div>
			</div>
		{/if}

		{#if step === 3}
			<div class="space-y-4">
				{#if modelError}
					<div class="p-3 rounded bg-red-50 text-red-700 text-sm">{modelError}</div>
				{/if}

				<div class="flex justify-end">
					<button type="button" class="btn-secondary text-sm" onclick={openCreateProvider}>
						+ Add Provider
					</button>
				</div>

				{#if providers.length === 0}
					<div class="p-3 rounded bg-amber-50 text-amber-700 text-sm">
						No providers found yet. Add one to continue.
					</div>
				{:else}
					<div class="surface-soft p-4 space-y-3">
						<div class="font-semibold text-slate-900">Vision model</div>
						<div>
							<label class="block text-sm font-medium mb-1" for="vision_provider">Provider</label>
							<select id="vision_provider" class="input-base" bind:value={visionProviderId}>
								{#each visionProviderOptions as p}
									<option value={p.id}>{providerLabel(p)}</option>
								{/each}
							</select>
						</div>
						<div>
							<label class="block text-sm font-medium mb-1" for="vision_model">Model name</label>
							<input id="vision_model" type="text" class="input-base" bind:value={visionModelName} placeholder="gpt-4.1-mini" />
						</div>
					</div>

					<div class="surface-soft p-4 space-y-3">
						<div class="font-semibold text-slate-900">Embedding model</div>
						<div>
							<label class="block text-sm font-medium mb-1" for="embedding_provider">Provider</label>
							<select id="embedding_provider" class="input-base" bind:value={embeddingProviderId}>
								{#each embeddingProviderOptions as p}
									<option value={p.id}>{providerLabel(p)}</option>
								{/each}
							</select>
						</div>
						<div>
							<label class="block text-sm font-medium mb-1" for="embedding_model">Model name</label>
							<input id="embedding_model" type="text" class="input-base" bind:value={embeddingModelName} placeholder="text-embedding-3-small" />
						</div>
					</div>

					<button
						type="button"
						class="w-full btn-primary disabled:opacity-50 disabled:cursor-not-allowed"
						onclick={finishSetup}
						disabled={modelSaving || !canFinishStep3}
					>
						{modelSaving ? 'Saving...' : 'Save and finish'}
					</button>
				{/if}
			</div>
		{/if}

		<div class="mt-6 text-sm text-slate-500">
			Already set up? <a href="/login" class="font-semibold text-slate-900 hover:underline">Sign in</a>.
		</div>
	</div>
</div>

<ProviderModal
	open={providerModalOpen}
	mode={providerModalMode}
	provider={null}
	providerTypes={providerTypes}
	onClose={() => (providerModalOpen = false)}
	onSaved={onProviderSaved}
/>
