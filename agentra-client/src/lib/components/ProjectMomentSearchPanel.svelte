<script lang="ts">
	import { Clock3, FileText, Sparkles } from 'lucide-svelte';
	import { api } from '$lib/api';
	import AuthenticatedImage from './AuthenticatedImage.svelte';

	type ProjectMoment = {
		media_id: string;
		filename: string;
		storage_path: string;
		start_sec: number;
		end_sec: number;
		score: number;
		matched_text: string;
		context_text: string;
		segment_indexes: number[];
		duration_sec?: number | null;
		captured_at?: string | null;
	};

	let {
		moments,
		token,
		onSelectMedia
	}: {
		moments: ProjectMoment[];
		token: string;
		onSelectMedia: (mediaId: string) => void;
	} = $props();

	function formatTimestamp(sec: number): string {
		const total = Math.max(0, Math.floor(sec));
		const h = Math.floor(total / 3600);
		const m = Math.floor((total % 3600) / 60);
		const s = total % 60;
		if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
		return `${m}:${String(s).padStart(2, '0')}`;
	}

	function scoreColor(score: number): string {
		if (score >= 0.85) return 'bg-emerald-100 text-emerald-700 border-emerald-200';
		if (score >= 0.7) return 'bg-sky-100 text-sky-700 border-sky-200';
		if (score >= 0.5) return 'bg-amber-100 text-amber-700 border-amber-200';
		return 'bg-slate-100 text-slate-500 border-slate-200';
	}
</script>

{#if moments.length === 0}
	<div class="py-20 flex flex-col items-center justify-center text-center">
		<Sparkles size={28} class="text-slate-300 mb-3" />
		<p class="text-sm text-slate-500">No project moments matched yet.</p>
		<p class="text-xs text-slate-400 mt-1">Try a clearer description, or wait until the scoped assets finish analysis.</p>
	</div>
{:else}
	<div class="px-6 pt-5 pb-2">
		<p class="text-xs text-slate-400 font-medium">
			{moments.length} moment{moments.length === 1 ? '' : 's'} ranked for this project
		</p>
	</div>
	<div class="divide-y divide-slate-100 px-2 pb-3">
		{#each moments as moment, i}
			<button
				type="button"
				class="w-full flex gap-4 px-4 py-4 rounded-2xl hover:bg-slate-50 cursor-pointer transition-colors text-left"
				onclick={() => onSelectMedia(moment.media_id)}
			>
				<div class="flex-shrink-0 flex flex-col items-center gap-2 pt-0.5">
					<span class="text-[10px] font-bold text-slate-300 w-5 text-center">#{i + 1}</span>
					<div class="w-24 h-16 rounded-xl overflow-hidden bg-slate-100 flex-shrink-0 shadow-sm">
						<AuthenticatedImage
							src={`${api.BASE}/media-assets/${moment.media_id}/thumbnail`}
							{token}
							alt={moment.filename}
							className="w-full h-full object-cover"
						/>
					</div>
				</div>

				<div class="flex-1 min-w-0 py-0.5">
					<div class="flex items-start justify-between gap-3">
						<div class="min-w-0">
							<p class="text-sm font-semibold text-slate-800 truncate leading-snug">{moment.filename}</p>
							<div class="flex items-center gap-1.5 mt-1 text-[11px] text-sky-700 font-semibold">
								<Clock3 size={10} />
								<span>{formatTimestamp(moment.start_sec)} - {formatTimestamp(moment.end_sec)}</span>
								<span class="text-slate-300 font-normal">window</span>
							</div>
						</div>
						<span class="flex-shrink-0 text-[10px] font-bold px-2 py-0.5 rounded-full border {scoreColor(moment.score)}">
							{(moment.score * 100).toFixed(0)}%
						</span>
					</div>

					{#if moment.matched_text}
						<div class="mt-2.5">
							<div class="text-[10px] font-bold uppercase tracking-[0.16em] text-slate-400 mb-1">Matched</div>
							<p class="text-[11px] text-slate-700 leading-relaxed line-clamp-2">{moment.matched_text}</p>
						</div>
					{/if}

					{#if moment.context_text}
						<div class="flex items-start gap-1.5 mt-2">
							<FileText size={10} class="text-slate-300 mt-[3px] flex-shrink-0" />
							<p class="text-[11px] text-slate-500 leading-relaxed line-clamp-3">{moment.context_text}</p>
						</div>
					{/if}
				</div>
			</button>
		{/each}
	</div>
{/if}
