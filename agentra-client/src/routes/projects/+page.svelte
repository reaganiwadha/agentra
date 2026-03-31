<script lang="ts">
	import { api } from '$lib/api';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { onMount } from 'svelte';

	let projects = $state<any[]>([]);
	let showForm = $state(false);
	let error = $state('');
	let submitting = $state(false);

	onMount(async () => {
		const res = await api.get('/projects');
		if (res.ok) projects = await res.json();
	});

	async function handleCreate(e: SubmitEvent) {
		e.preventDefault();
		submitting = true;
		error = '';
		const fd = new FormData(e.target as HTMLFormElement);
		const res = await api.post('/projects', {
			name: fd.get('name'),
			description: fd.get('description') || undefined
		});
		submitting = false;
		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			error = body.error ?? 'Failed to create project';
			return;
		}
		const project = await res.json();
		projects = [...projects, project];
		showForm = false;
		(e.target as HTMLFormElement).reset();
	}
</script>

<svelte:head><title>Projects - Agentra</title></svelte:head>

<div class="max-w-6xl mx-auto space-y-6">
	<div class="flex flex-wrap items-end justify-between gap-4">
		<div>
			<h1 class="text-3xl font-semibold">Projects</h1>
			<p class="text-sm text-slate-500 mt-1">Organize footage by domain and start analysis.</p>
		</div>
		<button onclick={() => (showForm = !showForm)} class="btn-primary text-sm">
			{showForm ? 'Close' : 'New project'}
		</button>
	</div>

	{#if showForm}
		<div class="surface-card p-6">
			<div class="flex items-start justify-between">
				<div>
					<h2 class="font-semibold">Create project</h2>
					<p class="text-sm text-slate-500 mt-1">Give the project a name and optional description.</p>
				</div>
				<button type="button" onclick={() => (showForm = false)} class="btn-secondary text-sm">Cancel</button>
			</div>
			{#if error}
				<div class="mt-4 p-3 rounded bg-red-50 text-red-700 text-sm">{error}</div>
			{/if}
			<form onsubmit={handleCreate} class="space-y-4 mt-4">
				<input name="name" type="text" placeholder="Project name" required class="input-base" />
				<input name="description" type="text" placeholder="Description (optional)" class="input-base" />
				<div class="flex gap-3">
					<button type="submit" class="btn-primary text-sm" disabled={submitting}>
						{submitting ? 'Creating...' : 'Create project'}
					</button>
					<button type="button" onclick={() => (showForm = false)} class="btn-secondary text-sm">Cancel</button>
				</div>
			</form>
		</div>
	{/if}

	<div class="surface-card">
		<div class="grid grid-cols-1">
			{#each projects as project}
				<a href="/projects/{project.id}" class="flex items-center gap-4 px-6 py-4 border-b border-slate-100 last:border-b-0 hover:bg-slate-50">
					<div>
						<div class="font-semibold text-sm">{project.name}</div>
						{#if project.description}
							<div class="text-xs text-slate-500 mt-1">{project.description}</div>
						{/if}
					</div>
					<div class="ml-auto">
						<StatusBadge status={project.status} />
					</div>
				</a>
			{:else}
				<div class="px-6 py-10 text-center text-slate-400 text-sm">No projects yet. Create one to get started.</div>
			{/each}
		</div>
	</div>
</div>
