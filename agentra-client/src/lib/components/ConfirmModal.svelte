<script lang="ts">
	import Modal from './Modal.svelte';

	export let open = false;
	export let title = 'Confirm action';
	export let message = 'Are you sure you want to continue?';
	export let confirmText = 'Confirm';
	export let cancelText = 'Cancel';
	export let danger = false;
	export let pending = false;
	export let onCancel: () => void = () => {};
	export let onConfirm: () => void = () => {};
</script>

<Modal {open} {title} onClose={onCancel} widthClass="max-w-md">
	<div class="space-y-4">
		<p class="text-sm text-slate-600">{message}</p>
		<div class="flex justify-end gap-2">
			<button type="button" class="btn-secondary text-sm" on:click={onCancel} disabled={pending}>{cancelText}</button>
			<button
				type="button"
				class={`text-sm inline-flex items-center gap-2 disabled:opacity-60 disabled:cursor-not-allowed ${danger ? 'rounded-xl bg-rose-600 text-white font-semibold px-4 py-2 hover:bg-rose-700' : 'btn-primary'}`}
				disabled={pending}
				on:click={onConfirm}
			>
				{#if pending}
					<svg viewBox="0 0 24 24" class="h-4 w-4 animate-spin" fill="none" stroke="currentColor" stroke-width="2">
						<path d="M21 12a9 9 0 1 1-6.2-8.56" />
					</svg>
					Processing...
				{:else}
					{confirmText}
				{/if}
			</button>
		</div>
	</div>
</Modal>
