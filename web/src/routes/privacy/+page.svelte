<script lang="ts">
	import { legalConfig } from '$lib/legalConfig';
	import { ArrowLeft, Shield, Mail, FileText, Lock } from '@lucide/svelte';
</script>

<svelte:head>
	<title>Zpracování osobních údajů (GDPR) | {legalConfig.schoolName}</title>
</svelte:head>

<div class="flex-1 w-full bg-neutral-50 text-black overflow-y-auto p-4 sm:p-6 md:p-8">
	<div class="max-w-3xl mx-auto space-y-6">
		<!-- Top Bar / Navigation -->
		<div class="flex items-center justify-between gap-4">
			<a
				href="/"
				class="inline-flex items-center gap-2 py-2 px-3 bg-white border-2 border-black text-black font-black text-xs uppercase tracking-wider hover:bg-black hover:text-white transition-colors cursor-pointer shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:translate-x-0.5 active:translate-y-0.5"
			>
				<ArrowLeft class="w-4 h-4" />
				<span>Zpět na burzu</span>
			</a>

			<!-- Tabs Switcher -->
			<div class="inline-flex border-2 border-black bg-white shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
				<a
					href="/terms"
					class="py-2 px-3 text-black hover:bg-neutral-100 font-bold text-xs uppercase tracking-wider flex items-center gap-1.5 transition-colors"
				>
					<FileText class="w-3.5 h-3.5 text-neutral-500" />
					<span>Pravidla</span>
				</a>
				<span class="py-2 px-3 bg-black text-white font-black text-xs uppercase tracking-wider flex items-center gap-1.5">
					<Shield class="w-3.5 h-3.5" />
					<span>GDPR</span>
				</span>
			</div>
		</div>

		<!-- Main Card -->
		<article class="bg-white border-2 border-black p-6 sm:p-8 md:p-10 shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] space-y-8">
			<!-- Header -->
			<header class="border-b-2 border-black pb-6">
				<div class="inline-flex items-center gap-2 px-2.5 py-1 bg-neutral-100 border border-black text-[11px] font-black uppercase tracking-wider mb-3">
					<Lock class="w-3.5 h-3.5" />
					<span>Ochrana soukromí</span>
				</div>
				<h1 class="text-2xl sm:text-3xl font-black uppercase tracking-tight text-black">
					Informace o zpracování osobních údajů
				</h1>
				<p class="text-xs sm:text-sm font-bold text-neutral-600 mt-1 uppercase">
					Studentská burza učebnic • {legalConfig.schoolName}
				</p>
			</header>

			<!-- Content Sections -->
			<div class="space-y-8 text-neutral-900 leading-relaxed text-sm sm:text-base font-normal">
				<!-- Section 1 -->
				<section class="space-y-2">
					<div class="flex items-center gap-2.5">
						<span class="w-6 h-6 rounded-none bg-black text-white font-mono font-black text-xs flex items-center justify-center shrink-0">
							1
						</span>
						<h2 class="text-lg sm:text-xl font-black uppercase tracking-tight text-black">
							Kdo údaje spravuje
						</h2>
					</div>
					<div class="pl-8 space-y-2 text-neutral-800">
						<p>
							Správcem údajů je studentský organizační tým Burzy učebnic <strong>{legalConfig.schoolName}</strong> pod vedením:
							<strong>{legalConfig.controllers.join(' a ')}</strong>.
						</p>
						<div class="flex flex-wrap items-center gap-2 pt-1">
							<span class="text-xs font-bold uppercase text-neutral-500">Kontakt:</span>
							{#each legalConfig.controllerEmails as email}
								<a
									href="mailto:{email}"
									class="inline-flex items-center gap-1 text-xs font-mono font-bold text-black underline hover:text-neutral-600"
								>
									<Mail class="w-3 h-3" />
									<span>{email}</span>
								</a>
							{/each}
						</div>
					</div>
				</section>

				<!-- Section 2 -->
				<section class="space-y-3">
					<div class="flex items-center gap-2.5">
						<span class="w-6 h-6 rounded-none bg-black text-white font-mono font-black text-xs flex items-center justify-center shrink-0">
							2
						</span>
						<h2 class="text-lg sm:text-xl font-black uppercase tracking-tight text-black">
							Jaké údaje shromažďujeme a proč
						</h2>
					</div>
					<div class="pl-8 space-y-3 text-neutral-800">
						<ul class="space-y-2.5 list-disc pl-4 marker:text-black">
							<li>
								<strong>Školní e-mail, jméno a profilová fotografie</strong> (získané přes Google přihlášení) – pro identifikaci uživatele a doručování mailů o stavu nákupu, prodeje a vyúčtování.
							</li>
							<li>
								<strong>Číslo bankovního účtu (IBAN)</strong> – zadává se <em>dobrovolně</em> pouze u prodávajících, kteří si přejí zaslat výtěžek na bankovní účet. Pokud číslo účtu nezadají, budou jim peníze předány v hotovosti spolu s vrácenými učebnicemi.
							</li>
							<li>
								<strong>Informace o platbách</strong> (variabilní symbol, částka, čas přijetí platby) – pro automatické spárování plateb a kontrolu vyúčtování.
							</li>
						</ul>
						<div class="p-3 bg-neutral-100 border border-black text-xs font-semibold text-neutral-800">
							<strong>Právní základ:</strong> Právním základem pro zpracování je plnění dohody o zprostředkování prodeje (poskytnutí služby).
						</div>
					</div>
				</section>

				<!-- Section 3 -->
				<section class="space-y-3">
					<div class="flex items-center gap-2.5">
						<span class="w-6 h-6 rounded-none bg-black text-white font-mono font-black text-xs flex items-center justify-center shrink-0">
							3
						</span>
						<h2 class="text-lg sm:text-xl font-black uppercase tracking-tight text-black">
							Doba uchování a výmaz
						</h2>
					</div>
					<div class="pl-8 space-y-2.5 text-neutral-800">
						<p>
							Veškeré relace mezi identitou studentů a bankovními účty budou z databáze trvale smazány <strong>nejpozději do 14 dnů</strong> po dokončení všech vyplácení a uzavření ročníku burzy.
						</p>
						<p class="font-bold text-black text-xs uppercase tracking-wide">
							Údaje nejsou předávány žádným komerčním subjektům ani třetím stranám.
						</p>
					</div>
				</section>
			</div>

			<!-- Footer Contact -->
			<footer class="pt-6 border-t-2 border-black flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 bg-neutral-50 p-4 border border-black">
				<div>
					<span class="block text-xs font-black uppercase tracking-wider text-neutral-500">
						Zpracování osobních údajů
					</span>
					<span class="text-sm font-bold text-black">Máte dotaz ke svým údajům?</span>
				</div>
				<div class="flex flex-wrap items-center gap-2">
					{#each legalConfig.controllerEmails as email}
						<a
							href="mailto:{email}"
							class="inline-flex items-center gap-1.5 py-1.5 px-3 bg-white border border-black text-xs font-bold text-black hover:bg-black hover:text-white transition-colors"
						>
							<Mail class="w-3.5 h-3.5" />
							<span>{email}</span>
						</a>
					{/each}
				</div>
			</footer>
		</article>

		<!-- Bottom return link -->
		<div class="text-center py-2">
			<a
				href="/"
				class="text-xs font-black uppercase tracking-wider text-neutral-500 hover:text-black hover:underline"
			>
				← Návrat na hlavní stránku burzy
			</a>
		</div>
	</div>
</div>
