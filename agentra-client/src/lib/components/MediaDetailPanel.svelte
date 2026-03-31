<script lang="ts">
	import { X, RotateCcw, Trash2 } from 'lucide-svelte';
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import AuthenticatedImage from './AuthenticatedImage.svelte';
	import StatusBadge from './StatusBadge.svelte';

	type AnalyzerResult = {
		analyzer_id: string;
		analyzer_name: string;
		analyzer_type: 'transcription' | 'vision_tags' | 'embedding' | string;
		status: 'ready' | 'pending' | string;
		output: any | null;
		analyzed_at: string | null;
	};

	type MediaDetail = {
		id: string;
		filename: string;
		thumbnail_path: string | null;
		mime_type: string | null;
		sha256: string | null;
		duration_sec: number | null;
		file_size_bytes: number | null;
		captured_at: string | null;
		status: string;
		created_at: string;
		updated_at: string;
		analyzer_results: AnalyzerResult[];
	};

	let {
		mediaId,
		token,
		onClose,
		onDeleted
	}: {
		mediaId: string | null;
		token: string;
		onClose: () => void;
		onDeleted?: (id: string) => void;
	} = $props();

	let detail = $state<MediaDetail | null>(null);
	let loading = $state(false);
	let error = $state(false);
	let confirmDelete = $state(false);
	let deleting = $state(false);
	let reanalyzing = $state(false);

	// ID of the latest completed/failed activity log for this asset — drives auto-refresh
	const latestCompletionLogId = $derived(
		mediaId
			? (activity.logs.find(
					(l) =>
						l.subject_id === mediaId &&
						(l.event_type === 'completed' || l.event_type === 'failed')
				)?.id ?? null)
			: null
	);

	$effect(() => {
		const logId = latestCompletionLogId;
		if (!logId || !mediaId) return;
		api
			.get(`/media-assets/${mediaId}`)
			.then((r) => (r.ok ? r.json() : null))
			.then((d) => {
				if (d) detail = d;
			});
	});

	// Latest step message for this asset while backend is working on it
	const latestStepMessage = $derived(
		mediaId && activity.active
			? (activity.logs.find((l) => l.subject_id === mediaId && l.event_type === 'progress')
					?.message ?? null)
			: null
	);

	$effect(() => {
		const id = mediaId;
		if (!id) {
			detail = null;
			return;
		}
		loading = true;
		error = false;
		detail = null;
		api
			.get(`/media-assets/${id}`)
			.then((r) => (r.ok ? r.json() : null))
			.then((d) => {
				detail = d;
				loading = false;
				if (!d) error = true;
			})
			.catch(() => {
				loading = false;
				error = true;
			});
	});

	function formatDuration(sec: number | null): string {
		if (sec == null) return '-';
		const h = Math.floor(sec / 3600);
		const m = Math.floor((sec % 3600) / 60);
		const s = Math.floor(sec % 60);
		if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
		return `${m}:${String(s).padStart(2, '0')}`;
	}

	function formatFileSize(bytes: number | null): string {
		if (bytes == null) return '-';
		if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
		if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
		return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
	}

	function formatDate(iso: string | null): string {
		if (!iso) return '-';
		return new Intl.DateTimeFormat('en-US', {
			month: 'short',
			day: 'numeric',
			year: 'numeric',
			hour: 'numeric',
			minute: '2-digit'
		}).format(new Date(iso));
	}

	function transcriptText(item: AnalyzerResult): string {
		if (!item.output || typeof item.output !== 'object') return '';
		const text = item.output.text;
		return typeof text === 'string' ? text.trim() : '';
	}

	function transcriptSegments(item: AnalyzerResult): Array<{ start: number; end: number; text: string }> {
		if (!item.output || !Array.isArray(item.output.segments)) return [];
		return item.output.segments;
	}

	function visionIsSegmented(item: AnalyzerResult): boolean {
		return !!(item.output && typeof item.output === 'object' && Array.isArray(item.output.segments));
	}

	function visionSegments(item: AnalyzerResult): Array<{ frame_number: number; timestamp_sec: number; description: string; thumbnail_storage_path: string }> {
		if (!item.output || !Array.isArray(item.output.segments)) return [];
		return item.output.segments;
	}

	function visionSummary(item: AnalyzerResult): string {
		if (!item.output || typeof item.output !== 'object') return '';
		return typeof item.output.summary === 'string' ? item.output.summary.trim() : '';
	}

	function visionDescription(item: AnalyzerResult): string {
		if (!item.output || typeof item.output !== 'object') return '';
		const description = item.output.description;
		return typeof description === 'string' ? description.trim() : '';
	}

	function visionTags(item: AnalyzerResult): string[] {
		if (!item.output || typeof item.output !== 'object') return [];
		const tags = item.output.tags;
		if (!Array.isArray(tags)) return [];
		return tags.filter((tag) => typeof tag === 'string' && tag.trim() !== '');
	}

	function mediaKind(mime: string | null): 'video' | 'audio' | 'image' | 'other' {
		if (!mime) return 'other';
		const m = mime.toLowerCase();
		if (m.startsWith('video/')) return 'video';
		if (m.startsWith('audio/')) return 'audio';
		if (m.startsWith('image/')) return 'image';
		return 'other';
	}

	function streamUrl(id: string) {
		return `${api.BASE}/media-assets/${id}/stream?token=${encodeURIComponent(token)}`;
	}

	function formatTimestamp(sec: number): string {
		const m = Math.floor(sec / 60);
		const s = Math.floor(sec % 60);
		return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
	}

	async function handleReanalyze() {
		if (!detail) return;
		reanalyzing = true;
		try {
			const res = await api.post(`/media-assets/${detail.id}/reset-analysis`);
			if (res.ok) {
				detail = await res.json();
			}
		} finally {
			reanalyzing = false;
		}
	}

	async function handleDelete() {
		if (!detail) return;
		if (!confirmDelete) {
			confirmDelete = true;
			return;
		}
		deleting = true;
		try {
			const res = await api.del(`/media-assets/${detail.id}`);
			if (res.ok) {
				onDeleted?.(detail.id);
				onClose();
			}
		} finally {
			deleting = false;
			confirmDelete = false;
		}
	}
