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
												<svg viewBox="0 0 72 72" xmlns="http://www.w3.org/2000/svg" class="w-full h-full">
													<rect width="100%" height="100%" fill="#ffffff" />
													<path d="M4 68L68 68L68 64L64 64L64 60L60 60L60 64L48 64L48 60L44 60L44 56L48 56L48 48L44 48L44 52L40 52L40 64L32 64L32 60L36 60L36 40L32 40L32 56L28 56L28 60L24 60L24 64L16 64L16 60L12 60L12 64L8 64L8 56L16 56L16 44L12 44L12 40L8 40L8 36L20 36L20 28L16 28L16 24L12 24L12 20L8 20L8 16L12 16L12 12L8 12L8 4L4 4ZM48 60L52 60L52 56L48 56ZM56 60L60 60L60 56L64 56L64 52L68 52L68 48L60 48L60 52L56 52ZM64 60L68 60L68 56L64 56ZM24 56L28 56L28 48L24 48ZM8 52L8 48L12 48L12 52ZM52 52L56 52L56 48L60 48L60 40L52 40L52 32L48 32L48 36L44 36L44 40L48 40L48 48L52 48ZM20 48L24 48L24 40L16 40L16 44L20 44ZM40 44L44 44L44 40L40 40ZM64 44L68 44L68 40L64 40ZM24 40L32 40L32 36L36 36L36 40L40 40L40 36L44 36L44 32L40 32L40 28L36 28L36 32L28 32L28 36L24 36ZM60 40L64 40L64 36L68 36L68 32L60 32L60 28L56 28L56 36L60 36ZM8 32L8 28L12 28L12 32ZM24 32L28 32L28 28L24 28ZM44 32L48 32L48 28L56 28L56 20L48 20L48 24L40 24L40 28L44 28ZM28 28L32 28L32 20L28 20L28 16L32 16L32 4L28 4L28 8L24 8L24 4L20 4L20 8L16 8L16 4L12 4L12 12L16 12L16 24L20 24L20 20L24 20L24 24L28 24ZM60 28L68 28L68 24L64 24L64 20L68 20L68 16L64 16L64 12L68 12L68 8L64 8L64 4L60 4L60 12L56 12L56 16L60 16ZM32 20L36 20L36 16L32 16ZM40 20L44 20L44 16L52 16L52 12L40 12L40 4L36 4L36 16L40 16ZM20 16L20 12L24 12L24 16ZM52 12L56 12L56 4L52 4ZM44 8L48 8L48 4L44 4Z" fill="#000000" fill-rule="evenodd" />
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
							<div class="p-4 sm:p-5 bg-neutral-100 border-2 border-black max-w-sm mx-auto flex flex-col items-center gap-3">
								<!-- Realistic Physical Sticker Card -->
								<div class="w-full max-w-[310px] bg-white border-2 border-neutral-200 rounded shadow-[0_2px_8px_rgba(0,0,0,0.08)] p-3 sm:p-3.5 relative">
									<div class="flex items-start gap-2.5 sm:gap-3.5">
										<!-- Left: Data Matrix SVG with quiet zone -->
										<div class="w-14 h-14 sm:w-16 sm:h-16 shrink-0 bg-white">
											<svg viewBox="0 0 72 72" xmlns="http://www.w3.org/2000/svg" class="w-full h-full">
												<rect width="100%" height="100%" fill="#ffffff" />
												<path d="M4 68L68 68L68 64L64 64L64 60L60 60L60 64L48 64L48 60L44 60L44 56L48 56L48 48L44 48L44 52L40 52L40 64L32 64L32 60L36 60L36 40L32 40L32 56L28 56L28 60L24 60L24 64L16 64L16 60L12 60L12 64L8 64L8 56L16 56L16 44L12 44L12 40L8 40L8 36L20 36L20 28L16 28L16 24L12 24L12 20L8 20L8 16L12 16L12 12L8 12L8 4L4 4ZM48 60L52 60L52 56L48 56ZM56 60L60 60L60 56L64 56L64 52L68 52L68 48L60 48L60 52L56 52ZM64 60L68 60L68 56L64 56ZM24 56L28 56L28 48L24 48ZM8 52L8 48L12 48L12 52ZM52 52L56 52L56 48L60 48L60 40L52 40L52 32L48 32L48 36L44 36L44 40L48 40L48 48L52 48ZM20 48L24 48L24 40L16 40L16 44L20 44ZM40 44L44 44L44 40L40 40ZM64 44L68 44L68 40L64 40ZM24 40L32 40L32 36L36 36L36 40L40 40L40 36L44 36L44 32L40 32L40 28L36 28L36 32L28 32L28 36L24 36ZM60 40L64 40L64 36L68 36L68 32L60 32L60 28L56 28L56 36L60 36ZM8 32L8 28L12 28L12 32ZM24 32L28 32L28 28L24 28ZM44 32L48 32L48 28L56 28L56 20L48 20L48 24L40 24L40 28L44 28ZM28 28L32 28L32 20L28 20L28 16L32 16L32 4L28 4L28 8L24 8L24 4L20 4L20 8L16 8L16 4L12 4L12 12L16 12L16 24L20 24L20 20L24 20L24 24L28 24ZM60 28L68 28L68 24L64 24L64 20L68 20L68 16L64 16L64 12L68 12L68 8L64 8L64 4L60 4L60 12L56 12L56 16L60 16ZM32 20L36 20L36 16L32 16ZM40 20L44 20L44 16L52 16L52 12L40 12L40 4L36 4L36 16L40 16ZM20 16L20 12L24 12L24 16ZM52 12L56 12L56 4L52 4ZM44 8L48 8L48 4L44 4Z" fill="#000000" fill-rule="evenodd" />
											</svg>
										</div>
										<!-- Right: Code + Teal line + Price Area -->
										<div class="flex-1 min-w-0 pt-0.5">
											<div class="inline-block">
												<div class="font-mono text-[10px] sm:text-[11px] font-bold text-black tracking-tight leading-none whitespace-nowrap">
													nhvlre22094812b
												</div>
												<div class="h-0.5 sm:h-1 bg-[#054a6a] w-full mt-1 rounded-xs"></div>
											</div>
											<!-- Clean price zone -->
											<div class="mt-2 text-center">
												<span class="text-xl sm:text-2xl font-black text-black tracking-tight block leading-tight">
													150 Kč
												</span>
											</div>
										</div>
									</div>
								</div>
								<p class="text-[11px] font-bold text-neutral-600 text-center leading-snug">
									Kód je vytištěný na archu. Cenu dopište perem na bílé místo pod linku.
								</p>
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
								Po odeslání platby na pokladně můžete s knihami rovnou odejít – nemusíte čekat na potvrzení připsání platby v systému. Pokud by nastal jakýkoliv problém s platbou, budeme vás kontaktovat e-mailem nebo osobně.
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
