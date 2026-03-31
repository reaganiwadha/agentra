<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import MediaDetailPanel from '$lib/components/MediaDetailPanel.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { Check, Loader2, Plus, Trash2 } from 'lucide-svelte';
	import { onMount } from 'svelte';

	type Project = {
		id: string;
		name: string;
		description?: string | null;
		status: string;
		editor_base_prompt: string;
		editor_variant_count: number;
		editor_min_duration_sec: number;
		editor_max_duration_sec: number;
	};

	type MediaAsset = {
		id: string;
		filename: string;
		status: string;
		mime_type?: string | null;
		duration_sec?: number | null;
	};

	type ProjectRun = {
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

	type DurationUnit = 'seconds' | 'minutes' | 'hours';

	let project = $state<Project | null>(null);
	let media = $state<MediaAsset[]>([]);
	let allMedia = $state<MediaAsset[]>([]);
	let runs = $state<ProjectRun[]>([]);
	let loading = $state(true);
	let detailId = $state<string | null>(null);

	let draftTitle = $state('');
	let basePrompt = $state('');
	let variantCount = $state(1);
	let minDurationValue = $state(30);
	let minDurationUnit = $state<DurationUnit>('seconds');
	let maxDurationValue = $state(1);
	let maxDurationUnit = $state<DurationUnit>('minutes');
	let draftError = $state('');
	let draftSaving = $state(false);
	let queueing = $state(false);

	let assetModalOpen = $state(false);
	let assetModalLoading = $state(false);
	let assetModalSaving = $state(false);
	let assetModalError = $state('');
	let assetFilter = $state('');
	let selectedAssetIDs = $state<string[]>([]);

	let cancellingRunIDs = $state<string[]>([]);

	const projectID = $derived($page.params.id);

	const filteredAvailableMedia = $derived(
		allMedia.filter((asset) => {
			const q = assetFilter.trim().toLowerCase();
			if (!q) return true;
			return asset.filename.toLowerCase().includes(q);
		})
	);

	onMount(async () => {
		await loadAll();
		loading = false;
	});

	async function loadAll() {
		await Promise.all([loadProject(), loadMedia(), loadRuns()]);
	}

	async function loadProject() {
		const res = await api.get(`/projects/${projectID}`);
		if (!res.ok) {
			project = null;
			return;
		}
		const nextProject: Project = await res.json();
		project = nextProject;
		draftTitle = nextProject.name;
		basePrompt = nextProject.editor_base_prompt ?? '';
		variantCount = normalizeVariantCount(nextProject.editor_variant_count);
		const minDuration = durationFromSeconds(nextProject.editor_min_duration_sec);
		minDurationValue = minDuration.value;
		minDurationUnit = minDuration.unit;
		const maxDuration = durationFromSeconds(nextProject.editor_max_duration_sec);
		maxDurationValue = maxDuration.value;
		maxDurationUnit = maxDuration.unit;
	}

	async function loadMedia() {
		const res = await api.get(`/projects/${projectID}/media`);
		if (!res.ok) {
			media = [];
			return;
		}
		media = await res.json();
	}

	async function loadRuns() {
		const res = await api.get(`/projects/${projectID}/runs`);
		if (!res.ok) {
			runs = [];
			return;
		}
		const data = await res.json();
		runs = data.runs ?? [];
	}

	function normalizeVariantCount(value: unknown): number {
		const parsed = Number(value);
		if (!Number.isFinite(parsed)) return 1;
		return Math.min(12, Math.max(1, Math.floor(parsed)));
	}

	function unitSeconds(unit: DurationUnit): number {
		if (unit === 'hours') return 3600;
		if (unit === 'minutes') return 60;
		return 1;
	}

	function durationToSeconds(value: number, unit: DurationUnit): number {
		const safeValue = Math.max(1, Math.floor(Number.isFinite(value) ? value : 1));
		return safeValue * unitSeconds(unit);
	}

	function durationFromSeconds(totalSeconds: number): { value: number; unit: DurationUnit } {
		if (totalSeconds > 0 && totalSeconds % 3600 === 0) {
			return { value: totalSeconds / 3600, unit: 'hours' };
		}
		if (totalSeconds > 0 && totalSeconds % 60 === 0) {
			return { value: totalSeconds / 60, unit: 'minutes' };
		}
		return { value: Math.max(1, totalSeconds || 30), unit: 'seconds' };
	}

	async function saveDraft() {
		draftSaving = true;
		draftError = '';
		const res = await api.put(`/projects/${projectID}`, {
			name: draftTitle.trim() || project?.name || 'Untitled Project',
			base_prompt: basePrompt,
			variant_count: normalizeVariantCount(variantCount),
			min_duration_sec: durationToSeconds(minDurationValue, minDurationUnit),
			max_duration_sec: durationToSeconds(maxDurationValue, maxDurationUnit)
		});
		draftSaving = false;
		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			draftError = body.error ?? 'Failed to save draft.';
			return false;
		}
		const nextProject: Project = await res.json();
		project = nextProject;
		draftTitle = nextProject.name;
		basePrompt = nextProject.editor_base_prompt ?? '';
		variantCount = normalizeVariantCount(nextProject.editor_variant_count);
		const minDuration = durationFromSeconds(nextProject.editor_min_duration_sec);
		minDurationValue = minDuration.value;
		minDurationUnit = minDuration.unit;
		const maxDuration = durationFromSeconds(nextProject.editor_max_duration_sec);
		maxDurationValue = maxDuration.value;
		maxDurationUnit = maxDuration.unit;
		return true;
	}

	async function queueProjectRun() {
		queueing = true;
		draftError = '';
		const res = await api.post(`/projects/${projectID}/runs`, {
			name: draftTitle.trim() || project?.name || 'Untitled Project',
			base_prompt: basePrompt,
			variant_count: normalizeVariantCount(variantCount),
			min_duration_sec: durationToSeconds(minDurationValue, minDurationUnit),
			max_duration_sec: durationToSeconds(maxDurationValue, maxDurationUnit)
		});
		queueing = false;
		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			draftError = body.error ?? 'Failed to queue project runs.';
			return;
		}
		await Promise.all([loadProject(), loadRuns()]);
	}

	async function openAssetModal() {
		assetModalOpen = true;
		assetModalLoading = true;
		assetModalSaving = false;
		assetModalError = '';
		assetFilter = '';
		selectedAssetIDs = media.map((asset) => asset.id);

		const res = await api.get('/media');
		assetModalLoading = false;
		if (!res.ok) {
			allMedia = [];
			assetModalError = 'Failed to load available media.';
			return;
		}
		allMedia = await res.json();
	}

	function closeAssetModal() {
		if (assetModalSaving) return;
		assetModalOpen = false;
		assetModalError = '';
		assetFilter = '';
	}

	function toggleAssetSelection(id: string) {
		if (selectedAssetIDs.includes(id)) {
			selectedAssetIDs = selectedAssetIDs.filter((item) => item !== id);
			return;
		}
		selectedAssetIDs = [...selectedAssetIDs, id];
	}

	async function saveSelectedAssets() {
		assetModalSaving = true;
		assetModalError = '';
		const res = await api.put(`/projects/${projectID}/media-scope`, {
			mode: 'selected',
			media_ids: selectedAssetIDs
		});
		assetModalSaving = false;
		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			assetModalError = body.error ?? 'Failed to update media asset pool.';
			return;
		}
		await loadMedia();
		closeAssetModal();
	}

	async function removeAsset(id: string) {
		const next = media.filter((asset) => asset.id !== id).map((asset) => asset.id);
		const res = await api.put(`/projects/${projectID}/media-scope`, {
			mode: 'selected',
			media_ids: next
		});
		if (!res.ok) return;
		media = media.filter((asset) => asset.id !== id);
		if (detailId === id) detailId = null;
	}

	async function cancelRun(id: string) {
		cancellingRunIDs = [...cancellingRunIDs, id];
		const res = await api.post(`/runs/${id}/cancel`);
		cancellingRunIDs = cancellingRunIDs.filter((item) => item !== id);
		if (!res.ok) return;
		await loadRuns();
	}

	function assetDurationLabel(sec?: number | null): string {
		if (sec == null) return '-';
		const total = Math.max(0, Math.floor(sec));
		const m = Math.floor(total / 60);
		const s = total % 60;
		return `${m}:${String(s).padStart(2, '0')}`;
	}

	function assetKind(asset: MediaAsset): string {
		const mime = (asset.mime_type || '').toLowerCase();
		if (mime.startsWith('video/')) return 'video';
		if (mime.startsWith('audio/')) return 'audio';
		if (mime.startsWith('image/')) return 'image';
		return 'asset';
	}

	function formatDate(iso: string): string {
		return new Intl.DateTimeFormat('en-US', {
			month: 'short',
			day: 'numeric',
			hour: 'numeric',
			minute: '2-digit'
		}).format(new Date(iso));
	}

	function durationLabel(sec: number): string {
		const total = Math.max(0, Math.floor(sec));
		const minutes = Math.floor(total / 60);
		const seconds = total % 60;
		return `${minutes}:${String(seconds).padStart(2, '0')}`;
	}

	function runStatusClasses(status: ProjectRun['status']): string {
		if (status === 'queued') return 'bg-amber-50 text-amber-700 border-amber-200';
		if (status === 'running') return 'bg-sky-50 text-sky-700 border-sky-200';
		if (status === 'completed') return 'bg-emerald-50 text-emerald-700 border-emerald-200';
		if (status === 'failed') return 'bg-red-50 text-red-700 border-red-200';
		return 'bg-slate-100 text-slate-500 border-slate-200';
	}

	function isCancelling(runID: string): boolean {
		return cancellingRunIDs.includes(runID);
	}
