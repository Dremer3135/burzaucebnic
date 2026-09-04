<script lang="ts">
	import '../app.css';
	import Header from '$lib/components/Header.svelte';
	import { auth, eventStore } from '$lib/stores.svelte';

	let { children } = $props();
</script>

<div class="h-dvh h-[100dvh] w-full bg-white text-black flex flex-col overflow-hidden antialiased selection:bg-black selection:text-white">
	<Header />
	<main class="flex-1 w-full overflow-hidden relative flex flex-col">
		{@render children()}
	</main>
</div>
