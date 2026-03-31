<script lang="ts">
	import { api } from '$lib/api';
	import { onMount } from 'svelte';

	let projects = $state<any[]>([]);

	onMount(async () => {
		const res = await api.get('/projects');
		if (res.ok) projects = await res.json();
	});
</script>

<svelte:head><title>Dashboard - Agentra</title></svelte:head>

<div class="max-w-6xl mx-auto space-y-6">
	<div class="flex flex-wrap items-end justify-between gap-4">
		<div>
			<h1 class="text-3xl font-semibold">Dashboard</h1>
			<p class="text-sm text-slate-500 mt-1">Monitor ingestion, analysis, and highlight generation.</p>
		</div>
		<a href="/projects" class="btn-primary text-sm">View projects</a>
	</div>

	<div class="grid grid-cols-1 md:grid-cols-3 gap-4">
		<div class="surface-card p-6">
			<div class="data-label">Projects</div>
			<div class="text-3xl font-semibold mt-2">{projects.length}</div>
			<div class="text-sm text-slate-500 mt-2">Active media domains</div>
		</div>
		<div class="surface-card p-6">
			<div class="data-label">Pipelines</div>
			<div class="text-lg font-semibold mt-2">Analyzer + Editor</div>
			<div class="text-sm text-slate-500 mt-2">Scheduled every 30 seconds</div>
		</div>
		<div class="surface-card p-6">
			<div class="data-label">Next step</div>
			<div class="text-lg font-semibold mt-2">Configure storage</div>
			<div class="text-sm text-slate-500 mt-2">Set SMB or MinIO to begin ingestion.</div>
		</div>
	</div>

	<div class="surface-card">
		<div class="px-6 py-4 border-b border-slate-200 flex items-center justify-between">
			<h2 class="font-semibold">Recent projects</h2>
			<a href="/projects" class="text-sm text-slate-600 hover:text-slate-900">View all</a>
		</div>
		<div class="divide-y divide-slate-100">
			{#each projects as project}
				<a href="/projects/{project.id}" class="flex items-center gap-4 px-6 py-4 hover:bg-slate-50">
					<div>
						<div class="font-semibold text-sm">{project.name}</div>
						{#if project.description}
							<div class="text-xs text-slate-500 mt-1">{project.description}</div>
						{/if}
					</div>
					<span class="ml-auto text-xs uppercase tracking-wide text-slate-400">{project.status}</span>
				</a>
			{:else}
				<div class="px-6 py-10 text-center text-slate-400 text-sm">No projects yet. Create one to start ingesting.</div>
			{/each}
		</div>
	</div>
</div>
