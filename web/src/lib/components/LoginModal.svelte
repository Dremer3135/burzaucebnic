<script lang="ts">
	import { auth } from '$lib/stores.svelte';
	import { LogIn, Key, Mail, User, ShieldCheck } from '@lucide/svelte';

	let { onsuccess } = $props<{ onsuccess?: () => void }>();

	let email = $state('');
	let password = $state('');
	let isRegister = $state(false);
	let name = $state('');
	let errorMessage = $state('');
	let isSubmitting = $state(false);

	async function handlePasswordAuth(e: SubmitEvent) {
		e.preventDefault();
		errorMessage = '';
		isSubmitting = true;
		try {
			if (isRegister) {
				const { pb } = await import('$lib/pocketbase');
				await pb.collection('users').create({
					email,
					password,
					passwordConfirm: password,
					name
				});
			}
			await auth.loginWithPassword(email, password);
			onsuccess?.();
		} catch (err: any) {
			console.error(err);
			errorMessage = err?.message || 'Chyba při přihlášení. Zkontrolujte údaje.';
		} finally {
			isSubmitting = false;
		}
	}

	async function handleGoogleLogin() {
		errorMessage = '';
		isSubmitting = true;
		try {
			await auth.loginWithGoogle();
			onsuccess?.();
		} catch (err: any) {
			console.error(err);
			errorMessage = err?.message || 'Chyba při přihlašování přes Google.';
		} finally {
			isSubmitting = false;
		}
	}

	async function quickLogin(targetEmail: string) {
		email = targetEmail;
		password = 'heslo123';
		errorMessage = '';
		isSubmitting = true;
		try {
			await auth.loginWithPassword(email, password);
			onsuccess?.();
		} catch (err: any) {
			errorMessage = err?.message || 'Chyba při rychlém přihlášení.';
		} finally {
			isSubmitting = false;
		}
	}
</script>

<div class="bg-slate-800/95 border border-slate-700/80 rounded-2xl p-6 shadow-2xl max-w-sm w-full backdrop-blur">
	<div class="text-center mb-6">
		<div class="inline-flex p-3 bg-emerald-500/10 text-emerald-400 rounded-2xl mb-3 border border-emerald-500/20">
			<LogIn class="w-8 h-8" />
		</div>
		<h2 class="text-xl font-bold text-white tracking-tight">
			{isRegister ? 'Vytvořit účet' : 'Přihlášení do Burzy'}
		</h2>
		<p class="text-xs text-slate-400 mt-1">
			Pro nákup i prodej učebnic se prosím přihlaste
		</p>
	</div>

	{#if errorMessage}
		<div class="mb-4 p-3 bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs rounded-xl">
			{errorMessage}
		</div>
	{/if}

	<!-- Google OAuth button -->
	<button
		onclick={handleGoogleLogin}
		disabled={isSubmitting}
		class="w-full flex items-center justify-center gap-3 py-2.5 px-4 rounded-xl bg-white text-slate-800 font-medium hover:bg-slate-100 transition-colors shadow-sm disabled:opacity-50 text-sm mb-4 cursor-pointer"
	>
		<svg class="w-4 h-4" viewBox="0 0 24 24">
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
		<span>Přihlásit se přes Google</span>
	</button>

	<div class="relative flex py-2 items-center mb-4">
		<div class="flex-grow border-t border-slate-700"></div>
		<span class="flex-shrink mx-3 text-xs text-slate-500 uppercase tracking-wider font-semibold">nebo emailem</span>
		<div class="flex-grow border-t border-slate-700"></div>
	</div>

	<!-- Email / Password form -->
	<form onsubmit={handlePasswordAuth} class="space-y-3">
		{#if isRegister}
			<div>
				<label for="login-name" class="block text-xs font-medium text-slate-300 mb-1">Jméno</label>
				<div class="relative">
					<User class="w-4 h-4 text-slate-400 absolute left-3 top-3" />
					<input
						id="login-name"
						type="text"
						bind:value={name}
						required
						placeholder="Jan Novák"
						class="w-full bg-slate-900 border border-slate-700 rounded-xl pl-9 pr-3 py-2 text-sm text-white focus:outline-none focus:border-emerald-500 transition-colors"
					/>
				</div>
			</div>
		{/if}

		<div>
			<label for="login-email" class="block text-xs font-medium text-slate-300 mb-1">E-mail</label>
			<div class="relative">
				<Mail class="w-4 h-4 text-slate-400 absolute left-3 top-3" />
				<input
					id="login-email"
					type="email"
					bind:value={email}
					required
					placeholder="student@skola.cz"
					class="w-full bg-slate-900 border border-slate-700 rounded-xl pl-9 pr-3 py-2 text-sm text-white focus:outline-none focus:border-emerald-500 transition-colors"
				/>
			</div>
		</div>

		<div>
			<label for="login-password" class="block text-xs font-medium text-slate-300 mb-1">Heslo</label>
			<div class="relative">
				<Key class="w-4 h-4 text-slate-400 absolute left-3 top-3" />
				<input
					id="login-password"
					type="password"
					bind:value={password}
					required
					placeholder="••••••••"
					class="w-full bg-slate-900 border border-slate-700 rounded-xl pl-9 pr-3 py-2 text-sm text-white focus:outline-none focus:border-emerald-500 transition-colors"
				/>
			</div>
		</div>

		<button
			type="submit"
			disabled={isSubmitting}
			class="w-full py-2.5 px-4 rounded-xl bg-emerald-600 text-white font-medium hover:bg-emerald-500 transition-colors shadow-sm disabled:opacity-50 text-sm cursor-pointer"
		>
			{isSubmitting ? 'Zpracovávám...' : isRegister ? 'Zaregistrovat se' : 'Přihlásit se'}
		</button>
	</form>

	<div class="mt-4 text-center">
		<button
			onclick={() => {
				isRegister = !isRegister;
				errorMessage = '';
			}}
			class="text-xs text-slate-400 hover:text-emerald-400 transition-colors cursor-pointer"
		>
			{isRegister ? 'Máte již účet? Přihlaste se' : 'Nemáte účet? Zaregistrujte se'}
		</button>
	</div>

	<!-- Quick Dev Accounts for Mobile Testing -->
	<div class="mt-6 pt-4 border-t border-slate-700/60">
		<p class="text-[11px] text-slate-400 font-medium mb-2 text-center uppercase tracking-wider">
			Rychlé testovací účty:
		</p>
		<div class="grid grid-cols-3 gap-1.5 text-xs">
			<button
				type="button"
				onclick={() => quickLogin('seller@burza.cz')}
				class="py-1.5 px-2 bg-slate-700/50 hover:bg-emerald-600/30 hover:border-emerald-500/50 border border-slate-600 rounded-lg text-slate-200 transition-colors text-center cursor-pointer truncate"
			>
				Prodejce
			</button>
			<button
				type="button"
				onclick={() => quickLogin('buyer@burza.cz')}
				class="py-1.5 px-2 bg-slate-700/50 hover:bg-emerald-600/30 hover:border-emerald-500/50 border border-slate-600 rounded-lg text-slate-200 transition-colors text-center cursor-pointer truncate"
			>
				Kupující
			</button>
			<button
				type="button"
				onclick={() => quickLogin('cashier@burza.cz')}
				class="py-1.5 px-2 bg-slate-700/50 hover:bg-amber-600/30 hover:border-amber-500/50 border border-slate-600 rounded-lg text-amber-300 transition-colors text-center cursor-pointer truncate"
			>
				Pokladní
			</button>
		</div>
	</div>
</div>
