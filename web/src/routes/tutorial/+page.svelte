<script lang="ts">
	import { legalConfig } from '$lib/legalConfig';
	import { Tag, ShoppingCart } from '@lucide/svelte';

	let activeTab = $state<'sell' | 'buy'>('sell');
</script>

<svelte:head>
	<title>Návod k použití | Burza učebnic</title>
</svelte:head>

<div class="flex-1 w-full bg-white text-black overflow-y-auto px-4 py-6 sm:py-10">
	<div class="max-w-2xl mx-auto">
		<!-- Top Back Link -->
		<div class="mb-5">
			<a href="/" class="text-xs font-bold text-neutral-500 hover:text-black transition-colors uppercase tracking-wider inline-flex items-center gap-1">
				← Zpět na burzu
			</a>
		</div>

		<!-- Page Header -->
		<div class="mb-6 border-b-2 border-black pb-4">
			<h1 class="text-2xl sm:text-3xl font-black uppercase tracking-tight text-black leading-none">
				Návod k použití
			</h1>
		</div>

		<!-- Mode Switch Tabs -->
		<div class="grid grid-cols-2 gap-2 mb-6">
			<button
				onclick={() => (activeTab = 'sell')}
				class="py-3 px-4 border-2 border-black font-black text-xs uppercase tracking-wider flex items-center justify-center gap-2 cursor-pointer transition-all {activeTab === 'sell'
					? 'bg-black text-white shadow-[3px_3px_0px_0px_rgba(0,0,0,1)]'
					: 'bg-white text-black hover:bg-neutral-100'}"
			>
				<Tag class="w-4 h-4" />
				<span>Jak prodávat</span>
			</button>

			<button
				onclick={() => (activeTab = 'buy')}
				class="py-3 px-4 border-2 border-black font-black text-xs uppercase tracking-wider flex items-center justify-center gap-2 cursor-pointer transition-all {activeTab === 'buy'
					? 'bg-black text-white shadow-[3px_3px_0px_0px_rgba(0,0,0,1)]'
					: 'bg-white text-black hover:bg-neutral-100'}"
			>
				<ShoppingCart class="w-4 h-4" />
				<span>Jak nakupovat</span>
			</button>
		</div>

		<!-- ============================================== -->
		<!-- TAB 1: SELLING GUIDE -->
		<!-- ============================================== -->
		{#if activeTab === 'sell'}
			<div class="space-y-4">
				<!-- Step 1 -->
				<div class="p-4 border-2 border-black bg-white shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
					<div class="flex items-start gap-3">
						<span class="w-7 h-7 bg-black text-white font-black text-xs flex items-center justify-center border border-black shrink-0">
							1
						</span>
						<div class="flex-1 min-w-0">
							<h2 class="text-sm font-black uppercase tracking-wide text-black mb-1">
								Nalepte nálepku na knihu
							</h2>
							<p class="text-xs text-neutral-700 leading-relaxed mb-3">
								Vezměte jednu volnou samolepku s kódem z archu a nalepte ji do <strong>pravého horního rohu</strong> přední desky učebnice.
							</p>

							<!-- Visual Book Schematic -->
							<div class="p-3 bg-neutral-50 border border-neutral-300 flex items-center justify-center">
								<div class="w-36 h-48 bg-white border-2 border-black relative shadow-xs p-2 flex flex-col justify-between">
									<!-- Realistic sticker in top-right corner -->
									<div class="self-end w-24 bg-white border border-black p-1 shadow-xs">
										<div class="flex items-center gap-1.5">
											<!-- Mini Data Matrix -->
											<div class="w-6 h-6 shrink-0">
												<svg viewBox="0 0 64 64" xmlns="http://www.w3.org/2000/svg" class="w-full h-full fill-black">
													<path d="M0 64L64 64L64 60L60 60L60 56L56 56L56 60L44 60L44 56L40 56L40 52L44 52L44 44L40 44L40 48L36 48L36 60L28 60L28 56L32 56L32 36L28 36L28 52L24 52L24 56L20 56L20 60L12 60L12 56L8 56L8 60L4 60L4 52L12 52L12 40L8 40L8 36L4 36L4 32L16 32L16 24L12 24L12 20L8 20L8 16L4 16L4 12L8 12L8 8L4 8L4 0L0 0ZM44 56L48 56L48 52L44 52ZM52 56L56 56L56 52L60 52L60 48L64 48L64 44L56 44L56 48L52 48ZM60 56L64 56L64 52L60 52ZM20 52L24 52L24 44L20 44ZM4 48L4 44L8 44L8 48ZM48 48L52 48L52 44L56 44L56 36L48 36L48 28L44 28L44 32L40 32L40 36L44 36L44 44L48 44ZM16 44L20 44L20 36L12 36L12 40L16 40ZM36 40L40 40L40 36L36 36ZM60 40L64 40L64 36L60 36ZM20 36L28 36L28 32L32 32L32 36L36 36L36 32L40 32L40 28L36 28L36 24L32 24L32 28L24 28L24 32L20 32ZM56 36L60 36L60 32L64 32L64 28L56 28L56 24L52 24L52 32L56 32ZM4 28L4 24L8 24L8 28ZM20 28L24 28L24 24L20 24ZM40 28L44 28L44 24L52 24L52 16L44 16L44 20L36 20L36 24L40 24ZM24 24L28 24L28 16L24 16L24 12L28 12L28 0L24 0L24 4L20 4L20 0L16 0L16 4L12 4L12 0L8 0L8 8L12 8L12 20L16 20L16 16L20 16L20 20L24 20ZM56 24L64 24L64 20L60 20L60 16L64 16L64 12L60 12L60 8L64 8L64 4L60 4L60 0L56 0L56 8L52 8L52 12L56 12ZM28 16L32 16L32 12L28 12ZM36 16L40 16L40 12L48 12L48 8L36 8L36 0L32 0L32 12L36 12ZM16 12L16 8L20 8L20 12ZM48 8L52 8L52 0L48 0ZM40 4L44 4L44 0L40 0Z" fill-rule="evenodd" />
												</svg>
											</div>
											<!-- Mini text & teal line -->
											<div class="flex-1 min-w-0">
												<div class="text-[5.5px] font-mono font-bold text-black truncate leading-tight">nhvlre...</div>
												<div class="h-0.5 bg-[#054a6a] w-full mt-0.5"></div>
												<div class="text-[5px] text-neutral-400 font-bold mt-0.5">cena</div>
											</div>
										</div>
									</div>

									<!-- Book Lines -->
									<div class="px-2 my-auto">
										<div class="h-2 bg-neutral-200 w-full mb-1.5"></div>
										<div class="h-1.5 bg-neutral-100 w-3/4 mb-1"></div>
										<div class="h-1 bg-neutral-100 w-1/2"></div>
									</div>

									<div class="text-[8px] font-bold text-neutral-400 uppercase tracking-wider text-center">
										Přední obálka knihy
									</div>
								</div>
							</div>
						</div>
					</div>
				</div>

				<!-- Step 2 -->
				<div class="p-4 border-2 border-black bg-white shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
					<div class="flex items-start gap-3">
						<span class="w-7 h-7 bg-black text-white font-black text-xs flex items-center justify-center border border-black shrink-0">
							2
						</span>
						<div class="flex-1 min-w-0">
							<h2 class="text-sm font-black uppercase tracking-wide text-black mb-1">
								Otevřete „Prodej“ a stiskněte „Přidat“
							</h2>
							<p class="text-xs text-neutral-700 leading-relaxed">
								V aplikaci se přepněte do sekce <strong>Prodej</strong> a klepněte na tlačítko <strong>„Přidat“</strong> (nebo velké tlačítko pro přidání učebnice). Aplikace si vyžádá přístup k fotoaparátu.
							</p>
						</div>
					</div>
				</div>

				<!-- Step 3 -->
				<div class="p-4 border-2 border-black bg-white shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
					<div class="flex items-start gap-3">
						<span class="w-7 h-7 bg-black text-white font-black text-xs flex items-center justify-center border border-black shrink-0">
							3
						</span>
						<div class="flex-1 min-w-0">
							<h2 class="text-sm font-black uppercase tracking-wide text-black mb-1">
								Naskenujte kód z nalepené nálepky
							</h2>
							<p class="text-xs text-neutral-700 leading-relaxed">
								Namiřte kameru telefonu na nalepenou nálepku v pravém horním rohu knihy. Jakmile aplikace rozpozná kód a ověří, že je volný (zelený rámeček), klepněte na <strong>„Pokračovat na foto“</strong>.
							</p>
						</div>
					</div>
				</div>

				<!-- Step 4 -->
				<div class="p-4 border-2 border-black bg-white shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
					<div class="flex items-start gap-3">
						<span class="w-7 h-7 bg-black text-white font-black text-xs flex items-center justify-center border border-black shrink-0">
							4
						</span>
						<div class="flex-1 min-w-0">
							<h2 class="text-sm font-black uppercase tracking-wide text-black mb-1">
								Vyfoťte obálku učebnice
							</h2>
							<p class="text-xs text-neutral-700 leading-relaxed">
								Přímo v aplikaci vyfoťte přední stranu učebnice tak, aby byla obálka zřetelná a vyplňovala rámeček. Fotografie slouží k vizuální kontrole knihy.
							</p>
						</div>
					</div>
				</div>

				<!-- Step 5 -->
				<div class="p-4 border-2 border-black bg-white shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
					<div class="flex items-start gap-3">
						<span class="w-7 h-7 bg-black text-white font-black text-xs flex items-center justify-center border border-black shrink-0">
							5
						</span>
						<div class="flex-1 min-w-0">
							<h2 class="text-sm font-black uppercase tracking-wide text-black mb-1">
								Zadejte prodejní cenu
							</h2>
							<p class="text-xs text-neutral-700 leading-relaxed">
								Zadejte částku v Kč, za kterou si přejete učebnici prodat, a klepněte na <strong>„Vystavit k prodeji“</strong>.
							</p>
						</div>
					</div>
				</div>

				<!-- Step 6 (CRITICAL: WRITE ON STICKER) -->
				<div class="p-4 border-2 border-black bg-amber-50 shadow-[3px_3px_0px_0px_rgba(0,0,0,1)]">
					<div class="flex items-start gap-3">
						<span class="w-7 h-7 bg-amber-500 text-black font-black text-xs flex items-center justify-center border-2 border-black shrink-0">
							6
						</span>
						<div class="flex-1 min-w-0">
							<div class="inline-block px-1.5 py-0.5 bg-black text-white text-[9px] font-black uppercase tracking-wider mb-1">
								Důležitý krok
							</div>
							<h2 class="text-sm font-black uppercase tracking-wide text-black mb-1">
								Napište cenu na nálepku!
							</h2>
							<p class="text-xs text-neutral-800 font-bold leading-relaxed mb-3">
								Perem nebo nesmazatelným fixem napište stejnou prodejní cenu na volné bílé místo na nálepce (přímo pod tmavě modrou linkou).
							</p>

							<!-- Sticker Diagram -->
							<div class="p-4 bg-neutral-100 border-2 border-black max-w-sm mx-auto flex flex-col items-center">
								<!-- Realistic Sticker Card (matching physical sticker) -->
								<div class="w-full max-w-[280px] bg-white border border-neutral-300 p-3 shadow-sm">
									<div class="flex items-start gap-3">
										<!-- Left: Data Matrix SVG -->
										<div class="w-18 h-18 shrink-0 bg-white">
											<svg viewBox="0 0 64 64" xmlns="http://www.w3.org/2000/svg" class="w-full h-full fill-black">
												<path d="M0 64L64 64L64 60L60 60L60 56L56 56L56 60L44 60L44 56L40 56L40 52L44 52L44 44L40 44L40 48L36 48L36 60L28 60L28 56L32 56L32 36L28 36L28 52L24 52L24 56L20 56L20 60L12 60L12 56L8 56L8 60L4 60L4 52L12 52L12 40L8 40L8 36L4 36L4 32L16 32L16 24L12 24L12 20L8 20L8 16L4 16L4 12L8 12L8 8L4 8L4 0L0 0ZM44 56L48 56L48 52L44 52ZM52 56L56 56L56 52L60 52L60 48L64 48L64 44L56 44L56 48L52 48ZM60 56L64 56L64 52L60 52ZM20 52L24 52L24 44L20 44ZM4 48L4 44L8 44L8 48ZM48 48L52 48L52 44L56 44L56 36L48 36L48 28L44 28L44 32L40 32L40 36L44 36L44 44L48 44ZM16 44L20 44L20 36L12 36L12 40L16 40ZM36 40L40 40L40 36L36 36ZM60 40L64 40L64 36L60 36ZM20 36L28 36L28 32L32 32L32 36L36 36L36 32L40 32L40 28L36 28L36 24L32 24L32 28L24 28L24 32L20 32ZM56 36L60 36L60 32L64 32L64 28L56 28L56 24L52 24L52 32L56 32ZM4 28L4 24L8 24L8 28ZM20 28L24 28L24 24L20 24ZM40 28L44 28L44 24L52 24L52 16L44 16L44 20L36 20L36 24L40 24ZM24 24L28 24L28 16L24 16L24 12L28 12L28 0L24 0L24 4L20 4L20 0L16 0L16 4L12 4L12 0L8 0L8 8L12 8L12 20L16 20L16 16L20 16L20 20L24 20ZM56 24L64 24L64 20L60 20L60 16L64 16L64 12L60 12L60 8L64 8L64 4L60 4L60 0L56 0L56 8L52 8L52 12L56 12ZM28 16L32 16L32 12L28 12ZM36 16L40 16L40 12L48 12L48 8L36 8L36 0L32 0L32 12L36 12ZM16 12L16 8L20 8L20 12ZM48 8L52 8L52 0L48 0ZM40 4L44 4L44 0L40 0Z" fill-rule="evenodd" />
											</svg>
										</div>
										<!-- Right: Code + Teal line + Price Area -->
										<div class="flex-1 min-w-0 pt-0.5">
											<div class="font-mono text-xs font-bold text-black tracking-tight leading-tight select-all">
												nhvlre22094812b
											</div>
											<div class="h-1 bg-[#054a6a] w-full mt-1"></div>
											<!-- Handwriting price zone -->
											<div class="mt-2.5 pt-1 border border-dashed border-neutral-300 rounded bg-neutral-50 px-2 py-1 text-center">
												<span class="text-[9px] font-black uppercase text-neutral-400 block tracking-wider">Sem napište cenu</span>
												<span class="text-xl font-black text-black tracking-tight font-sans">150 Kč</span>
											</div>
										</div>
									</div>
								</div>
								<span class="text-[11px] font-bold text-neutral-600 mt-2 text-center">
									Ukázka: Kód nahoře, modrá linka a cena napsaná pod ní
								</span>
							</div>
						</div>
					</div>
				</div>

				<!-- Step 7 -->
				<div class="p-4 border-2 border-black bg-white shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
					<div class="flex items-start gap-3">
						<span class="w-7 h-7 bg-black text-white font-black text-xs flex items-center justify-center border border-black shrink-0">
							7
						</span>
						<div class="flex-1 min-w-0">
							<h2 class="text-sm font-black uppercase tracking-wide text-black mb-1">
								Odevzdání knih na burze
							</h2>
							<p class="text-xs text-neutral-700 leading-relaxed">
								Označené knihy přineste na sběrné místo burzy ve stanoveném termínu. Obsluha zkontroluje stav knih a potvrdí jejich příjem do systému. Po skončení burzy vám peníze vyplatíme na bankovní účet nebo v hotovosti.
							</p>
						</div>
					</div>
				</div>
			</div>
		{:else}
			<!-- ============================================== -->
			<!-- TAB 2: BUYING GUIDE -->
			<!-- ============================================== -->
			<div class="space-y-4">
				<!-- Step 1 -->
				<div class="p-4 border-2 border-black bg-white shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
					<div class="flex items-start gap-3">
						<span class="w-7 h-7 bg-black text-white font-black text-xs flex items-center justify-center border border-black shrink-0">
							1
						</span>
						<div class="flex-1 min-w-0">
							<h2 class="text-sm font-black uppercase tracking-wide text-black mb-1">
								Výběr učebnic
							</h2>
							<p class="text-xs text-neutral-700 leading-relaxed">
								Přijďte do prostor konání burzy a vyberte si učebnice přímo z fyzických pultů. Můžete si prolistovat stav každé knihy.
							</p>
						</div>
					</div>
				</div>

				<!-- Step 2 -->
				<div class="p-4 border-2 border-black bg-white shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
					<div class="flex items-start gap-3">
						<span class="w-7 h-7 bg-black text-white font-black text-xs flex items-center justify-center border border-black shrink-0">
							2
						</span>
						<div class="flex-1 min-w-0">
							<h2 class="text-sm font-black uppercase tracking-wide text-black mb-1">
								Ověření ceny („Cena“)
							</h2>
							<p class="text-xs text-neutral-700 leading-relaxed">
								Pokud si chcete ověřit cenu nebo podrobnosti knihy, přepněte se v horním menu do režimu <strong>„Cena“</strong> a namiřte fotoaparát na kód na nálepce. Aplikace okamžitě zobrazí cenu a stav knihy.
							</p>
						</div>
					</div>
				</div>

				<!-- Step 3 -->
				<div class="p-4 border-2 border-black bg-white shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
					<div class="flex items-start gap-3">
						<span class="w-7 h-7 bg-black text-white font-black text-xs flex items-center justify-center border border-black shrink-0">
							3
						</span>
						<div class="flex-1 min-w-0">
							<h2 class="text-sm font-black uppercase tracking-wide text-black mb-1">
								Platba u pokladny (QR kód nebo hotovost)
							</h2>
							<p class="text-xs text-neutral-700 leading-relaxed mb-2">
								S vybranými knihami přejděte k pokladně, kde obsluha naskenuje vaše knihy a vygeneruje platbu:
							</p>
							<ul class="list-disc pl-5 text-xs text-neutral-700 space-y-1">
								<li><strong>QR platba (doporučeno):</strong> Naskenujte SPAYD QR kód ve své bankovní aplikaci a odešlete okamžitou platbu.</li>
								<li><strong>Hotovost:</strong> Zaplaťte přímo pokladníkovi.</li>
							</ul>
						</div>
					</div>
				</div>

				<!-- Step 4 -->
				<div class="p-4 border-2 border-black bg-white shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
					<div class="flex items-start gap-3">
						<span class="w-7 h-7 bg-black text-white font-black text-xs flex items-center justify-center border border-black shrink-0">
							4
						</span>
						<div class="flex-1 min-w-0">
							<h2 class="text-sm font-black uppercase tracking-wide text-black mb-1">
								Převzetí zakoupených knih
							</h2>
							<p class="text-xs text-neutral-700 leading-relaxed">
								Jakmile je platba potvrzena na pokladně (nebo automaticky přes bankovní párování), knihy jsou vaše a můžete s nimi odejít.
							</p>
						</div>
					</div>
				</div>
			</div>
		{/if}

		<!-- Bottom Action / Support -->
		<div class="mt-8 p-4 border-2 border-black bg-neutral-50">
			<p class="text-xs font-black uppercase tracking-wide text-black">Potřebujete poradit?</p>
			<p class="text-[11px] text-neutral-600 font-medium mt-0.5">Napište na {legalConfig.contactEmails.join(', ')} nebo se zeptejte obsluhy u pokladny.</p>
		</div>
	</div>
</div>
