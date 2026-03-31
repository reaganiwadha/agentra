<script lang="ts">
	let { src, token = '' }: { src: string; token?: string } = $props();

	let resolvedSrc = $state('');
	let loading = $state(false);
	let loadError = $state('');
	let currentObjectURL = '';
	let requestCounter = 0;

	function cleanupObjectURL() {
		if (currentObjectURL) {
			URL.revokeObjectURL(currentObjectURL);
			currentObjectURL = '';
		}
	}

	$effect(() => {
		const srcValue = src;
		const tokenValue = token;
		requestCounter += 1;
		const currentRequest = requestCounter;

		loadError = '';

		if (!srcValue) {
			loading = false;
			resolvedSrc = '';
			cleanupObjectURL();
			return;
		}

		if (!tokenValue) {
			loading = false;
			cleanupObjectURL();
			resolvedSrc = srcValue;
			return;
		}

		const controller = new AbortController();
		loading = true;
		resolvedSrc = '';
		cleanupObjectURL();

		(async () => {
			try {
				const res = await fetch(srcValue, {
					headers: { Authorization: `Bearer ${tokenValue}` },
					signal: controller.signal
				});
				if (!res.ok) {
					throw new Error(`Video request failed (${res.status})`);
				}
				const blob = await res.blob();
				if (currentRequest !== requestCounter) return;
				currentObjectURL = URL.createObjectURL(blob);
				resolvedSrc = currentObjectURL;
			} catch (err: any) {
				if (controller.signal.aborted) return;
				if (currentRequest !== requestCounter) return;
				loadError = err?.message ?? 'Failed to load video';
			} finally {
				if (currentRequest === requestCounter) {
					loading = false;
				}
			}
		})();

		return () => {
			controller.abort();
			if (currentRequest === requestCounter) {
				loading = false;
			}
			cleanupObjectURL();
		};
	});
</script>

<div class="surface-soft p-2">
	{#if loading}
		<div class="w-full rounded-xl bg-slate-100 text-slate-500 text-sm p-6 text-center">Loading video...</div>
	{:else if loadError}
		<div class="w-full rounded-xl bg-red-50 text-red-700 text-sm p-6 text-center">{loadError}</div>
	{:else if resolvedSrc}
		<video controls preload="metadata" class="w-full rounded-xl bg-slate-950">
			<source src={resolvedSrc} />
			Your browser does not support the video tag.
		</video>
	{:else}
		<div class="w-full rounded-xl bg-slate-100 text-slate-500 text-sm p-6 text-center">No video source available.</div>
	{/if}
</div>
