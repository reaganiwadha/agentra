<script lang="ts">
	export let open = false;
	export let title = '';
	export let description = '';
	export let widthClass = 'max-w-xl';
	export let closeOnBackdrop = false;
	export let onClose: () => void = () => {};

	function handleBackdropClick() {
		if (closeOnBackdrop) onClose();
	}
</script>

{#if open}
	<button type="button" class="fixed inset-0 z-40 bg-slate-900/45" aria-label="Close modal backdrop" on:click={handleBackdropClick} disabled={!closeOnBackdrop}></button>
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4">
		<div class={`w-full ${widthClass} rounded-xl border border-slate-200 bg-white shadow-xl`} role="dialog" aria-modal="true" aria-labelledby="modal-title">
			<div class="px-5 py-4 border-b border-slate-200 flex items-center justify-between gap-3">
				<div>
					<h3 id="modal-title" class="text-lg font-semibold">{title}</h3>
					{#if description}
						<p class="text-xs text-slate-500 mt-0.5">{description}</p>
					{/if}
				</div>
				<button type="button" class="h-8 w-8 inline-flex items-center justify-center rounded-md border border-slate-200 text-slate-500 hover:bg-slate-100 hover:text-slate-700" aria-label="Close modal" on:click={onClose}>
					<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2">
						<path d="M18 6L6 18M6 6l12 12" />
					</svg>
				</button>
			</div>
			<div class="p-5">
				<slot />
			</div>
		</div>
	</div>
{/if}
