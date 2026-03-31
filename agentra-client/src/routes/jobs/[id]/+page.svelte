<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { auth } from '$lib/auth.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import VideoPlayer from '$lib/components/VideoPlayer.svelte';
	import { onMount } from 'svelte';

	const POLL_INTERVAL = 5000;
	type RunJob = {
		id: string;
		project_id: string;
		requested_by: string;
		prompt: string;
		variant_index: number;
		variant_count: number;
		min_duration_sec: number;
		max_duration_sec: number;
		status: 'queued' | 'running' | 'completed' | 'failed' | 'cancelled';
		error_message?: string | null;
		created_at: string;
	};

	type RunTimeline = {
		id: string;
		job_id: string;
		otio_json: unknown;
		created_at: string;
	};

	type RunRender = {
		id: string;
		job_id: string;
		output_path: string;
		duration_sec?: number | null;
		file_size_bytes?: number | null;
		created_at: string;
	};

	type RunTrace = {
		id: string;
		job_id: string;
		phase: string;
		message: string;
		payload: unknown;
		created_at: string;
	};

	type RunDetail = {
		job: RunJob;
		timeline?: RunTimeline | null;
		render?: RunRender | null;
		traces: RunTrace[];
	};

	let run = $state<RunDetail | null>(null);
	let loadError = $state('');
	let timer: ReturnType<typeof setTimeout> | undefined;

	async function fetchRun() {
		const res = await api.get(`/runs/${$page.params.id}`);
		if (!res.ok) {
			run = null;
			const body = await res.json().catch(() => ({}));
			loadError = body.error ?? 'Failed to load run.';
			return;
		}
		loadError = '';
		run = await res.json();
	}

	function poll() {
		if (run?.job.status === 'queued' || run?.job.status === 'running') {
			timer = setTimeout(async () => {
				await fetchRun();
				poll();
			}, POLL_INTERVAL);
		}
	}

	function durationLabel(sec?: number | null): string {
		if (sec == null || !Number.isFinite(sec)) return '-';
		const total = Math.max(0, Math.floor(sec));
		const minutes = Math.floor(total / 60);
		const seconds = total % 60;
		return `${minutes}:${String(seconds).padStart(2, '0')}`;
	}

	function fileSizeLabel(bytes?: number | null): string {
		if (bytes == null || bytes < 1) return '-';
		const mb = bytes / (1024 * 1024);
		if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
		return `${mb.toFixed(1)} MB`;
	}

	function formatDate(iso: string): string {
		return new Intl.DateTimeFormat('en-US', {
			month: 'short',
			day: 'numeric',
			hour: 'numeric',
			minute: '2-digit'
		}).format(new Date(iso));
	}

	onMount(() => {
		void fetchRun().then(() => {
			poll();
		});
		return () => {
			if (timer) clearTimeout(timer);
		};
	});
</script>

<svelte:head><title>Job - Agentra</title></svelte:head>

{#if loadError}
	<div class="max-w-4xl mx-auto">
		<div class="surface-card p-6 border border-red-200 bg-red-50 text-red-700 text-sm">{loadError}</div>
	</div>
{:else if run}
	<div class="max-w-4xl mx-auto space-y-6">
		<div class="flex flex-wrap items-center justify-between gap-3">
			<div class="flex flex-wrap items-center gap-3">
				<h1 class="text-3xl font-semibold">Project run</h1>
				<StatusBadge status={run.job.status} />
			</div>
			<a href={`/projects/${run.job.project_id}`} class="btn-secondary text-sm">Back to project</a>
		</div>

		<div class="surface-card p-6 grid grid-cols-1 md:grid-cols-2 gap-4">
			<div>
				<div class="data-label mb-2">Variant</div>
				<p class="text-sm text-slate-700">{run.job.variant_index} of {run.job.variant_count}</p>
			</div>
			<div>
				<div class="data-label mb-2">Queued</div>
				<p class="text-sm text-slate-700">{formatDate(run.job.created_at)}</p>
			</div>
			<div>
				<div class="data-label mb-2">Minimum duration</div>
				<p class="text-sm text-slate-700">{durationLabel(run.job.min_duration_sec)}</p>
			</div>
			<div>
				<div class="data-label mb-2">Maximum duration</div>
				<p class="text-sm text-slate-700">{durationLabel(run.job.max_duration_sec)}</p>
			</div>
		</div>

		<div class="surface-card p-6">
			<div class="data-label mb-2">Prompt</div>
			<p class="text-sm text-slate-700 whitespace-pre-wrap">{run.job.prompt || 'No prompt set.'}</p>
		</div>

		{#if run.job.status === 'failed' && run.job.error_message}
			<div class="surface-card p-4 border border-red-200 bg-red-50 text-red-700 text-sm">
				{run.job.error_message}
			</div>
		{/if}

		{#if run.job.status === 'queued' || run.job.status === 'running'}
			<div class="surface-card p-8 text-center">
				<div class="text-sm text-slate-500">Processing... This page refreshes every 5 seconds.</div>
			</div>
		{/if}

		{#if run.timeline}
			<div class="surface-card p-6">
				<h2 class="font-semibold mb-3">Plan snapshot</h2>
				<pre class="rounded-xl bg-slate-950 text-slate-100 text-xs p-4 overflow-x-auto">{JSON.stringify(run.timeline.otio_json, null, 2)}</pre>
			</div>
		{/if}

		<div class="surface-card p-6">
			<h2 class="font-semibold mb-3">Invoke process</h2>
			{#if run.traces.length === 0}
				<p class="text-sm text-slate-500">No invocation trace stored for this run yet.</p>
			{:else}
				<div class="space-y-3">
					{#each run.traces as trace}
						<div class="rounded-2xl border border-slate-200 p-4">
							<div class="flex flex-wrap items-center justify-between gap-2">
								<div>
									<p class="text-xs font-bold uppercase tracking-[0.16em] text-sky-700">{trace.phase}</p>
									<p class="text-sm font-medium text-slate-800 mt-1">{trace.message}</p>
								</div>
								<p class="text-[11px] text-slate-400">{formatDate(trace.created_at)}</p>
							</div>
							<pre class="mt-3 rounded-xl bg-slate-950 text-slate-100 text-xs p-4 overflow-x-auto">{JSON.stringify(trace.payload, null, 2)}</pre>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		{#if run.render}
			<div class="surface-card p-6">
				<div class="flex flex-wrap items-center justify-between gap-3 mb-3">
					<h2 class="font-semibold">Render</h2>
					<a href={`${api.BASE}/renders/${run.render.id}/stream?token=${encodeURIComponent(auth.token ?? '')}`} class="btn-secondary text-sm">
						Download
					</a>
				</div>
				<div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
					<div>
						<div class="data-label mb-2">Duration</div>
						<p class="text-sm text-slate-700">{durationLabel(run.render.duration_sec)}</p>
					</div>
					<div>
						<div class="data-label mb-2">File size</div>
						<p class="text-sm text-slate-700">{fileSizeLabel(run.render.file_size_bytes)}</p>
					</div>
				</div>
				<VideoPlayer src={`${api.BASE}/renders/${run.render.id}/stream`} token={auth.token ?? ''} />
			</div>
		{:else if run.job.status === 'completed'}
			<div class="surface-card p-6">
				<h2 class="font-semibold mb-3">Render</h2>
				<p class="text-sm text-slate-500">This run completed, but no render file was produced by the current editor stub.</p>
			</div>
		{/if}
	</div>
{/if}
