<script lang="ts">
	import { api } from '$lib/api';
	import { onMount } from 'svelte';

	let config = $state<any>(null);
	let providers = $state<any[]>([]);
	let saving = $state(false);
	let error = $state('');
	let success = $state(false);

	onMount(async () => {
		const [cfgRes, providersRes] = await Promise.all([
			api.get('/editor-config'),
			api.get('/admin/providers')
		]);
		if (cfgRes.ok) {
			config = await cfgRes.json();
		} else {
			config = {
				provider_id: '',
				model_name: '',
				base_prompt: '',
				max_duration_sec: 300,
				is_autonomous_enabled: false
			};
		}
		if (providersRes.ok) providers = (await providersRes.json()).filter((p: any) => p.provider_type !== 'deepgram');
		if (!config.provider_id && providers.length > 0) config.provider_id = providers[0].id;
	});

	async function handleSave(e: SubmitEvent) {
		e.preventDefault();
		saving = true;
		error = '';
		success = false;
		const fd = new FormData(e.target as HTMLFormElement);
		const res = await api.put('/editor-config', {
			provider_id: fd.get('provider_id'),
			model_name: fd.get('model_name'),
			base_prompt: fd.get('base_prompt'),
			max_duration_sec: Number(fd.get('max_duration_sec')),
			is_autonomous_enabled: fd.get('is_autonomous_enabled') === 'true'
		});
		saving = false;
		if (!res.ok) {
			const b = await res.json().catch(() => ({}));
			error = b.error ?? 'Failed to save editor config';
			return;
		}
		config = await res.json();
		success = true;
	}
</script>

<svelte:head><title>Editor config - Admin - Agentra</title></svelte:head>

<div class="max-w-4xl mx-auto space-y-6">
	<div>
		<h1 class="text-3xl font-semibold">Editor configuration</h1>
		<p class="text-sm text-slate-500 mt-1">Set the base prompt and runtime limits for ragEDITOR.</p>
	</div>

	{#if error}
		<div class="p-3 rounded bg-red-50 text-red-700 text-sm">{error}</div>
	{/if}

	{#if success}
		<div class="p-3 rounded bg-emerald-50 text-emerald-700 text-sm">Editor configuration saved.</div>
	{/if}

	<div class="surface-card p-6">
		<form onsubmit={handleSave} class="space-y-4">
			<div>
				<label class="block text-sm font-medium mb-1">Provider</label>
				<select name="provider_id" class="input-base" required>
					{#each providers as p}
						<option value={p.id} selected={p.id === config?.provider_id}>{p.name} ({p.provider_type})</option>
					{/each}
				</select>
			</div>
			<div>
				<label class="block text-sm font-medium mb-1">Model name</label>
				<input name="model_name" type="text" value={config?.model_name ?? ''} class="input-base" required />
			</div>
			<div>
				<label class="block text-sm font-medium mb-1">Base prompt</label>
				<textarea name="base_prompt" rows="6" class="input-base font-mono resize-y">{config?.base_prompt ?? ''}</textarea>
			</div>
			<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
				<div>
					<label class="block text-sm font-medium mb-1">Max duration (seconds)</label>
					<input name="max_duration_sec" type="number" min="1" value={config?.max_duration_sec ?? 30} class="input-base" />
				</div>
				<div>
					<label class="block text-sm font-medium mb-1">Autonomous mode</label>
					<select name="is_autonomous_enabled" class="input-base">
						<option value="false" selected={!config?.is_autonomous_enabled}>Off</option>
						<option value="true" selected={config?.is_autonomous_enabled}>On</option>
					</select>
				</div>
			</div>
			<button type="submit" class="btn-primary text-sm" disabled={saving}>{saving ? 'Saving...' : 'Save'}</button>
		</form>
	</div>
</div>
