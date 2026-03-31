<script lang="ts">
	import { api } from '$lib/api';
	import { onMount } from 'svelte';

	let storage = $state<any>(null);
	let storageStatus = $state<any>(null);
	let storageType = $state('smb');
	let checkingStatus = $state(false);
	let saving = $state(false);
	let error = $state('');
	let success = $state(false);

	onMount(async () => {
		const [storageRes, statusRes] = await Promise.all([
			api.get('/storage'),
			api.get('/storage/status')
		]);
		if (storageRes.ok) {
			storage = await storageRes.json();
			storageType = storage.storage_type ?? 'smb';
		}
		if (statusRes.ok) storageStatus = await statusRes.json();
	});

	async function checkStatus() {
		checkingStatus = true;
		const res = await api.get('/storage/status');
		if (res.ok) storageStatus = await res.json();
		checkingStatus = false;
	}

	async function handleSave(e: SubmitEvent) {
		e.preventDefault();
		saving = true;
		error = '';
		success = false;
		const fd = new FormData(e.target as HTMLFormElement);
		const body: Record<string, any> = {
			storage_type: fd.get('storage_type'),
			endpoint: fd.get('endpoint'),
			access_key: fd.get('access_key'),
			bucket: fd.get('bucket'),
			base_path: fd.get('base_path'),
			output_base_path: fd.get('output_base_path')
		};
		const secret = fd.get('secret_key') as string;
		if (secret) body.secret_key = secret;
		const res = await api.put('/storage', body);
		saving = false;
		if (!res.ok) {
			const b = await res.json().catch(() => ({}));
			error = b.error ?? 'Failed to save storage config';
			return;
		}
		storage = await res.json();
		storageType = storage.storage_type;
		success = true;
	}
</script>

<svelte:head><title>Storage - Admin - Agentra</title></svelte:head>

<div class="max-w-4xl mx-auto space-y-6">
	<div>
		<h1 class="text-3xl font-semibold">Storage configuration</h1>
		<p class="text-sm text-slate-500 mt-1">Define the primary source and output locations.</p>
	</div>

	<div class="surface-card p-4 border border-amber-200 bg-amber-50 text-amber-700 text-sm">
		Changing storage does not migrate existing media.
	</div>

	{#if error}
		<div class="p-3 rounded bg-red-50 text-red-700 text-sm">{error}</div>
	{/if}

	{#if success}
		<div class="p-3 rounded bg-emerald-50 text-emerald-700 text-sm">Storage configuration saved.</div>
	{/if}

	<div class="surface-card p-6">
		<div class="flex items-center justify-between gap-3">
			<div>
				<h2 class="text-lg font-semibold">Storage connection</h2>
				<p class="text-sm text-slate-500 mt-1">Realtime read/write probe for current storage config.</p>
			</div>
			<button type="button" class="btn-secondary text-sm" onclick={checkStatus} disabled={checkingStatus}>
				{checkingStatus ? 'Checking...' : 'Check now'}
			</button>
		</div>

		<div class="mt-4 surface-soft p-4">
			{#if !storageStatus}
				<div class="text-sm text-slate-600">No storage status available yet.</div>
			{:else}
				<div class="flex items-center gap-2 text-sm">
					<div class={`h-2.5 w-2.5 rounded-full ${storageStatus.probe?.healthy ? 'bg-emerald-500' : 'bg-amber-500'}`}></div>
					<div class="font-medium">{storageStatus.probe?.healthy ? 'Healthy' : 'Needs attention'}</div>
				</div>
				<div class="text-sm text-slate-600 mt-2">{storageStatus.probe?.message ?? 'No probe message'}</div>
				<div class="grid grid-cols-3 gap-3 mt-3 text-xs text-slate-600">
					<div>Configured: <span class="font-semibold text-slate-800">{storageStatus.configured ? 'Yes' : 'No'}</span></div>
					<div>Read: <span class="font-semibold text-slate-800">{storageStatus.probe?.can_read ? 'OK' : 'Fail'}</span></div>
					<div>Write: <span class="font-semibold text-slate-800">{storageStatus.probe?.can_write ? 'OK' : 'Fail'}</span></div>
				</div>
				{#if storageStatus.probe?.checked_at}
					<div class="text-xs text-slate-500 mt-2">Last checked: {new Date(storageStatus.probe.checked_at).toLocaleString()}</div>
				{/if}
			{/if}
		</div>
	</div>

	<div class="surface-card p-6">
		<form onsubmit={handleSave} class="space-y-4">
			<div>
				<label class="block text-sm font-medium mb-1">Storage type</label>
				<select name="storage_type" bind:value={storageType} class="input-base">
					<option value="smb">SMB (NAS)</option>
					<option value="minio">MinIO (S3)</option>
				</select>
			</div>
			<div>
				<label class="block text-sm font-medium mb-1">Endpoint</label>
				<input name="endpoint" type="text" value={storage?.endpoint ?? ''} placeholder={storageType === 'smb' ? '192.168.1.100:445' : 'http://minio:9000'} class="input-base" />
			</div>
			<div>
				<label class="block text-sm font-medium mb-1">{storageType === 'smb' ? 'Username' : 'Access key'}</label>
				<input name="access_key" type="text" value={storage?.access_key ?? ''} class="input-base" />
			</div>
			<div>
				<label class="block text-sm font-medium mb-1">{storageType === 'smb' ? 'Password' : 'Secret key'}</label>
				<input name="secret_key" type="password" placeholder="********" class="input-base" />
			</div>
			<div>
				<label class="block text-sm font-medium mb-1">{storageType === 'smb' ? 'Share name' : 'Bucket'}</label>
				<input name="bucket" type="text" value={storage?.bucket ?? ''} class="input-base" />
			</div>
			<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
				<div>
					<label class="block text-sm font-medium mb-1">Media base path</label>
					<input name="base_path" type="text" value={storage?.base_path ?? ''} class="input-base" />
				</div>
				<div>
					<label class="block text-sm font-medium mb-1">Output base path</label>
					<input name="output_base_path" type="text" value={storage?.output_base_path ?? ''} class="input-base" />
				</div>
			</div>
			<button type="submit" class="btn-primary text-sm" disabled={saving}>{saving ? 'Saving...' : 'Save'}</button>
		</form>
	</div>
</div>
