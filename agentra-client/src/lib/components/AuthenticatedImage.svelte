<script lang="ts">
	import mediumZoom from 'medium-zoom';

	let {
		src,
		token = '',
		alt = '',
		className = '',
		zoomable = false
	}: {
		src: string;
		token?: string;
		alt?: string;
		className?: string;
		zoomable?: boolean;
	} = $props();

	let resolvedSrc = $state('');
	let hidden = $state(false);
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

		hidden = false;

		if (!srcValue) {
			resolvedSrc = '';
			cleanupObjectURL();
			hidden = true;
			return;
		}

		if (!tokenValue) {
			resolvedSrc = srcValue;
			cleanupObjectURL();
			return;
		}

		const controller = new AbortController();
		resolvedSrc = '';
		cleanupObjectURL();

		(async () => {
			try {
				const res = await fetch(srcValue, {
					headers: { Authorization: `Bearer ${tokenValue}` },
					signal: controller.signal
				});
				if (!res.ok) throw new Error(`Image request failed (${res.status})`);
				const blob = await res.blob();
				if (controller.signal.aborted || currentRequest !== requestCounter) return;
				currentObjectURL = URL.createObjectURL(blob);
				resolvedSrc = currentObjectURL;
			} catch {
				if (controller.signal.aborted || currentRequest !== requestCounter) return;
				hidden = true;
			}
		})();

		return () => {
			controller.abort();
			cleanupObjectURL();
		};
	});

	function zoom(node: HTMLImageElement) {
		const mz = mediumZoom(node, { background: 'rgba(0,0,0,0.9)', margin: 24 });
		return {
			destroy() {
				mz.detach(node);
			}
		};
	}
</script>

{#if !hidden && resolvedSrc}
	{#if zoomable}
		<img src={resolvedSrc} alt={alt} class="{className} cursor-zoom-in" use:zoom />
	{:else}
		<img src={resolvedSrc} alt={alt} class={className} />
	{/if}
{/if}
