<script lang="ts">
	import { Terminal } from 'lucide-svelte';
	import { activity } from '$lib/activity.svelte';

	let open = $state(false);
	let terminalEl = $state<HTMLElement | null>(null);

	// Auto-scroll to bottom whenever logs change or terminal opens
	$effect(() => {
		// Track both logs and open so scroll fires on open too
		void activity.logs.length;
		void open;
		if (terminalEl) {
			terminalEl.scrollTop = terminalEl.scrollHeight;
		}
	});

	function levelColor(level: string) {
		switch ((level || '').toLowerCase()) {
			case 'error':
				return 'text-rose-400';
			case 'warn':
			case 'warning':
				return 'text-amber-400';
			default:
				return 'text-emerald-400';
		}
	}

	function logTime(ts: string) {
		if (!ts) return '--:--:--';
		const d = new Date(ts);
		if (isNaN(d.getTime())) return ts;
		return d.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
	}

	function transportLabel() {
		if (activity.transport === 'stream' && activity.connected) return '● live';
		if (activity.transport === 'poll' && activity.connected) return '⟳ poll';
		return '○ off';
	}
</script>

<div class="relative">
	<button
		type="button"
		class="flex items-center gap-2 rounded-full border border-slate-200 bg-white px-3 py-1.5 hover:bg-slate-50 transition-colors"
		onclick={() => (open = !open)}
	>
		<span
			class="h-2 w-2 rounded-full shrink-0 {activity.active
				? 'bg-emerald-500'
				: 'bg-slate-300'} {activity.active && activity.connected ? 'animate-pulse' : ''}"
		></span>
		<span class="text-xs text-slate-600 truncate max-w-[180px]">{activity.message}</span>
		<Terminal size={12} class="text-slate-400 shrink-0" />
	</button>

	{#if open}
		<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
		<div class="fixed inset-0 z-[100]" onclick={() => (open = false)}></div>

		<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
		<div
			class="absolute right-0 top-full z-[101] mt-2 w-[38rem] max-w-[92vw] rounded-xl border border-slate-700 bg-slate-950 shadow-2xl overflow-hidden"
			onclick={(e) => e.stopPropagation()}
		>
			<div class="flex items-center justify-between px-3 py-2 border-b border-slate-800">
				<div class="flex items-center gap-2.5">
					<span
						class="h-2 w-2 rounded-full {activity.active
							? 'bg-emerald-500 animate-pulse'
							: 'bg-slate-600'}"
					></span>
					<span class="text-[11px] font-mono font-semibold text-slate-300">backend activity</span>
					<span class="text-[10px] font-mono text-slate-600">{transportLabel()}</span>
				</div>
				<button
					type="button"
					class="text-[10px] font-mono text-slate-500 hover:text-slate-300 transition-colors px-1"
					onclick={() => (open = false)}>close</button
				>
			</div>

			<div class="max-h-96 overflow-y-auto" bind:this={terminalEl}>
				{#if activity.logs.length === 0}
					<div class="font-mono text-xs text-slate-600 text-center py-8">No activity yet.</div>
				{:else}
					<div class="p-2 space-y-0">
						{#each [...activity.logs].reverse() as log (log.id)}
							<div
								class="flex gap-2 items-baseline px-1.5 py-[3px] rounded hover:bg-slate-900/60 font-mono text-[11px] leading-relaxed"
							>
								<span class="text-slate-600 shrink-0 tabular-nums">{logTime(log.created_at)}</span>
								<span class="{levelColor(log.level)} shrink-0 w-[38px] text-right"
									>[{log.level.toUpperCase().slice(0, 4)}]</span
								>
								<span class="text-slate-300 break-words min-w-0">{log.message}</span>
							</div>
						{/each}
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>
