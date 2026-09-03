<script lang="ts">
	import '../app.css';
	import Header from '$lib/components/Header.svelte';
	import { auth, eventStore } from '$lib/stores.svelte';

	let { children } = $props();
</script>

<div class="min-h-screen bg-white text-black flex flex-col antialiased selection:bg-black selection:text-white">
	<Header />
	<main class="flex-1 flex flex-col w-full">
		{@render children()}
	</main>
</div>
