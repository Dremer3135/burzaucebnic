<script lang="ts">
	import { legalConfig } from '$lib/legalConfig';
</script>

<svelte:head>
	<title>Pravidla burzy učebnic | {legalConfig.schoolName}</title>
</svelte:head>

<div class="flex-1 w-full bg-white text-black overflow-y-auto px-4 py-8 sm:py-12">
	<div class="max-w-2xl mx-auto">
		<div class="mb-6">
			<a href="/" class="text-xs text-neutral-400 hover:text-black transition-colors">
				← Zpět na burzu
			</a>
		</div>

		<h1 class="text-2xl sm:text-3xl font-bold text-black mb-1">
			Pravidla studentské burzy učebnic
		</h1>
		<p class="text-xs text-neutral-500 mb-8 uppercase tracking-wide">
			{legalConfig.schoolName}
		</p>

		<div class="space-y-6 text-sm text-neutral-800 leading-relaxed">
			<section>
				<h2 class="font-bold text-base text-black mb-1.5">
					1. Účel platformy
				</h2>
				<p>
					Platforma slouží pro usnadnění prodeje a nákupu učebnic mezi studenty školy {legalConfig.schoolName}.
				</p>
			</section>

			<section>
				<h2 class="font-bold text-base text-black mb-1.5">
					2. Průběh nákupu a platby
				</h2>
				<ul class="list-disc pl-5 space-y-1.5">
					<li>
						Prodej funguje formou samoobsluhy. Kupující si učebnice sám vybere a hradí částku na pokladně.
					</li>
					<li>
						Platbu je možné provést jak v hotovosti, tak bezhotovostně přes vygenerovaný QR kód na sběrný účet organizátorů.
					</li>
					<li>
						Pro správné spárování QR platby je kupující povinen zachovat předvyplněný Variabilní symbol (VS).
					</li>
					<li>
						Peníze za prodané učebnice budou prodávajícím odeslány na jimi zadaný bankovní účet do {legalConfig.payoutDays} po oficiálním skončení burzy, nebo předány v hotovosti spolu s neprodanými učebnicemi.
					</li>
				</ul>
			</section>

			<section>
				<h2 class="font-bold text-base text-black mb-1.5">
					3. Odpovědnost
				</h2>
				<ul class="list-disc pl-5 space-y-1.5">
					<li>
						Organizátoři zajišťují výhradně technické a organizační zprostředkování prodeje a úschovu vybraných plateb.
					</li>
					<li>
						Za reálný fyzický stav nabízené učebnice odpovídá prodávající student.
					</li>
					<li>
						Kupující má možnost si veškeré učebnice před nákupem plně fyzicky prohlédnout a zkontrolovat. Po zaplacení a převzetí knihy na pokladně již nelze nákup stornovat ani reklamovat.
					</li>
				</ul>
			</section>

			<section class="pt-2">
				<p>
					Kontakt na organizátory:
					{#each legalConfig.contactEmails as email, i}
						<a href="mailto:{email}" class="underline hover:text-neutral-600">{email}</a>{i < legalConfig.contactEmails.length - 1 ? ', ' : ''}
					{/each}
				</p>
			</section>
		</div>

		<hr class="my-8 border-neutral-200" />

		<div class="flex items-center justify-between text-xs text-neutral-400">
			<a href="/" class="hover:text-black transition-colors">
				← Zpět na burzu
			</a>
			<a href="/privacy" class="hover:text-black underline transition-colors">
				Informace o zpracování osobních údajů (GDPR) →
			</a>
		</div>
	</div>
</div>
