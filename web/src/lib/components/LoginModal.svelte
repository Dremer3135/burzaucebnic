<script lang="ts">
	import { auth } from '$lib/stores.svelte';
	import { LogIn, Key, Mail, User, ShieldCheck } from '@lucide/svelte';

	let { onsuccess } = $props<{ onsuccess?: () => void }>();

	let email = $state('');
	let password = $state('');
	let passwordConfirm = $state('');
	let isRegister = $state(false);
	let name = $state('');
	let errorMessage = $state('');
	let isSubmitting = $state(false);

	async function handlePasswordAuth(e: SubmitEvent) {
		e.preventDefault();
		errorMessage = '';

		if (isRegister) {
			if (password.length < 8) {
				errorMessage = 'Heslo musí mít alespoň 8 znaků.';
				return;
			}
			if (password !== passwordConfirm) {
				errorMessage = 'Hesla se neshodují.';
				return;
			}
		}

		isSubmitting = true;
		try {
			if (isRegister) {
				const { pb } = await import('$lib/pocketbase');
				await pb.collection('users').create({
					email: email.trim(),
					password,
					passwordConfirm: passwordConfirm,
					name: name.trim()
				});
			}
			await auth.loginWithPassword(email.trim(), password);
			onsuccess?.();
		} catch (err: any) {
			console.error(err);
			const errData = err?.response?.data || err?.data;
			if (errData?.password?.message) {
				errorMessage = 'Heslo musí mít alespoň 8 znaků.';
			} else if (errData?.email?.message) {
				errorMessage = 'Tento e-mail již existuje nebo je neplatný.';
			} else if (errData?.passwordConfirm?.message) {
				errorMessage = 'Hesla se neshodují.';
			} else {
				errorMessage = err?.message || 'Chyba při zpracování. Zkontrolujte zadané údaje.';
			}
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

<div class="bg-white border-2 border-black p-6 max-w-sm w-full text-black">
	<!-- Header -->
	<div class="text-center mb-6">
		<h2 class="text-2xl font-black uppercase tracking-tight text-black">
			{isRegister ? 'NOVÝ ÚČET' : 'PŘIHLÁŠENÍ'}
		</h2>
	</div>

	{#if errorMessage}
		<div class="mb-4 p-3 bg-red-50 border-2 border-red-600 text-red-700 text-xs font-bold">
			{errorMessage}
		</div>
	{/if}

	<!-- Google Button -->
	<button
		type="button"
		onclick={handleGoogleLogin}
		disabled={isSubmitting}
		class="w-full flex items-center justify-center gap-2 py-3 px-4 bg-white text-black font-black text-xs uppercase border-2 border-black hover:bg-neutral-100 transition-colors disabled:opacity-50 mb-4 cursor-pointer"
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
		<span>PŘIHLÁSIT PŘES GOOGLE</span>
	</button>

	<div class="relative flex py-2 items-center mb-4">
		<div class="flex-grow border-t-2 border-black"></div>
		<span class="flex-shrink mx-3 text-[11px] text-black uppercase font-black tracking-wider">NEBO</span>
		<div class="flex-grow border-t-2 border-black"></div>
	</div>

	<!-- Email / Password Form -->
	<form onsubmit={handlePasswordAuth} class="space-y-3">
		{#if isRegister}
			<div>
				<label for="login-name" class="block text-xs font-black uppercase text-black mb-1">Jméno</label>
				<input
					id="login-name"
					type="text"
					bind:value={name}
					required
					placeholder="Jan Novák"
					class="w-full bg-white border-2 border-black px-3 py-2.5 text-sm text-black font-semibold focus:outline-none focus:bg-neutral-50"
				/>
			</div>
		{/if}

		<div>
			<label for="login-email" class="block text-xs font-black uppercase text-black mb-1">E-mail</label>
			<input
				id="login-email"
				type="email"
				bind:value={email}
				required
				placeholder="student@skola.cz"
				class="w-full bg-white border-2 border-black px-3 py-2.5 text-sm text-black font-semibold focus:outline-none focus:bg-neutral-50"
			/>
		</div>

		<div>
			<label for="login-password" class="block text-xs font-black uppercase text-black mb-1">Heslo</label>
			<input
				id="login-password"
				type="password"
				bind:value={password}
				required
				minlength="8"
				placeholder="Minimálně 8 znaků"
				class="w-full bg-white border-2 border-black px-3 py-2.5 text-sm text-black font-semibold focus:outline-none focus:bg-neutral-50"
			/>
		</div>

		{#if isRegister}
			<div>
				<label for="login-password-confirm" class="block text-xs font-black uppercase text-black mb-1">Potvrzení hesla</label>
				<input
					id="login-password-confirm"
					type="password"
					bind:value={passwordConfirm}
					required
					minlength="8"
					placeholder="Zadejte heslo znovu"
					class="w-full bg-white border-2 border-black px-3 py-2.5 text-sm text-black font-semibold focus:outline-none focus:bg-neutral-50"
				/>
			</div>
		{/if}

		<button
			type="submit"
			disabled={isSubmitting}
			class="w-full py-3.5 px-4 bg-black text-white font-black text-sm uppercase tracking-wider border-2 border-black hover:bg-neutral-800 active:bg-neutral-900 transition-colors disabled:opacity-50 cursor-pointer"
		>
			{isSubmitting ? 'ČEKEJTE...' : isRegister ? 'ZAREGISTROVAT SE' : 'PŘIHLÁSIT SE'}
		</button>
	</form>

	<!-- Toggle Login / Register -->
	<div class="mt-4 text-center">
		<button
			type="button"
			onclick={() => {
				isRegister = !isRegister;
				errorMessage = '';
			}}
			class="text-xs font-black uppercase underline hover:no-underline cursor-pointer"
		>
			{isRegister ? 'MÁTE ÚČET? PŘIHLÁSIT SE' : 'NEMÁTE ÚČET? VYTVOŘIT'}
		</button>
	</div>

	<!-- Quick Dev Accounts -->
	<div class="mt-6 pt-4 border-t-2 border-black">
		<div class="grid grid-cols-3 gap-2">
			<button
				type="button"
				onclick={() => quickLogin('seller@burza.cz')}
				class="py-2 px-1 bg-neutral-100 hover:bg-black hover:text-white border-2 border-black text-black font-black text-xs uppercase tracking-tight transition-colors text-center cursor-pointer truncate"
			>
				PRODEJCE
			</button>
			<button
				type="button"
				onclick={() => quickLogin('buyer@burza.cz')}
				class="py-2 px-1 bg-neutral-100 hover:bg-black hover:text-white border-2 border-black text-black font-black text-xs uppercase tracking-tight transition-colors text-center cursor-pointer truncate"
			>
				KUPUJÍCÍ
			</button>
			<button
				type="button"
				onclick={() => quickLogin('cashier@burza.cz')}
				class="py-2 px-1 bg-neutral-100 hover:bg-black hover:text-white border-2 border-black text-black font-black text-xs uppercase tracking-tight transition-colors text-center cursor-pointer truncate"
			>
				POKLADNÍ
			</button>
		</div>
	</div>
</div>