</script>

{#if mediaId}
	<button
		type="button"
		class="fixed inset-0 z-40 bg-slate-900/30 backdrop-blur-[1px]"
		aria-label="Close panel"
		onclick={onClose}
	></button>

	<div
		class="fixed right-0 top-0 bottom-0 z-50 w-full max-w-[460px] bg-white border-l border-slate-200 shadow-2xl flex flex-col overflow-hidden animate-in slide-in-from-right duration-200"
	>
		<div class="px-5 py-3.5 border-b border-slate-100 flex items-center justify-between gap-3 shrink-0">
			<h2 class="text-sm font-semibold text-slate-800 truncate">
				{#if detail}{detail.filename}{:else}Media detail{/if}
			</h2>
			<div class="flex items-center gap-1.5 shrink-0">
				{#if detail}
					<button
						type="button"
						onclick={handleReanalyze}
						disabled={reanalyzing}
						class="h-7 px-2 inline-flex items-center gap-1 rounded-md border border-slate-200 text-slate-400 hover:bg-slate-100 hover:text-slate-700 transition-colors text-[11px] disabled:opacity-50"
						title="Re-analyze"
					>
						<RotateCcw size={12} class={reanalyzing ? 'animate-spin' : ''} />
					</button>
					{#if confirmDelete}
						<button
							type="button"
							onclick={handleDelete}
							disabled={deleting}
							class="h-7 px-2 inline-flex items-center gap-1 rounded-md border border-red-300 bg-red-50 text-red-600 hover:bg-red-100 transition-colors text-[11px] font-medium disabled:opacity-50"
						>
							{deleting ? 'Deleting…' : 'Confirm?'}
						</button>
						<button
							type="button"
							onclick={() => (confirmDelete = false)}
							class="h-7 w-7 inline-flex items-center justify-center rounded-md border border-slate-200 text-slate-400 hover:bg-slate-100 hover:text-slate-700 transition-colors"
						>
							<X size={12} />
						</button>
					{:else}
						<button
							type="button"
							onclick={handleDelete}
							class="h-7 w-7 inline-flex items-center justify-center rounded-md border border-slate-200 text-slate-400 hover:bg-red-50 hover:border-red-200 hover:text-red-500 transition-colors"
							title="Delete"
						>
							<Trash2 size={13} />
						</button>
					{/if}
				{/if}
				<button
					type="button"
					onclick={onClose}
					class="h-7 w-7 shrink-0 inline-flex items-center justify-center rounded-md border border-slate-200 text-slate-400 hover:bg-slate-100 hover:text-slate-700 transition-colors"
					aria-label="Close"
				>
					<X size={13} />
				</button>
			</div>
		</div>

		{#if loading}
			<div class="flex-1 flex items-center justify-center text-sm text-slate-400">Loading...</div>
		{:else if error || !detail}
			<div class="flex-1 flex items-center justify-center text-sm text-slate-400">Could not load media.</div>
		{:else}
			<div class="flex-1 overflow-y-auto">
				{#if mediaKind(detail.mime_type) === 'video'}
					<div class="bg-slate-950">
						<!-- svelte-ignore a11y_media_has_caption -->
						<video
							controls
							class="w-full max-h-64 outline-none block"
							src={streamUrl(detail.id)}
							preload="metadata"
						></video>
					</div>
				{:else if mediaKind(detail.mime_type) === 'audio'}
					<div class="bg-slate-100 px-5 py-6">
						<!-- svelte-ignore a11y_media_has_caption -->
						<audio controls class="w-full" src={streamUrl(detail.id)} preload="metadata"></audio>
					</div>
				{:else if detail.thumbnail_path}
					<div class="bg-slate-950 flex items-center justify-center" style="height: 220px;">
						<AuthenticatedImage
							src={`${api.BASE}/media-assets/${detail.id}/thumbnail`}
							{token}
							alt={detail.filename}
							className="max-h-full max-w-full object-contain"
							zoomable
						/>
					</div>
				{/if}

				<div class="p-5 space-y-6">
					<section>
						<h3 class="text-[10px] font-bold text-slate-400 uppercase tracking-widest mb-3">Metadata</h3>
						<dl class="space-y-2.5">
							<div class="flex items-center justify-between gap-4">
								<dt class="text-xs text-slate-500 shrink-0">Status</dt>
								<dd><StatusBadge status={detail.status} /></dd>
							</div>
							{#if detail.mime_type}
								<div class="flex items-center justify-between gap-4">
									<dt class="text-xs text-slate-500 shrink-0">Type</dt>
									<dd class="text-xs text-slate-700 font-mono">{detail.mime_type}</dd>
								</div>
							{/if}
							{#if detail.duration_sec != null}
								<div class="flex items-center justify-between gap-4">
									<dt class="text-xs text-slate-500 shrink-0">Duration</dt>
									<dd class="text-xs text-slate-700 font-mono">{formatDuration(detail.duration_sec)}</dd>
								</div>
							{/if}
							{#if detail.file_size_bytes != null}
								<div class="flex items-center justify-between gap-4">
									<dt class="text-xs text-slate-500 shrink-0">Size</dt>
									<dd class="text-xs text-slate-700">{formatFileSize(detail.file_size_bytes)}</dd>
								</div>
							{/if}
							{#if detail.captured_at}
								<div class="flex items-center justify-between gap-4">
									<dt class="text-xs text-slate-500 shrink-0">Captured</dt>
									<dd class="text-xs text-slate-700 text-right">{formatDate(detail.captured_at)}</dd>
								</div>
							{/if}
							<div class="flex items-center justify-between gap-4">
								<dt class="text-xs text-slate-500 shrink-0">Uploaded</dt>
								<dd class="text-xs text-slate-700 text-right">{formatDate(detail.created_at)}</dd>
							</div>
							{#if detail.sha256}
								<div class="flex items-start justify-between gap-4">
									<dt class="text-xs text-slate-500 shrink-0">SHA-256</dt>
									<dd class="text-[10px] text-slate-400 font-mono break-all text-right">{detail.sha256.slice(0, 16)}...</dd>
								</div>
							{/if}
						</dl>
					</section>

					<section>
						<div class="flex items-center gap-2 mb-3">
							<h3 class="text-[10px] font-bold text-slate-400 uppercase tracking-widest">Analysis</h3>
							{#if latestStepMessage}
								<div class="flex items-center gap-1.5 text-[10px] text-sky-600 font-medium min-w-0">
									<div class="h-2.5 w-2.5 border-[1.5px] border-sky-500 border-t-transparent rounded-full animate-spin shrink-0"></div>
									<span class="truncate">{latestStepMessage}</span>
								</div>
							{/if}
						</div>

						{#if detail.analyzer_results.length > 0}
							<div class="space-y-3">
								{#each detail.analyzer_results as item}
									<div class="rounded-lg border border-slate-200 bg-white p-3">
										<div class="flex items-center justify-between gap-3 mb-2">
											<div>
												<div class="text-xs font-semibold text-slate-800">{item.analyzer_name}</div>
												<div class="text-[10px] text-slate-400 uppercase tracking-wide">{item.analyzer_type}</div>
											</div>
											<StatusBadge status={item.status} />
										</div>

										{#if item.status === 'ready'}
											{#if item.analyzer_type === 'vision_tags'}
												{#if visionIsSegmented(item)}
													{@const segments = visionSegments(item)}
													{@const summary = visionSummary(item)}
													{#if summary}
														<p class="text-xs text-slate-700 leading-relaxed mb-3">{summary}</p>
													{/if}
													{#if segments.length > 0}
														<div class="space-y-2.5 max-h-72 overflow-y-auto overflow-x-hidden">
															{#each segments as seg}
																<div class="flex gap-2 items-start">
																	{#if seg.thumbnail_storage_path}
																		<AuthenticatedImage
																			src={`${api.BASE}/media-assets/${detail!.id}/segment-frame/${seg.frame_number}`}
																			{token}
																			alt={`Frame at ${formatTimestamp(seg.timestamp_sec)}`}
																			className="w-20 h-12 object-cover rounded shrink-0"
																			zoomable
																		/>
																	{/if}
																	<div class="min-w-0">
																		<span class="inline-block px-1.5 py-0.5 rounded bg-slate-100 text-slate-500 text-[10px] font-mono mb-0.5">{formatTimestamp(seg.timestamp_sec)}</span>
																		<p class="text-xs text-slate-700 leading-relaxed break-words">{seg.description}</p>
																	</div>
																</div>
															{/each}
														</div>
													{/if}
													{#if !summary && segments.length === 0}
														<p class="text-xs text-slate-400">Ready, but output was empty.</p>
													{/if}
												{:else}
													{@const description = visionDescription(item)}
													{@const tags = visionTags(item)}
													{#if description}
														<p class="text-xs text-slate-700 leading-relaxed mb-2">{description}</p>
													{/if}
													{#if tags.length > 0}
														<div class="flex flex-wrap gap-1">
															{#each tags as tag}
																<span class="px-2 py-0.5 rounded-full bg-sky-50 text-sky-700 border border-sky-100 text-[10px] font-medium">{tag}</span>
															{/each}
														</div>
													{/if}
													{#if !description && tags.length === 0}
														<p class="text-xs text-slate-400">Ready, but output was empty.</p>
													{/if}
												{/if}
											{:else if item.analyzer_type === 'transcription'}
												{@const text = transcriptText(item)}
												{@const segs = transcriptSegments(item)}
												{#if text}
													<div class="bg-slate-50 rounded-lg p-2.5 max-h-40 overflow-y-auto mb-2">
														<p class="text-xs text-slate-700 leading-relaxed whitespace-pre-wrap">{text}</p>
													</div>
												{/if}
												{#if segs.length > 0}
													<details>
														<summary class="text-[10px] text-slate-500 cursor-pointer select-none hover:text-slate-700">Segments ({segs.length})</summary>
														<div class="mt-1.5 space-y-1 max-h-48 overflow-y-auto pl-1">
															{#each segs as seg}
																<div class="flex gap-2 items-baseline">
																	<span class="text-[10px] font-mono text-slate-400 shrink-0">[{formatTimestamp(seg.start)}]</span>
																	<span class="text-xs text-slate-600">{seg.text}</span>
																</div>
															{/each}
														</div>
													</details>
												{/if}
												{#if !text && segs.length === 0}
													<p class="text-xs text-slate-400">Ready, but output was empty.</p>
												{/if}
											{:else if item.analyzer_type === 'embedding'}
												<p class="text-xs text-slate-600">Embedding ready.</p>
											{:else}
												<p class="text-xs text-slate-600">Analyzer output ready.</p>
											{/if}
											{#if item.analyzed_at}
												<p class="text-[10px] text-slate-400 mt-2">Analyzed {formatDate(item.analyzed_at)}</p>
											{/if}
										{:else}
											<p class="text-xs text-slate-400">Pending...</p>
										{/if}
									</div>
								{/each}
							</div>
						{:else if detail.status === 'pending'}
							<div class="flex items-center gap-2 text-xs text-slate-400">
								{#if latestStepMessage}
									<div class="h-3 w-3 border-[1.5px] border-slate-400 border-t-transparent rounded-full animate-spin shrink-0"></div>
									<span>{latestStepMessage}</span>
								{:else}
									<span>Waiting for analyzer...</span>
								{/if}
							</div>
						{:else if detail.status === 'ready'}
							<p class="text-xs text-slate-400">No analyzers configured.</p>
						{:else}
							<p class="text-xs text-slate-400">No analyzer results.</p>
						{/if}
					</section>
				</div>
			</div>
		{/if}
	</div>
{/if}
