<script lang="ts">
	import '../app.css';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import Header from '$lib/components/Header.svelte';
	import OnboardingModal from '$lib/components/OnboardingModal.svelte';
	import { auth, eventStore } from '$lib/stores.svelte';

	let { children } = $props();

	let showOnboarding = $derived(!!auth.user && !auth.user.onboardingComplete);

	$effect(() => {
		if (auth.user && !auth.isCashier) {
			if (eventStore.isMaintenance() && $page.url.pathname !== '/maintenance') {
				goto('/maintenance');
			} else if (!eventStore.isMaintenance() && $page.url.pathname === '/maintenance') {
				goto('/');
			}
		}
	});
</script>

<div class="h-dvh h-[100dvh] w-full bg-white text-black flex flex-col overflow-hidden antialiased selection:bg-black selection:text-white">
	<div class="contents" inert={showOnboarding ? true : undefined} aria-hidden={showOnboarding ? 'true' : undefined}>
		<Header />
		<main class="flex-1 w-full overflow-hidden relative flex flex-col">
			{@render children()}
		</main>
	</div>
	{#if showOnboarding}
		<OnboardingModal />
	{/if}
</div>

