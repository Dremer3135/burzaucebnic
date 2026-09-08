<script lang="ts">
	import { legalConfig } from '$lib/legalConfig';
</script>

<svelte:head>
	<title>Zpracování osobních údajů (GDPR) | {legalConfig.schoolName}</title>
</svelte:head>

<div class="flex-1 w-full bg-white text-black overflow-y-auto px-4 py-8 sm:py-12">
	<div class="max-w-2xl mx-auto">
		<div class="mb-6">
			<a href="/" class="text-xs text-neutral-400 hover:text-black transition-colors">
				← Zpět na burzu
			</a>
		</div>

		<h1 class="text-2xl sm:text-3xl font-bold text-black mb-1">
			Informace o zpracování osobních údajů
		</h1>
		<p class="text-xs text-neutral-500 mb-8 uppercase tracking-wide">
			Studentská burza učebnic • {legalConfig.schoolName}
		</p>

		<div class="space-y-6 text-sm text-neutral-800 leading-relaxed">
			<section>
				<h2 class="font-bold text-base text-black mb-1.5">
					1. Kdo údaje spravuje
				</h2>
				<p>
					Správcem údajů je studentský organizační tým Burzy učebnic {legalConfig.schoolName} pod vedením:
					<strong>{legalConfig.controllers.join(' a ')}</strong>, kontakt:
					{#each legalConfig.controllerEmails as email, i}
						<a href="mailto:{email}" class="underline hover:text-neutral-600">{email}</a>{i < legalConfig.controllerEmails.length - 1 ? ', ' : ''}
					{/each}.
				</p>
			</section>

			<section>
				<h2 class="font-bold text-base text-black mb-1.5">
					2. Jaké údaje shromažďujeme a proč
				</h2>
				<ul class="list-disc pl-5 space-y-1.5">
					<li>
						<strong>Školní e-mail, jméno a profilová fotografie</strong> (získané přes Google přihlášení) – pro identifikaci uživatele a doručování mailů o stavu nákupu, prodeje a vyúčtování.
					</li>
					<li>
						<strong>Číslo bankovního účtu (IBAN)</strong> – zadává se dobrovolně pouze u prodávajících, kteří si přejí zaslat výtěžek na bankovní účet. Pokud číslo účtu nezadají, budou jim peníze předány v hotovosti spolu s vrácenými učebnicemi.
					</li>
					<li>
						<strong>Informace o platbách</strong> (variabilní symbol, částka, čas přijetí platby) – pro automatické spárování plateb a kontrolu vyúčtování.
					</li>
				</ul>
				<p class="mt-2 text-xs text-neutral-600">
					Právním základem pro zpracování je plnění dohody o zprostředkování prodeje (poskytnutí služby).
				</p>
			</section>

			<section>
				<h2 class="font-bold text-base text-black mb-1.5">
					3. Doba uchování a výmaz
				</h2>
				<p>
					Veškeré relace mezi identitou studentů a bankovními účty budou z databáze trvale smazány nejpozději do 14 dnů po dokončení všech vyplácení a uzavření ročníku burzy. Údaje nejsou předávány žádným komerčním subjektům ani třetím stranám.
				</p>
			</section>
		</div>

		<hr class="my-8 border-neutral-200" />

		<div class="flex items-center justify-between text-xs text-neutral-400">
			<a href="/" class="hover:text-black transition-colors">
				← Zpět na burzu
			</a>
			<a href="/terms" class="hover:text-black underline transition-colors">
				← Pravidla studentské burzy učebnic
			</a>
		</div>
	</div>
</div>
