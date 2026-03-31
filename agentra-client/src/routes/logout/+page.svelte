<script lang="ts">
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { clear } from '$lib/auth.svelte';
	import { onMount } from 'svelte';

	onMount(async () => {
		await api.post('/logout');
		clear();
		goto('/login');
	});
</script>
