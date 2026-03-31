<script lang="ts">
	import { Clock, FileText, Sparkles } from 'lucide-svelte';
	import { api } from '$lib/api';
	import AuthenticatedImage from './AuthenticatedImage.svelte';

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

	let {
		hits,
		token,
		onSelectMedia
	}: {
		hits: SearchHit[];
		token: string;
		onSelectMedia: (mediaId: string) => void;
	} = $props();

	function formatTimestamp(sec: number): string {
		const m = Math.floor(sec / 60);
		const s = Math.floor(sec % 60);
		return `${m}:${String(s).padStart(2, '0')}`;
	}

	function scoreColor(score: number): string {
		if (score >= 0.85) return 'bg-emerald-100 text-emerald-700 border-emerald-200';
		if (score >= 0.7) return 'bg-violet-100 text-violet-700 border-violet-200';
		if (score >= 0.5) return 'bg-amber-100 text-amber-700 border-amber-200';
		return 'bg-slate-100 text-slate-500 border-slate-200';
	}
</script>

{#if hits.length === 0}
	<div class="py-24 flex flex-col items-center justify-center text-center">
		<Sparkles size={28} class="text-slate-300 mb-3" />
		<p class="text-sm text-slate-500">No matching moments found.</p>
		<p class="text-xs text-slate-400 mt-1">Try a different description, or check that media has been fully analyzed.</p>
	</div>
{:else}
	<div class="px-6 pt-4 pb-2">
		<p class="text-xs text-slate-400 font-medium">{hits.length} result{hits.length === 1 ? '' : 's'} — ranked by semantic similarity</p>
	</div>
	<div class="divide-y divide-slate-100 px-2">
		{#each hits as hit, i}
			<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
			<div
				class="flex gap-4 px-4 py-3.5 rounded-xl hover:bg-slate-50 cursor-pointer transition-colors"
				onclick={() => onSelectMedia(hit.media_id)}
			>
				<!-- Rank -->
				<div class="flex-shrink-0 flex flex-col items-center gap-2 pt-0.5">
					<span class="text-[10px] font-bold text-slate-300 w-5 text-center">#{i + 1}</span>
					<div class="w-20 h-14 rounded-lg overflow-hidden bg-slate-100 flex-shrink-0 shadow-sm">
						<AuthenticatedImage
							src={`${api.BASE}/media-assets/${hit.media_id}/thumbnail`}
							{token}
							alt={hit.filename}
							className="w-full h-full object-cover"
						/>
					</div>
				</div>

				<!-- Info -->
				<div class="flex-1 min-w-0 py-0.5">
					<div class="flex items-start justify-between gap-3">
						<p class="text-sm font-semibold text-slate-800 truncate leading-snug">{hit.filename}</p>
						<span class="flex-shrink-0 text-[10px] font-bold px-2 py-0.5 rounded-full border {scoreColor(hit.score)}">
							{(hit.score * 100).toFixed(0)}%
						</span>
					</div>

					{#if hit.start_sec != null && hit.end_sec != null}
						<div class="flex items-center gap-1.5 mt-1 text-[11px] text-violet-600 font-semibold">
							<Clock size={10} />
							<span>{formatTimestamp(hit.start_sec)} – {formatTimestamp(hit.end_sec)}</span>
							<span class="text-slate-300 font-normal">· segment</span>
						</div>
					{:else}
						<p class="text-[10px] text-slate-400 mt-0.5 font-medium">Whole asset</p>
					{/if}

					<div class="flex items-start gap-1.5 mt-1.5">
						<FileText size={10} class="text-slate-300 mt-[3px] flex-shrink-0" />
						<p class="text-[11px] text-slate-500 leading-relaxed line-clamp-2">{hit.source_text}</p>
					</div>
				</div>
			</div>
		{/each}
	</div>
{/if}
