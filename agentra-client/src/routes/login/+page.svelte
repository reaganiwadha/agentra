<script lang="ts">
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { setSession } from '$lib/auth.svelte';

	let email = $state('');
	let password = $state('');
	let error = $state('');
	let loading = $state(false);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		loading = true;
		error = '';

		const res = await api.post('/login', { email, password });
		if (!res.ok) {
			const data = await res.json().catch(() => ({}));
			error = data.error ?? 'Invalid email or password';
			loading = false;
			return;
		}

		const data = await res.json();
		const meRes = await fetch(api.BASE + '/me', {
			headers: { Authorization: `Bearer ${data.token}` }
		});
		const me = meRes.ok ? await meRes.json() : { email, role: 'editor' };
		setSession(data.token, me);
		goto('/dashboard');
	}
</script>

<svelte:head><title>Login</title></svelte:head>

<div class="min-h-screen flex items-center justify-center px-6 py-12">
	<div class="surface-card w-full max-w-md p-8">
		<div class="mb-6">
			<h1 class="text-2xl font-semibold">Sign in</h1>
			<p class="text-sm text-slate-500 mt-1">Enter your credentials to continue.</p>
		</div>

		{#if error}
			<div class="mb-4 p-3 rounded bg-red-50 text-red-700 text-sm">{error}</div>
		{/if}

		<form onsubmit={handleSubmit} class="space-y-4">
			<div>
				<label class="block text-sm font-medium mb-1" for="email">Email</label>
				<input id="email" name="email" type="email" required bind:value={email} class="input-base" />
			</div>
			<div>
				<label class="block text-sm font-medium mb-1" for="password">Password</label>
				<input id="password" name="password" type="password" required bind:value={password} class="input-base" />
			</div>
			<button type="submit" class="w-full btn-primary" disabled={loading}>
				{loading ? 'Signing in...' : 'Sign in'}
			</button>
		</form>

		<div class="mt-6 text-sm text-slate-500">
			Need to initialize the system? <a href="/setup" class="font-semibold text-slate-900 hover:underline">Run setup</a>.
		</div>
	</div>
</div>
