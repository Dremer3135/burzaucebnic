<script lang="ts">
	import { auth } from '$lib/stores.svelte';

	let { onsuccess } = $props<{ onsuccess?: () => void }>();

	let errorMessage = $state('');
	let isSubmitting = $state(false);

	async function handleGoogleLogin() {
		errorMessage = '';
		isSubmitting = true;
		try {
			await auth.loginWithGoogle();
			onsuccess?.();
		} catch (err: any) {
			console.error('Google login error:', err);
			errorMessage = err?.message || 'Chyba při přihlašování přes Google.';
		} finally {
			isSubmitting = false;
		}
	}

	async function quickLogin(targetEmail: string) {
		errorMessage = '';
		isSubmitting = true;
		try {
			await auth.loginWithPassword(targetEmail, 'heslo123');
			onsuccess?.();
		} catch (err: any) {
			errorMessage = err?.message || 'Chyba při rychlém přihlášení.';
		} finally {
			isSubmitting = false;
		}
	}
</script>

<div class="bg-white border-2 border-black p-6 sm:p-8 max-w-sm w-full text-black shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]">
	<!-- Header -->
	<div class="text-center mb-6">
		<h2 class="text-2xl font-black uppercase tracking-tight text-black mb-1">
			BURZA UČEBNIC
		</h2>
		<p class="text-xs font-bold text-neutral-600 uppercase">
			Přihlaste se školním Google účtem
		</p>
	</div>

	{#if errorMessage}
		<div class="mb-4 p-3 bg-red-50 border-2 border-red-600 text-red-700 text-xs font-bold">
			{errorMessage}
		</div>
	{/if}

	<!-- Google Button (Sole Login Method) -->
	<button
		type="button"
		onclick={handleGoogleLogin}
		disabled={isSubmitting}
		class="w-full flex items-center justify-center gap-2.5 py-3.5 px-4 bg-black text-white font-black text-xs uppercase tracking-wider border-2 border-black hover:bg-neutral-800 active:scale-98 transition-all disabled:opacity-50 cursor-pointer"
	>
		<svg class="w-4 h-4 shrink-0" viewBox="0 0 24 24">
			<path
				fill="#4285F4"
				d="M23.745 12.27c0-.7-.06-1.4-.19-2.07H12v4.51h6.6c-.29 1.52-1.14 2.8-2.4 3.66v3.05h3.88c2.27-2.09 3.665-5.17 3.665-9.15z"
			/>
			<path
				fill="#34A853"
				d="M12 24c3.24 0 5.95-1.08 7.93-2.91l-3.88-3.05c-1.08.72-2.45 1.16-4.05 1.16-3.12 0-5.77-2.1-6.72-4.93H1.25v3.15C3.26 21.36 7.34 24 12 24z"
			/>
			<path
				fill="#FBBC05"
				d="M5.28 14.27c-.25-.72-.38-1.49-.38-2.27s.13-1.55.38-2.27V6.58H1.25C.45 8.18 0 10.03 0 12s.45 3.82 1.25 5.42l4.03-3.15z"
			/>
			<path
				fill="#EA4335"
				d="M12 4.75c1.77 0 3.35.61 4.6 1.8l3.42-3.42C17.95 1.19 15.24 0 12 0 7.34 0 3.26 2.64 1.25 6.58l4.03 3.15c.95-2.83 3.6-4.98 6.72-4.98z"
			/>
		</svg>
		<span>{isSubmitting ? 'PŘIHLAŠUJI...' : 'PŘIHLÁSIT SE ŠKOLNÍM ÚČTEM'}</span>
	</button>

	<!-- Quick Dev Accounts (Visible only in local development) -->
	{#if import.meta.env.DEV}
		<div class="mt-6 pt-4 border-t-2 border-dashed border-neutral-300">
			<div class="text-[10px] font-mono font-bold text-neutral-400 uppercase text-center mb-2">
				[DEV RYCHLÉ PŘIHLÁŠENÍ]
			</div>
			<div class="grid grid-cols-3 gap-1.5">
				<button
					type="button"
					onclick={() => quickLogin('seller@burza.cz')}
					class="py-1.5 px-1 bg-neutral-100 hover:bg-black hover:text-white border border-black text-black font-black text-[10px] uppercase transition-colors text-center cursor-pointer truncate"
				>
					PRODEJCE
				</button>
				<button
					type="button"
					onclick={() => quickLogin('buyer@burza.cz')}
					class="py-1.5 px-1 bg-neutral-100 hover:bg-black hover:text-white border border-black text-black font-black text-[10px] uppercase transition-colors text-center cursor-pointer truncate"
				>
					KUPUJÍCÍ
				</button>
				<button
					type="button"
					onclick={() => quickLogin('cashier@burza.cz')}
					class="py-1.5 px-1 bg-neutral-100 hover:bg-black hover:text-white border border-black text-black font-black text-[10px] uppercase transition-colors text-center cursor-pointer truncate"
				>
					POKLADNÍ
				</button>
			</div>
		</div>
	{/if}
</div>
