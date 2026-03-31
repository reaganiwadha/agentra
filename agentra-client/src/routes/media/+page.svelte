<script lang="ts">
	import { onMount } from 'svelte';
	import { Sparkles, Upload, X, CheckCircle2, Play, Music2, Image as ImageIcon, Download, Trash2, Share2, Loader2 } from 'lucide-svelte';
	import { api } from '$lib/api';
	import { auth, clear } from '$lib/auth.svelte';
	import { activity } from '$lib/activity.svelte';
	import { goto } from '$app/navigation';
	import Modal from '$lib/components/Modal.svelte';
	import AuthenticatedImage from '$lib/components/AuthenticatedImage.svelte';
	import MediaDetailPanel from '$lib/components/MediaDetailPanel.svelte';
	import BackendActivityButton from '$lib/components/BackendActivityButton.svelte';
	import EmbeddingSearchPanel from '$lib/components/EmbeddingSearchPanel.svelte';

	type Project = {
		id: string;
		name: string;
	};

	type MediaAsset = {
		id: string;
		project_id: string | null;
		filename: string;
		thumbnail_path: string | null;
		mime_type: string | null;
		status: string;
		captured_at: string | null;
		created_at: string;
	};

	type DayGroup = {
		dayKey: string;
		dayLabel: string;
		items: MediaAsset[];
	};

	type MonthGroup = {
		monthKey: string;
		monthLabel: string;
		days: DayGroup[];
	};

	type SearchHit = {
		media_id: string;
		filename: string;
		storage_path: string;
		segment_index?: number;
		start_sec?: number;
		end_sec?: number;
		source_text: string;
		score: number;
	};

	let projects = $state<Project[]>([]);
	let media = $state<MediaAsset[]>([]);
	let loading = $state(true);
	let searchQuery = $state('');
	let selectedIds = $state<string[]>([]);

	let detailId = $state<string | null>(null);
	let confirmBulkDelete = $state(false);
	let confirmClearAll = $state(false);

	// Semantic search
	const suggestions = [
		'players celebrating a goal',
		'crowd reaction in the stands',
		'penalty kick sequence',
		'referee making a call',
		'team huddle at half-time',
		'training drill on the pitch',
		'post-match interview',
		'substitution being made',
		'free kick routine',
		'goalkeeper making a save'
	];
	let suggestionIdx = $state(0);
	let searchFocused = $state(false);
	let semanticQuery = $state('');
	let semanticLoading = $state(false);
	let semanticError = $state('');
	let semanticHits = $state<SearchHit[] | null>(null);

	onMount(() => {
		const interval = setInterval(() => {
			suggestionIdx = (suggestionIdx + 1) % suggestions.length;
		}, 3000);
		return () => clearInterval(interval);
	});

	// Patch asset statuses in the list from the live activity stream
	$effect(() => {
		const latest = activity.logs[0];
		if (!latest || latest.subject_type !== 'media_asset' || !latest.subject_id) return;
		const assetId = latest.subject_id;
		const idx = media.findIndex((m) => m.id === assetId);
		if (idx === -1) return;
		let newStatus: string | null = null;
		if (latest.event_type === 'completed') newStatus = 'ready';
		else if (latest.event_type === 'failed') newStatus = 'failed';
		else if (latest.event_type === 'progress') newStatus = 'analyzing';
		if (newStatus && media[idx].status !== newStatus) {
			media = media.map((m) => (m.id === assetId ? { ...m, status: newStatus! } : m));
		}
	});

	let uploadOpen = $state(false);
	let uploadSubmitting = $state(false);
	let uploadError = $state('');
	let uploadFiles = $state<File[]>([]);
	let uploadAsFolder = $state(false);
	let uploadProgress = $state({ current: 0, total: 0 });

	const monthFormatter = new Intl.DateTimeFormat('en-US', { month: 'long', year: 'numeric' });
	const dayFormatter = new Intl.DateTimeFormat('en-US', { weekday: 'short', month: 'short', day: 'numeric' });
	const timeFormatter = new Intl.DateTimeFormat('en-US', { hour: 'numeric', minute: '2-digit' });

	const projectNameByID = $derived(new Map(projects.map((project) => [project.id, project.name])));

	const filteredMedia = $derived(
		[...media]
			.filter((item) => {
				const q = searchQuery.trim().toLowerCase();
				if (!q) return true;
				const projectName = item.project_id ? (projectNameByID.get(item.project_id) ?? '') : '';
				return item.filename.toLowerCase().includes(q) || projectName.toLowerCase().includes(q);
			})
			.sort((a, b) => timestampOf(b).getTime() - timestampOf(a).getTime())
	);

	const groupedMedia = $derived(groupByMonthAndDay(filteredMedia));

	onMount(async () => {
		await Promise.all([loadProjects(), loadMedia()]);
		loading = false;
	});

	async function handleSemanticSearch(e: SubmitEvent) {
		e.preventDefault();
		const q = semanticQuery.trim();
		if (!q) return;
		searchFocused = false;
		semanticLoading = true;
		semanticError = '';
		semanticHits = null;
		const res = await api.post('/media/search', { query: q, limit: 30 });
		semanticLoading = false;
		if (!res.ok) {
			const body = await res.json().catch(() => ({}));
			semanticError = body.error ?? 'Search failed.';
			return;
		}
		const data = await res.json();
		semanticHits = data.hits ?? [];
	}

	function clearSemanticSearch() {
		semanticQuery = '';
		semanticHits = null;
		semanticError = '';
	}

	async function loadProjects() {
		const res = await api.get('/projects');
		if (!res.ok) {
			projects = [];
			return;
		}
		projects = await res.json();
	}

	async function loadMedia() {
		const res = await api.get('/media');
		if (!res.ok) {
			media = [];
			return;
		}
		media = await res.json();
	}

	function timestampOf(item: MediaAsset) {
		return new Date(item.captured_at || item.created_at);
	}

	function toMonthKey(date: Date) {
		return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`;
	}

	function toDayKey(date: Date) {
		return `${toMonthKey(date)}-${String(date.getDate()).padStart(2, '0')}`;
	}

	function groupByMonthAndDay(items: MediaAsset[]): MonthGroup[] {
		const months: MonthGroup[] = [];
		for (const item of items) {
			const capturedAt = timestampOf(item);
			const monthKey = toMonthKey(capturedAt);
			const dayKey = toDayKey(capturedAt);

			let month = months.find((entry) => entry.monthKey === monthKey);
			if (!month) {
				month = {
					monthKey,
					monthLabel: monthFormatter.format(capturedAt),
					days: []
				};
				months.push(month);
			}

			let day = month.days.find((entry) => entry.dayKey === dayKey);
			if (!day) {
				day = {
					dayKey,
					dayLabel: dayFormatter.format(capturedAt),
					items: []
				};
				month.days.push(day);
			}
			day.items.push(item);
		}
		return months;
	}

	function openUploadModal() {
		uploadError = '';
		uploadFiles = [];
		uploadAsFolder = false;
		uploadProgress = { current: 0, total: 0 };
		uploadOpen = true;
	}

	function closeUploadModal() {
		if (uploadSubmitting) return;
		uploadOpen = false;
		uploadError = '';
		uploadFiles = [];
	}

	async function handleUpload(e: SubmitEvent) {
		e.preventDefault();
		if (uploadFiles.length === 0) {
			uploadError = 'Choose at least one file to upload.';
			return;
		}

		uploadSubmitting = true;
		uploadError = '';
		uploadProgress = { current: 0, total: uploadFiles.length };

		let hasError = false;
		for (const file of uploadFiles) {
			const formData = new FormData();
			formData.set('file', file);

			const res = await api.postForm('/media/upload', formData);
			if (!res.ok) {
				const body = await res.json().catch(() => ({}));
				uploadError = body.error ?? `Upload failed for ${file.name}.`;
				hasError = true;
				break;
			}
			uploadProgress.current += 1;
		}

		uploadSubmitting = false;
		await loadMedia();
		if (!hasError) {
			closeUploadModal();
		}
	}

	function toggleSelection(id: string) {
		if (selectedIds.includes(id)) {
			selectedIds = selectedIds.filter((itemID) => itemID !== id);
		} else {
			selectedIds = [...selectedIds, id];
		}
	}

	function mediaKind(item: MediaAsset) {
		const mime = (item.mime_type || '').toLowerCase();
		if (mime.startsWith('video/')) return 'video';
		if (mime.startsWith('audio/')) return 'audio';
		return 'image';
	}

	function projectLabel(item: MediaAsset) {
		if (!item.project_id) return 'Global media';
		return projectNameByID.get(item.project_id) || 'Scoped media';
	}

	async function handleBulkDelete() {
		if (!confirmBulkDelete) {
			confirmBulkDelete = true;
			return;
		}
		const ids = [...selectedIds];
		for (const id of ids) {
			await api.del(`/media-assets/${id}`);
		}
		media = media.filter((m) => !ids.includes(m.id));
		selectedIds = [];
		confirmBulkDelete = false;
	}

	async function handleClearAll() {
		if (!confirmClearAll) {
			confirmClearAll = true;
			return;
		}
		await api.post('/media/clear-all');
		media = [];
		selectedIds = [];
		confirmClearAll = false;
	}

	function handleAssetDeleted(id: string) {
		media = media.filter((m) => m.id !== id);
	}

	async function handleLogout() {
		await api.post('/logout');
		clear();
		goto('/login');
	}
</script>

<svelte:head><title>Media - Agentra</title></svelte:head>

<div class="min-h-screen bg-white text-slate-900">
	<!-- Fixed Header -->
	<header class="sticky top-0 z-50 bg-white border-b border-slate-200">
		<div class="px-6 md:px-10 py-3 flex items-center justify-between gap-6">
			<div class="min-w-max flex items-center gap-4">
				<h1 class="text-xl font-semibold tracking-tight">Media</h1>
				<button type="button" class="btn-primary text-xs py-1.5 flex items-center gap-2" onclick={openUploadModal}>
					<Upload size={14} />
					<span>Upload</span>
				</button>
				{#if confirmClearAll}
					<button type="button" class="btn-secondary text-xs py-1.5 flex items-center gap-2 bg-red-50 text-red-600 border-red-200 hover:bg-red-100" onclick={handleClearAll}>
						<Trash2 size={14} />
						<span>Confirm Clear All?</span>
					</button>
					<button type="button" class="p-1 text-slate-400 hover:text-slate-600" onclick={() => confirmClearAll = false}>
						<X size={14} />
					</button>
				{:else}
					<button type="button" class="btn-secondary text-xs py-1.5 flex items-center gap-2 text-slate-500 hover:text-red-600 border-transparent shadow-none hover:bg-slate-50" onclick={() => confirmClearAll = true}>
						<Trash2 size={14} />
						<span>Clear All</span>
					</button>
				{/if}
			</div>

			<div class="flex-1 max-w-xl relative">
				<form onsubmit={handleSemanticSearch}>
					<div class="relative group">
						<Sparkles
							class="absolute left-3.5 top-1/2 -translate-y-1/2 transition-colors {searchFocused || semanticQuery ? 'text-violet-500' : 'text-slate-400'}"
							size={15}
						/>
						<input
							type="text"
							bind:value={semanticQuery}
							onfocus={() => (searchFocused = true)}
							onblur={() => setTimeout(() => (searchFocused = false), 150)}
							placeholder="Try: &quot;{suggestions[suggestionIdx]}&quot;"
							class="w-full bg-slate-50 border rounded-xl py-2 pl-10 pr-10 transition-all outline-none text-sm text-slate-700 placeholder:text-slate-400
								{searchFocused || semanticQuery
									? 'border-violet-300 bg-white ring-4 ring-violet-100 shadow-[0_0_20px_rgba(139,92,246,0.12)]'
									: 'border-slate-200 hover:border-violet-200'}"
						/>
						{#if semanticQuery}
							<button
								type="button"
								onclick={clearSemanticSearch}
								class="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 transition-colors"
							>
								<X size={14} />
							</button>
						{/if}
					</div>
				</form>

				<!-- Suggestion chips on focus when query is empty -->
				{#if searchFocused && !semanticQuery}
					<div class="absolute top-full left-0 right-0 mt-2 z-50 bg-white border border-slate-200 rounded-xl shadow-xl p-3 flex flex-wrap gap-1.5 animate-in fade-in slide-in-from-top-1 duration-150">
						<p class="w-full text-[10px] font-bold text-slate-400 uppercase tracking-widest mb-1">Try searching for</p>
						{#each suggestions.slice(0, 6) as s}
							<button
								type="button"
								class="text-xs px-2.5 py-1 rounded-full bg-violet-50 text-violet-700 border border-violet-100 hover:bg-violet-100 transition-colors"
								onmousedown={() => { semanticQuery = s; searchFocused = false; }}
							>
								{s}
							</button>
						{/each}
					</div>
				{/if}
			</div>

			<div class="flex items-center gap-3 ml-auto">
				<div class="hidden md:block">
					<BackendActivityButton />
				</div>
				{#if auth.user}
					<div class="hidden lg:block text-right">
						<div class="text-[10px] text-slate-400 uppercase tracking-widest font-bold leading-none">Signed in</div>
						<div class="text-xs font-semibold text-slate-900">{auth.user.email}</div>
					</div>
					<button type="button" class="btn-secondary text-xs px-3 py-1.5" onclick={handleLogout}>Logout</button>
				{/if}
			</div>
		</div>
	</header>

	<main class="px-2 md:px-6 py-6">
		<!-- Selection Mode Bar -->
		{#if selectedIds.length > 0}
			<div class="fixed top-0 left-0 right-0 h-[61px] bg-white text-slate-900 z-[60] flex items-center px-10 justify-between border-b-2 border-blue-500 shadow-xl animate-in slide-in-from-top duration-200">
				<div class="flex items-center gap-6">
					<button onclick={() => { selectedIds = []; confirmBulkDelete = false; }} class="p-2 hover:bg-slate-100 rounded-full transition-colors">
						<X size={20} />
					</button>
					<span class="text-lg font-medium">{selectedIds.length} selected</span>
				</div>
				<div class="flex items-center gap-3">
					<button class="p-2 hover:bg-slate-100 rounded-full text-slate-600 transition-colors" title="Share"><Share2 size={18} /></button>
					<button class="p-2 hover:bg-slate-100 rounded-full text-slate-600 transition-colors" title="Download"><Download size={18} /></button>
					{#if confirmBulkDelete}
						<button onclick={handleBulkDelete} class="px-3 py-1.5 rounded-lg bg-red-600 text-white text-sm font-medium hover:bg-red-700 transition-colors">
							Delete {selectedIds.length} item{selectedIds.length === 1 ? '' : 's'}?
						</button>
						<button onclick={() => (confirmBulkDelete = false)} class="p-2 hover:bg-slate-100 rounded-full text-slate-600 transition-colors">
							<X size={18} />
						</button>
					{:else}
						<button onclick={handleBulkDelete} class="p-2 hover:bg-red-50 rounded-full text-slate-600 hover:text-red-600 transition-colors" title="Delete"><Trash2 size={18} /></button>
					{/if}
				</div>
			</div>
		{/if}

		{#if loading}
			<div class="py-20 text-center text-sm text-slate-400">Loading media library...</div>
		{:else if semanticLoading}
			<div class="py-24 flex flex-col items-center justify-center text-center">
				<Loader2 size={28} class="text-violet-400 animate-spin mb-3" />
				<p class="text-sm text-slate-500">Searching your media library...</p>
			</div>
		{:else if semanticError}
			<div class="py-16 text-center">
				<p class="text-sm text-red-500">{semanticError}</p>
				<button onclick={clearSemanticSearch} class="mt-3 text-xs text-slate-500 underline">Clear search</button>
			</div>
		{:else if semanticHits !== null}
			<EmbeddingSearchPanel hits={semanticHits} token={auth.token ?? ''} onSelectMedia={(id) => (detailId = id)} />
		{:else if groupedMedia.length === 0}
			<div class="py-32 flex flex-col items-center justify-center text-center">
				<div class="w-20 h-20 rounded-full bg-slate-50 flex items-center justify-center mb-6">
					<ImageIcon size={32} class="text-slate-200" />
				</div>
				<h3 class="text-lg font-semibold text-slate-800">No media yet</h3>
				<p class="text-slate-500 text-sm mt-1 max-w-xs">Start building your timeline by uploading your first media file.</p>
				<button type="button" class="btn-primary mt-6 text-sm" onclick={openUploadModal}>Upload your first file</button>
			</div>
		{:else}
			{#each groupedMedia as month}
				<section class="mb-10">
					<h2 class="text-lg font-semibold mb-4 px-4 text-slate-800 tracking-tight">{month.monthLabel}</h2>

					{#each month.days as day}
						<div class="mb-8">
							<div class="sticky top-[61px] z-40 bg-white/95 py-2 px-4 flex items-center justify-between backdrop-blur-sm">
								<h3 class="text-[11px] font-bold text-slate-400 uppercase tracking-widest">{day.dayLabel}</h3>
							</div>

							<div class="flex flex-wrap gap-1 px-1">
								{#each day.items as item}
									{@const kind = mediaKind(item)}
									{@const isSelected = selectedIds.includes(item.id)}
									<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
									<div
										class="group relative h-[180px] md:h-[240px] overflow-hidden transition-all duration-300 rounded-sm bg-slate-50 border border-slate-100 shrink-0 cursor-pointer"
										onclick={() => { detailId = item.id; }}
									>
										{#if item.thumbnail_path}
											<AuthenticatedImage
												src={`${api.BASE}/media/${item.id}/thumbnail`}
												token={auth.token ?? ''}
												alt={item.filename}
												className="h-full w-auto block transition-transform duration-500 group-hover:scale-105"
											/>
										{:else}
											<div class="h-full w-[180px] flex flex-col items-center justify-center text-slate-400 gap-2">
												{#if kind === 'video'}
													<Play size={24} />
												{:else if kind === 'audio'}
													<Music2 size={24} />
												{:else}
													<ImageIcon size={24} />
												{/if}
												<span class="text-[9px] uppercase font-bold tracking-tighter opacity-50">{kind}</span>
											</div>
										{/if}

										<!-- Selection Overlay -->
										{#if isSelected}
											<div class="absolute inset-0 bg-blue-500/10 border-[6px] border-blue-500 z-10"></div>
										{/if}

										<!-- Checkbox/Selection Button -->
										<button
											class="absolute top-3 left-3 z-20 transition-all duration-300 {isSelected ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'}"
											onclick={(e) => {
												e.stopPropagation();
												toggleSelection(item.id);
											}}
										>
											{#if isSelected}
												<div class="bg-white rounded-full text-blue-600 shadow-lg">
													<CheckCircle2 size={24} fill="currentColor" class="text-blue-600" />
												</div>
											{:else}
												<div class="w-6 h-6 rounded-full border-2 border-white/90 bg-black/10 backdrop-blur-sm shadow-sm"></div>
											{/if}
										</button>

										<!-- Video Tag -->
										{#if kind === 'video'}
											<div class="absolute bottom-3 right-3 z-20 flex items-center gap-1.5 px-2 py-0.5 rounded bg-black/50 backdrop-blur-sm text-white text-[10px] font-bold">
												<Play size={10} fill="currentColor" />
												<span>Video</span>
											</div>
										{/if}

										<!-- Info Overlay on Hover -->
										<div class="absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none">
											<div class="absolute bottom-3 left-4 text-white">
												<p class="text-[11px] font-semibold truncate max-w-[200px] shadow-sm">{item.filename}</p>
												<p class="text-[9px] text-white/80">{projectLabel(item)}</p>
											</div>
										</div>
									</div>
								{/each}
							</div>
						</div>
					{/each}
				</section>
			{/each}
		{/if}
	</main>
</div>

<MediaDetailPanel
	mediaId={detailId}
	token={auth.token ?? ''}
	onClose={() => { detailId = null; }}
	onDeleted={handleAssetDeleted}
/>

<Modal
	open={uploadOpen}
	title="Upload Media"
	description="Upload org-level media. Projects can scope what they see."
	widthClass="max-w-md"
	closeOnBackdrop={true}
	onClose={closeUploadModal}
>
	<form onsubmit={handleUpload} class="space-y-5">
		{#if uploadError}
			<div class="rounded-lg border border-red-200 bg-red-50 text-red-700 text-xs px-3 py-2.5 font-medium">{uploadError}</div>
		{/if}

		<div class="space-y-1.5">
			<label for="upload-file-multi" class="text-xs font-bold text-slate-500 uppercase tracking-wider block mb-1">Files</label>
			<div class="relative">
				<input
					id="upload-file-multi"
					type="file"
					multiple
					accept=".mov,.mp4,.jpg,.jpeg,.png,.wav,.mp3"
					class="input-base text-sm file:mr-4 file:py-1 file:px-3 file:rounded-md file:border-0 file:text-xs file:font-semibold file:bg-blue-50 file:text-blue-700 hover:file:bg-blue-100 cursor-pointer w-full"
					required
					onchange={(event) => {
						const target = event.currentTarget as HTMLInputElement;
						uploadFiles = Array.from(target.files || []);
					}}
				/>
			</div>
			<div class="flex justify-between items-center text-[10px]">
				<span class="text-slate-400 italic">Supported formats: MOV, MP4, JPG, PNG, WAV, MP3</span>
				{#if uploadFiles.length > 0}
					<span class="text-blue-600 font-semibold">{uploadFiles.length} file(s) selected</span>
				{/if}
			</div>
		</div>

		<div class="flex justify-end gap-2 pt-2">
			<button type="button" class="btn-secondary text-xs px-4 py-2" onclick={closeUploadModal} disabled={uploadSubmitting}>
				Cancel
			</button>
			<button type="submit" class="btn-primary text-xs px-4 py-2 inline-flex items-center gap-2" disabled={uploadSubmitting}>
				{#if uploadSubmitting}
					<div class="h-3 w-3 border-2 border-white/30 border-t-white rounded-full animate-spin"></div>
					<span>Uploading {uploadProgress.current}/{uploadProgress.total}...</span>
				{:else}
					<Upload size={14} />
					<span>Upload Media</span>
				{/if}
			</button>
		</div>
	</form>
</Modal>

<style>
	:global(body) {
		background-color: white;
	}
</style>
