<script lang="ts">
	import '../app.css';
	import Header from '$lib/components/Header.svelte';
	import { auth, eventStore } from '$lib/stores.svelte';

	let { children } = $props();
</script>

<div class="min-h-screen bg-slate-900 text-slate-100 flex flex-col antialiased">
	<Header />
	<main class="flex-1 flex flex-col">
		{@render children()}
	</main>
</div>
