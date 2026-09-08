<script lang="ts">
	import { legalConfig } from '$lib/legalConfig';
	import { ArrowLeft, BookOpen, Shield, Mail, FileText } from '@lucide/svelte';
</script>

<svelte:head>
	<title>Pravidla burzy učebnic | {legalConfig.schoolName}</title>
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
				<span class="py-2 px-3 bg-black text-white font-black text-xs uppercase tracking-wider flex items-center gap-1.5">
					<FileText class="w-3.5 h-3.5" />
					<span>Pravidla</span>
				</span>
				<a
					href="/privacy"
					class="py-2 px-3 text-black hover:bg-neutral-100 font-bold text-xs uppercase tracking-wider flex items-center gap-1.5 transition-colors"
				>
					<Shield class="w-3.5 h-3.5 text-neutral-500" />
					<span>GDPR</span>
				</a>
			</div>
		</div>

		<!-- Main Card -->
		<article class="bg-white border-2 border-black p-6 sm:p-8 md:p-10 shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] space-y-8">
			<!-- Header -->
			<header class="border-b-2 border-black pb-6">
				<div class="inline-flex items-center gap-2 px-2.5 py-1 bg-neutral-100 border border-black text-[11px] font-black uppercase tracking-wider mb-3">
					<BookOpen class="w-3.5 h-3.5" />
					<span>Oficiální pravidla</span>
				</div>
				<h1 class="text-2xl sm:text-3xl font-black uppercase tracking-tight text-black">
					Pravidla studentské burzy učebnic
				</h1>
				<p class="text-xs sm:text-sm font-bold text-neutral-600 mt-1 uppercase">
					{legalConfig.schoolName}
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
							Účel platformy
						</h2>
					</div>
					<div class="pl-8 text-neutral-800">
						<p>
							Platforma slouží pro usnadnění prodeje a nákupu učebnic mezi studenty školy <strong>{legalConfig.schoolName}</strong>.
						</p>
					</div>
				</section>

				<!-- Section 2 -->
				<section class="space-y-3">
					<div class="flex items-center gap-2.5">
						<span class="w-6 h-6 rounded-none bg-black text-white font-mono font-black text-xs flex items-center justify-center shrink-0">
							2
						</span>
						<h2 class="text-lg sm:text-xl font-black uppercase tracking-tight text-black">
							Průběh nákupu a platby
						</h2>
					</div>
					<div class="pl-8 space-y-2.5 text-neutral-800">
						<ul class="space-y-2 list-disc pl-4 marker:text-black">
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
					</div>
				</section>

				<!-- Section 3 -->
				<section class="space-y-3">
					<div class="flex items-center gap-2.5">
						<span class="w-6 h-6 rounded-none bg-black text-white font-mono font-black text-xs flex items-center justify-center shrink-0">
							3
						</span>
						<h2 class="text-lg sm:text-xl font-black uppercase tracking-tight text-black">
							Odpovědnost
						</h2>
					</div>
					<div class="pl-8 space-y-2.5 text-neutral-800">
						<ul class="space-y-2 list-disc pl-4 marker:text-black">
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
					</div>
				</section>
			</div>

			<!-- Footer Contact -->
			<footer class="pt-6 border-t-2 border-black flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 bg-neutral-50 p-4 border border-black">
				<div>
					<span class="block text-xs font-black uppercase tracking-wider text-neutral-500">
						Dotazy a podpora
					</span>
					<span class="text-sm font-bold text-black">Kontakt na organizátory:</span>
				</div>
				<div class="flex items-center gap-2">
					{#each legalConfig.contactEmails as email}
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
