<script lang="ts">
	import { api } from '$lib/api';

	let error = $state('');
	let pending = $state(false);
	let waitingForBackend = $state(false);

	async function handleReset() {
		const ok = confirm('Running the first setup wizard again may lead to unintended behavior. Continue?');
		if (!ok) return;
		pending = true;
		waitingForBackend = false;
		error = '';
		const res = await api.post('/setup/reset');
		if (!res.ok) {
			pending = false;
			const b = await res.json().catch(() => ({}));
			error = b.error ?? 'Failed to reset setup';
			return;
		}
		waitingForBackend = true;
		await waitForBackendAndRedirect();
	}

	async function waitForBackendAndRedirect() {
		const maxAttempts = 90;
		for (let i = 0; i < maxAttempts; i += 1) {
			await new Promise((resolve) => setTimeout(resolve, 1000));
			const alive = await fetch(`${api.BASE}/setup`, { cache: 'no-store' })
				.then((r) => r.ok)
				.catch(() => false);
			if (alive) {
				window.location.href = '/setup';
				return;
			}
		}
		pending = false;
		waitingForBackend = false;
		error = 'Backend did not come back online in time. Try again in a few seconds.';
	}
</script>

<svelte:head><title>Misc - Admin - Agentra</title></svelte:head>

<div class="max-w-4xl mx-auto space-y-6">
	<div>
		<h1 class="text-3xl font-semibold">Misc</h1>
		<p class="text-sm text-slate-500 mt-1">Administrative safety actions and system utilities.</p>
	</div>

	<div class="surface-card p-6">
		<h2 class="font-semibold">First setup wizard</h2>
		<p class="text-sm text-slate-500 mt-2">
			Use this only if you need to re-run the initial admin setup flow.
		</p>
		{#if error}
			<div class="mt-4 p-3 rounded bg-red-50 text-red-700 text-sm">{error}</div>
		{/if}
		<div class="mt-4">
			<button type="button" class="btn-secondary text-sm" onclick={handleReset} disabled={pending}>
				{pending ? (waitingForBackend ? 'Waiting for backend...' : 'Requesting...') : 'Request setup reset'}
			</button>
		</div>
		{#if waitingForBackend}
			<div class="mt-3 text-sm text-slate-500">
				Reset requested. Waiting for backend restart...
			</div>
		{/if}
	</div>
</div>