</script>

<svelte:head>
	<title>{project ? `${project.name} - Project - Agentra` : 'Project - Agentra'}</title>
</svelte:head>

<div class="max-w-6xl mx-auto space-y-6">
	<div class="flex flex-wrap items-end justify-between gap-4">
		<div>
			<p class="text-[11px] font-bold uppercase tracking-[0.2em] text-sky-600">Project detail</p>
			<h1 class="text-3xl font-semibold mt-1">{project?.name ?? 'Project'}</h1>
			<p class="text-sm text-slate-500 mt-1">
				Prepare a run draft, define the media asset pool, and queue variants.
			</p>
		</div>
		<div class="flex items-center gap-3">
			{#if project}
				<StatusBadge status={project.status} />
			{/if}
			<a href="/projects" class="btn-secondary text-sm">Back to projects</a>
		</div>
	</div>

	<div class="grid grid-cols-1 xl:grid-cols-2 gap-6 items-start">
		<section class="surface-card overflow-hidden">
			<div class="px-6 py-5 border-b border-slate-100">
				<h2 class="text-xl font-semibold text-slate-900">Draft</h2>
				<p class="text-sm text-slate-500 mt-1">
					Set the run inputs here. Variants reuse the same parameters and split into separate queued runs.
				</p>
			</div>

			<div class="p-6 space-y-6">
				{#if draftError}
					<div class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{draftError}</div>
				{/if}

				<div>
					<label for="draft-title" class="block text-xs font-bold uppercase tracking-[0.16em] text-slate-400 mb-2">
						Project title
					</label>
					<input id="draft-title" type="text" bind:value={draftTitle} class="input-base" />
				</div>

				<div>
					<label for="base-prompt" class="block text-xs font-bold uppercase tracking-[0.16em] text-slate-400 mb-2">
						Base prompt
					</label>
					<textarea
						id="base-prompt"
						rows="7"
						bind:value={basePrompt}
						class="input-base resize-y"
						placeholder="Describe the tone, intent, and what the editor should prioritize."
					></textarea>
				</div>

				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<div>
						<label for="variant-count" class="block text-xs font-bold uppercase tracking-[0.16em] text-slate-400 mb-2">
							Variant count
						</label>
						<input id="variant-count" type="number" min="1" max="12" bind:value={variantCount} class="input-base" />
					</div>
					<div class="rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3">
						<p class="text-[10px] font-bold uppercase tracking-[0.16em] text-slate-400">Run summary</p>
						<p class="text-sm text-slate-700 mt-1">{media.length} media asset{media.length === 1 ? '' : 's'}</p>
						<p class="text-sm text-slate-700">{normalizeVariantCount(variantCount)} queued variant{normalizeVariantCount(variantCount) === 1 ? '' : 's'}</p>
					</div>
				</div>

				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<div>
						<label for="min-duration-value" class="block text-xs font-bold uppercase tracking-[0.16em] text-slate-400 mb-2">
							Minimum duration
						</label>
						<div class="grid grid-cols-[1fr_140px] gap-2">
							<input id="min-duration-value" type="number" min="1" bind:value={minDurationValue} class="input-base" />
							<select bind:value={minDurationUnit} class="input-base" aria-label="Minimum duration unit">
								<option value="seconds">Seconds</option>
								<option value="minutes">Minutes</option>
								<option value="hours">Hours</option>
							</select>
						</div>
					</div>
					<div>
						<label for="max-duration-value" class="block text-xs font-bold uppercase tracking-[0.16em] text-slate-400 mb-2">
							Maximum duration
						</label>
						<div class="grid grid-cols-[1fr_140px] gap-2">
							<input id="max-duration-value" type="number" min="1" bind:value={maxDurationValue} class="input-base" />
							<select bind:value={maxDurationUnit} class="input-base" aria-label="Maximum duration unit">
								<option value="seconds">Seconds</option>
								<option value="minutes">Minutes</option>
								<option value="hours">Hours</option>
							</select>
						</div>
					</div>
				</div>

				<div class="rounded-2xl border border-slate-200 overflow-hidden">
					<div class="px-4 py-3 border-b border-slate-100 flex items-center justify-between gap-3">
						<div>
							<h3 class="font-semibold text-slate-900">Media asset pool</h3>
							<p class="text-xs text-slate-400 mt-0.5">Only selected assets are in scope</p>
						</div>
						<button type="button" class="btn-secondary text-xs px-3 py-2 inline-flex items-center gap-2" onclick={openAssetModal}>
							<Plus size={13} />
							Add Assets
						</button>
					</div>

					{#if loading}
						<div class="px-4 py-8 text-sm text-slate-400">Loading media assets...</div>
					{:else if media.length === 0}
						<div class="px-4 py-10 text-center">
							<p class="text-sm font-medium text-slate-700">No assets selected</p>
							<p class="text-xs text-slate-400 mt-1">The asset pool starts empty. Add only the media this project should use.</p>
						</div>
					{:else}
						<div class="divide-y divide-slate-100">
							{#each media as asset}
								<div class="px-4 py-3 flex items-start gap-3">
									<button
										type="button"
										class="min-w-0 flex-1 text-left rounded-xl px-2 py-1.5 hover:bg-slate-50 transition"
										onclick={() => (detailId = asset.id)}
									>
										<div class="flex items-start justify-between gap-3">
											<div class="min-w-0">
												<p class="text-sm font-medium text-slate-800 truncate">{asset.filename}</p>
												<div class="flex items-center gap-2 mt-1 text-[11px] text-slate-400">
													<span class="uppercase tracking-wide">{assetKind(asset)}</span>
													<span>{assetDurationLabel(asset.duration_sec)}</span>
												</div>
											</div>
											<StatusBadge status={asset.status} />
										</div>
									</button>
									<button
										type="button"
										class="mt-1 h-8 w-8 shrink-0 inline-flex items-center justify-center rounded-lg border border-slate-200 text-slate-400 hover:border-red-200 hover:bg-red-50 hover:text-red-600 transition"
										onclick={() => removeAsset(asset.id)}
										aria-label={`Remove ${asset.filename}`}
									>
										<Trash2 size={14} />
									</button>
								</div>
							{/each}
						</div>
					{/if}
				</div>

				<div class="flex justify-end gap-2">
					<button type="button" class="btn-secondary text-sm" onclick={saveDraft} disabled={draftSaving || queueing}>
						{draftSaving ? 'Saving...' : 'Save Draft'}
					</button>
					<button type="button" class="btn-primary text-sm" onclick={queueProjectRun} disabled={loading || media.length === 0 || queueing}>
						{queueing ? 'Queueing...' : 'Queue Project Run'}
					</button>
				</div>
			</div>
		</section>

		<section class="surface-card overflow-hidden">
			<div class="px-6 py-5 border-b border-slate-100">
				<h2 class="text-xl font-semibold text-slate-900">Runs</h2>
				<p class="text-sm text-slate-500 mt-1">
					Each queued variant becomes its own run entry with the same draft parameters.
				</p>
			</div>

			<div class="p-6">
				{#if loading}
					<div class="rounded-2xl border border-dashed border-slate-200 bg-slate-50/70 px-5 py-12 text-center">
						<p class="text-sm text-slate-500">Loading runs...</p>
					</div>
				{:else if runs.length === 0}
					<div class="rounded-2xl border border-dashed border-slate-200 bg-slate-50/70 px-5 py-12 text-center">
						<p class="text-sm font-medium text-slate-700">No queued runs yet</p>
						<p class="text-xs text-slate-400 mt-1">Queue the draft to create one run per variant.</p>
					</div>
				{:else}
					<div class="space-y-3">
						{#each runs as run}
							<div class="rounded-2xl border border-slate-200 px-4 py-4">
								<div class="flex items-start justify-between gap-3">
									<div class="min-w-0">
										<div class="flex items-center gap-2 flex-wrap">
											<p class="text-sm font-semibold text-slate-900 truncate">{project?.name ?? 'Project run'}</p>
											<span class="text-[10px] font-bold uppercase tracking-[0.16em] text-slate-400">
												Variant {run.variant_index}/{run.variant_count}
											</span>
										</div>
										<p class="text-[11px] text-slate-400 mt-1">Queued {formatDate(run.created_at)}</p>
									</div>
									<span class={`text-[10px] font-bold px-2.5 py-1 rounded-full border ${runStatusClasses(run.status)}`}>
										{run.status}
									</span>
								</div>

								{#if run.prompt}
									<p class="text-xs text-slate-600 leading-relaxed mt-3 line-clamp-3">{run.prompt}</p>
								{:else}
									<p class="text-xs text-slate-400 mt-3 italic">No base prompt set for this run.</p>
								{/if}

								<p class="text-[11px] text-slate-400 mt-3">
									Target window {durationLabel(run.min_duration_sec)} to {durationLabel(run.max_duration_sec)}
								</p>

								{#if run.error_message}
									<div class="mt-3 rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">
										{run.error_message}
									</div>
								{/if}

								<div class="flex justify-between items-center gap-3 mt-4">
									<a href={`/jobs/${run.id}`} class="text-xs font-medium text-sky-700 hover:text-sky-900 transition">
										View run
									</a>
									{#if run.status === 'queued' || run.status === 'running'}
										<button
											type="button"
											class="btn-secondary text-xs px-3 py-2"
											onclick={() => cancelRun(run.id)}
											disabled={isCancelling(run.id)}
										>
											{isCancelling(run.id) ? 'Cancelling...' : 'Cancel Run'}
										</button>
									{:else if run.status === 'cancelled'}
										<span class="text-xs text-slate-400">Run cancelled</span>
									{/if}
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</div>
		</section>
	</div>
</div>

<MediaDetailPanel
	mediaId={detailId}
	token={api.token()}
	onClose={() => {
		detailId = null;
	}}
/>

<Modal
	open={assetModalOpen}
	title="Media Asset Pool"
	description="Selected assets belong to this project. Unselected assets stay out of scope."
	widthClass="max-w-3xl"
	closeOnBackdrop={true}
	onClose={closeAssetModal}
>
	<div class="space-y-4">
		{#if assetModalError}
			<div class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{assetModalError}</div>
		{/if}

		<div class="flex items-center justify-between gap-3">
			<input
				type="text"
				bind:value={assetFilter}
				placeholder="Filter available media by filename"
				class="input-base"
				disabled={assetModalLoading || assetModalSaving}
			/>
			<div class="text-xs text-slate-400 whitespace-nowrap">{selectedAssetIDs.length} selected</div>
		</div>

		{#if assetModalLoading}
			<div class="py-14 flex flex-col items-center justify-center text-center">
				<Loader2 size={26} class="text-sky-400 animate-spin mb-3" />
				<p class="text-sm text-slate-500">Loading available media...</p>
			</div>
		{:else if filteredAvailableMedia.length === 0}
			<div class="rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-5 py-10 text-center">
				<p class="text-sm font-medium text-slate-700">No matching media</p>
				<p class="text-xs text-slate-400 mt-1">Clear the filter or upload more org media.</p>
			</div>
		{:else}
			<div class="rounded-2xl border border-slate-200 overflow-hidden">
				<div class="max-h-[55vh] overflow-y-auto divide-y divide-slate-100">
					{#each filteredAvailableMedia as asset}
						<button
							type="button"
							class={`w-full px-4 py-3 flex items-center gap-3 text-left transition ${selectedAssetIDs.includes(asset.id) ? 'bg-sky-50/60' : 'hover:bg-slate-50'}`}
							onclick={() => toggleAssetSelection(asset.id)}
						>
							<div class={`h-5 w-5 shrink-0 rounded border flex items-center justify-center ${selectedAssetIDs.includes(asset.id) ? 'border-sky-500 bg-sky-500 text-white' : 'border-slate-300 bg-white text-transparent'}`}>
								<Check size={12} />
							</div>
							<div class="min-w-0 flex-1">
								<div class="flex items-center gap-2 flex-wrap">
									<p class="text-sm font-medium text-slate-800 truncate">{asset.filename}</p>
									{#if selectedAssetIDs.includes(asset.id)}
										<span class="text-[10px] font-bold uppercase tracking-[0.16em] text-sky-700">Selected</span>
									{/if}
								</div>
								<div class="flex items-center gap-2 mt-1 text-[11px] text-slate-400">
									<span class="uppercase tracking-wide">{assetKind(asset)}</span>
									<span>{assetDurationLabel(asset.duration_sec)}</span>
								</div>
							</div>
							<StatusBadge status={asset.status} />
						</button>
					{/each}
				</div>
			</div>
		{/if}

		<div class="flex justify-end gap-2 pt-1">
			<button type="button" class="btn-secondary text-sm" onclick={closeAssetModal} disabled={assetModalSaving}>
				Cancel
			</button>
			<button
				type="button"
				class="btn-primary text-sm inline-flex items-center gap-2"
				onclick={saveSelectedAssets}
				disabled={assetModalLoading || assetModalSaving}
			>
				{#if assetModalSaving}
					<Loader2 size={14} class="animate-spin" />
					Saving...
				{:else}
					Save Asset Pool
				{/if}
			</button>
		</div>
	</div>
</Modal>
