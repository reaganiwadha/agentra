<script lang="ts">
	let { status }: { status: string } = $props();

	const styles: Record<string, { badge: string; dot: string }> = {
		pending: { badge: 'bg-slate-100 text-slate-600 border-slate-200', dot: 'bg-slate-400' },
		ready: { badge: 'bg-emerald-50 text-emerald-700 border-emerald-200', dot: 'bg-emerald-500' },
		queued: { badge: 'bg-slate-100 text-slate-600 border-slate-200', dot: 'bg-slate-400' },
		running: { badge: 'bg-amber-50 text-amber-700 border-amber-200', dot: 'bg-amber-500' },
		completed: { badge: 'bg-emerald-50 text-emerald-700 border-emerald-200', dot: 'bg-emerald-500' }
	};

	const style = styles[status] ?? { badge: 'bg-slate-100 text-slate-600 border-slate-200', dot: 'bg-slate-400' };
	const label = status.replace('_', ' ');
</script>

<span class={`inline-flex items-center gap-2 px-2.5 py-1 rounded-full border text-xs font-semibold ${style.badge}`}>
	<span class={`h-2 w-2 rounded-full ${style.dot}`}></span>
	<span class="capitalize">{label}</span>
</span>
