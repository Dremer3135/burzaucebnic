<script lang="ts">
	import { legalConfig } from '$lib/legalConfig';
	import { Tag, ShoppingCart, Camera, Scan, PenTool, CheckCircle2, QrCode, ArrowRight, BookOpen, Sparkles } from '@lucide/svelte';

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
			<div class="inline-flex items-center gap-1.5 px-2 py-0.5 bg-black text-white text-[10px] font-black uppercase tracking-wider mb-2">
				<BookOpen class="w-3 h-3" />
				<span>Průvodce burzou</span>
			</div>
			<h1 class="text-2xl sm:text-3xl font-black uppercase tracking-tight text-black leading-none mb-1">
				Návod k použití
			</h1>
			<p class="text-xs text-neutral-600 font-medium">
				Jak funguje prodej a nákup učebnic na {legalConfig.schoolName}
			</p>
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
				<div class="p-4 bg-neutral-100 border-2 border-black">
					<p class="text-xs font-bold text-neutral-700 leading-relaxed">
						Prodej učebnic je jednoduchý proces o 6 krocích. K prodeji budete potřebovat arch vytištěných samolepek s kódy, pero na zapsání ceny a telefon s fotoaparátem.
					</p>
				</div>

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
								<div class="w-32 h-44 bg-white border-2 border-black relative shadow-xs p-1">
									<div class="absolute top-1.5 right-1.5 w-10 h-10 border border-black bg-yellow-200 flex flex-col items-center justify-center text-[7px] font-black text-center shadow-xs">
										<span>KÓD</span>
										<span class="text-[6px] text-neutral-600">ZDE</span>
									</div>
									<div class="mt-14 px-2">
										<div class="h-2 bg-neutral-200 w-full mb-1"></div>
										<div class="h-1.5 bg-neutral-100 w-3/4 mb-1"></div>
										<div class="h-1 bg-neutral-100 w-1/2"></div>
									</div>
									<div class="absolute bottom-1.5 left-2 text-[8px] font-bold text-neutral-400 uppercase">
										Přední obálka
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
								Perem nebo nesmazatelným fixem napište stejnou prodejní cenu na volné bílé místo na nálepce (přímo pod barevnou linkou).
							</p>

							<!-- Sticker Diagram -->
							<div class="p-3 bg-white border-2 border-black max-w-xs mx-auto">
								<div class="border border-neutral-300 p-2 text-center">
									<div class="flex items-center justify-center gap-2 mb-1.5">
										<div class="w-6 h-6 bg-black flex items-center justify-center text-white text-[8px] font-mono">
											:::
										</div>
										<span class="text-[10px] font-mono font-bold text-neutral-600">kód učebnice</span>
									</div>
									<!-- Colored divider line -->
									<div class="h-1 bg-gradient-to-r from-emerald-500 via-blue-500 to-purple-500 w-full my-1.5"></div>
									<!-- Price write area -->
									<div class="py-1.5 bg-neutral-50 border border-dashed border-neutral-400">
										<span class="text-[10px] text-neutral-400 uppercase font-black block">volné bílé místo</span>
										<span class="text-base font-black text-black tracking-tight">150 Kč</span>
									</div>
								</div>
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
				<div class="p-4 bg-neutral-100 border-2 border-black">
					<p class="text-xs font-bold text-neutral-700 leading-relaxed">
						Nákup učebnic probíhá přímo na burze. Můžete si knihy prohlédnout, zjistit jejich cenu a zaplatit v hotovosti nebo okamžitě QR kódem.
					</p>
				</div>

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
		<div class="mt-8 p-4 border-2 border-black bg-neutral-50 flex items-center justify-between gap-4">
			<div>
				<p class="text-xs font-black uppercase tracking-wide text-black">Potřebujete poradit?</p>
				<p class="text-[11px] text-neutral-600 font-medium">Napište na {legalConfig.contactEmails.join(', ')} nebo se zeptejte obsluhy u pokladny.</p>
			</div>
			<a
				href="/sell"
				class="px-4 py-2.5 bg-black text-white hover:bg-neutral-800 font-black text-xs uppercase tracking-wider border-2 border-black shrink-0 transition-colors"
			>
				Přejít na burzu
			</a>
		</div>
	</div>
</div>
