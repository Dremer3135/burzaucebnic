<script lang="ts">
	import { onMount } from 'svelte';
	import { pb } from '$lib/pocketbase';
	import { auth } from '$lib/stores.svelte';
	import type { EmailTemplate, EmailCampaignStats } from '$lib/types';
	import {
		Mail,
		Send,
		Eye,
		Settings,
		CheckCircle2,
		AlertCircle,
		AlertTriangle,
		RefreshCw,
		Users,
		BookOpen,
		DollarSign,
		Check,
		X,
		FileText,
		ExternalLink,
		CreditCard,
		Banknote
	} from '@lucide/svelte';

	let activeTab = $state<'intake' | 'sale' | 'templates'>('intake');

	// Stats
	let stats = $state<EmailCampaignStats | null>(null);
	let statsLoading = $state(false);

	// Templates
	let templates = $state<EmailTemplate[]>([]);
	let selectedTemplateKey = $state<'intake_recap' | 'sale_summary'>('intake_recap');
	let templateForm = $state<Partial<EmailTemplate>>({});
	let savingTemplate = $state(false);
	let templateSaveSuccess = $state(false);

	// Previews
	let previewData = $state<{ subject: string; html: string } | null>(null);
	let previewLoading = $state(false);
	let previewPayoutOption = $state<'bank' | 'cash'>('bank');

	// Actions state
	let testSending = $state(false);
	let testSuccessMessage = $state('');
	let testErrorMessage = $state('');

	// Bulk sending modal & progress
	let showConfirmModal = $state(false);
	let bulkActionType = $state<'intake_recap' | 'sale_summary'>('intake_recap');
	let isBulkSending = $state(false);
	let bulkResult = $state<{
		success: boolean;
		totalSent: number;
		totalErrors: number;
		errors: string[];
	} | null>(null);

	async function loadStats() {
		statsLoading = true;
		try {
			stats = await pb.send<EmailCampaignStats>('/api/admin/emails/stats', {});
		} catch (err) {
			console.error('Failed to load email stats:', err);
		} finally {
			statsLoading = false;
		}
	}

	async function loadTemplates() {
		try {
			const list = await pb.collection('email_templates').getFullList<EmailTemplate>();
			templates = list;
			updateTemplateForm();
		} catch (err) {
			console.error('Failed to load email templates:', err);
		}
	}

	function updateTemplateForm() {
		const found = templates.find((t) => t.key === selectedTemplateKey);
		if (found) {
			templateForm = { ...found };
		} else {
			templateForm = {
				key: selectedTemplateKey,
				name: selectedTemplateKey === 'intake_recap' ? 'Potvrzení příjmu' : 'Vyúčtování',
				subject: '',
				bodyIntro: '',
				unacceptedWarning: '',
				payoutBankNote: '',
				payoutCashNote: '',
				bodyOutro: ''
			};
		}
	}

	async function saveCurrentTemplate() {
		if (!templateForm.key) return;
		savingTemplate = true;
		templateSaveSuccess = false;
		try {
			if (templateForm.id) {
				await pb.collection('email_templates').update(templateForm.id, templateForm);
			} else {
				const created = await pb.collection('email_templates').create<EmailTemplate>(templateForm);
				templateForm.id = created.id;
			}
			templateSaveSuccess = true;
			await loadTemplates();
			await loadPreview();
			setTimeout(() => {
				templateSaveSuccess = false;
			}, 3000);
		} catch (err: any) {
			alert('Chyba při ukládání šablony: ' + (err?.message || err));
		} finally {
			savingTemplate = false;
		}
	}

	async function loadPreview() {
		const type = activeTab === 'sale' ? 'sale_summary' : 'intake_recap';
		previewLoading = true;
		try {
			const res = await pb.send<{ subject: string; html: string }>('/api/admin/emails/preview', {
				method: 'POST',
				body: {
					type,
					payoutOption: previewPayoutOption
				}
			});
			previewData = res;
		} catch (err) {
			console.error('Failed to load preview:', err);
		} finally {
			previewLoading = false;
		}
	}

	async function sendTestEmail() {
		const type = activeTab === 'sale' ? 'sale_summary' : 'intake_recap';
		testSending = true;
		testSuccessMessage = '';
		testErrorMessage = '';
		try {
			const res = await pb.send<{ success: boolean; recipient: string; subject: string }>(
				'/api/admin/emails/send-test',
				{
					method: 'POST',
					body: {
						type,
						payoutOption: previewPayoutOption
					}
				}
			);
			testSuccessMessage = `Testovací e-mail byl úspěšně odeslán na adresu ${res.recipient}. Zkontroluj prosím svou schránku.`;
		} catch (err: any) {
			testErrorMessage = 'Nepodařilo se odeslat testovací e-mail: ' + (err?.message || err);
		} finally {
			testSending = false;
		}
	}

	function openBulkModal(type: 'intake_recap' | 'sale_summary') {
		bulkActionType = type;
		bulkResult = null;
		showConfirmModal = true;
	}

	async function executeBulkSend() {
		isBulkSending = true;
		bulkResult = null;
		const endpoint =
			bulkActionType === 'intake_recap'
				? '/api/admin/emails/send-intake-recap'
				: '/api/admin/emails/send-sale-summary';

		try {
			const res = await pb.send<{
				success: boolean;
				totalSent: number;
				totalErrors: number;
				errors: string[];
			}>(endpoint, {
				method: 'POST'
			});
			bulkResult = res;
			await loadStats();
		} catch (err: any) {
			bulkResult = {
				success: false,
				totalSent: 0,
				totalErrors: 1,
				errors: [err?.message || String(err)]
			};
		} finally {
			isBulkSending = false;
		}
	}

	$effect(() => {
		if (activeTab === 'intake' || activeTab === 'sale') {
			loadPreview();
		}
	});

	$effect(() => {
		if (previewPayoutOption) {
			loadPreview();
		}
	});

	onMount(() => {
		loadStats();
		loadTemplates();
	});
</script>

<svelte:head>
	<title>Hromadné e-maily | Pokladna</title>
</svelte:head>

<div class="min-h-[calc(100vh-65px)] bg-neutral-50 p-4 sm:p-6 pb-20">
	<div class="max-w-5xl mx-auto space-y-6">
		<!-- Header -->
		<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-white border-2 border-black p-4 sm:p-5 shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]">
			<div>
				<div class="flex items-center gap-2">
					<Mail class="w-6 h-6 text-black" />
					<h1 class="text-xl sm:text-2xl font-black uppercase tracking-tight text-black">
						Hromadné e-maily
					</h1>
				</div>
				<p class="text-xs sm:text-sm text-neutral-600 mt-1">
					Správa a rozesílání rekapitulačních e-mailů a závěrečného vyúčtování prodejcům
				</p>
			</div>

			<div class="flex items-center gap-2">
				{#if stats?.activeEvent}
					<div class="border-2 border-black bg-neutral-100 px-3 py-1 text-xs font-black uppercase tracking-wider">
						{stats.activeEvent.name}
					</div>
				{/if}
				<button
					type="button"
					onclick={() => {
						loadStats();
						loadPreview();
					}}
					class="p-2 border-2 border-black bg-white hover:bg-neutral-100 active:scale-95 transition cursor-pointer"
					title="Obnovit data"
				>
					<RefreshCw class="w-4 h-4 text-black {statsLoading ? 'animate-spin' : ''}" />
				</button>
			</div>
		</div>

		<!-- Navigation Tabs -->
		<div class="grid grid-cols-3 gap-2 sm:gap-3">
			<button
				type="button"
				onclick={() => (activeTab = 'intake')}
				class="p-3 sm:p-4 border-2 border-black text-left cursor-pointer transition-all flex flex-col justify-between {activeTab === 'intake'
					? 'bg-black text-white shadow-[3px_3px_0px_0px_rgba(0,0,0,1)]'
					: 'bg-white text-black hover:bg-neutral-100'}"
			>
				<div class="text-[10px] sm:text-xs font-mono font-bold uppercase opacity-80 mb-1">1. Fáze příjmu</div>
				<div class="text-xs sm:text-sm font-black uppercase tracking-tight">Potvrzení příjmu</div>
			</button>

			<button
				type="button"
				onclick={() => (activeTab = 'sale')}
				class="p-3 sm:p-4 border-2 border-black text-left cursor-pointer transition-all flex flex-col justify-between {activeTab === 'sale'
					? 'bg-black text-white shadow-[3px_3px_0px_0px_rgba(0,0,0,1)]'
					: 'bg-white text-black hover:bg-neutral-100'}"
			>
				<div class="text-[10px] sm:text-xs font-mono font-bold uppercase opacity-80 mb-1">2. Fáze ukončení</div>
				<div class="text-xs sm:text-sm font-black uppercase tracking-tight">Vyúčtování po prodeji</div>
			</button>

			<button
				type="button"
				onclick={() => (activeTab = 'templates')}
				class="p-3 sm:p-4 border-2 border-black text-left cursor-pointer transition-all flex flex-col justify-between {activeTab === 'templates'
					? 'bg-black text-white shadow-[3px_3px_0px_0px_rgba(0,0,0,1)]'
					: 'bg-white text-black hover:bg-neutral-100'}"
			>
				<div class="text-[10px] sm:text-xs font-mono font-bold uppercase opacity-80 mb-1">Nastavení</div>
				<div class="text-xs sm:text-sm font-black uppercase tracking-tight flex items-center gap-1.5">
					<Settings class="w-3.5 h-3.5" />
					<span>Šablony e-mailů</span>
				</div>
			</button>
		</div>

		<!-- TAB 1: POTVRZENÍ PŘÍJMU UČEBNIC -->
		{#if activeTab === 'intake'}
			<div class="space-y-6">
				<!-- Metrics Bar -->
				<div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
					<div class="bg-white border-2 border-black p-3.5 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
						<div class="text-[10px] font-black uppercase tracking-wider text-neutral-500">Prodejců k oslovení</div>
						<div class="text-2xl font-black text-black mt-1">{stats?.totalSellers ?? '—'}</div>
						<div class="text-[11px] text-neutral-500 mt-0.5">přihlásili ≥ 1 knihu</div>
					</div>

					<div class="bg-white border-2 border-black p-3.5 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
						<div class="text-[10px] font-black uppercase tracking-wider text-neutral-500">Knih celkem v burze</div>
						<div class="text-2xl font-black text-black mt-1">{stats?.totalBooks ?? '—'}</div>
						<div class="text-[11px] text-neutral-500 mt-0.5">v aktivní burze</div>
					</div>

					<div class="bg-white border-2 border-black p-3.5 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
						<div class="text-[10px] font-black uppercase tracking-wider text-emerald-700">Přijato k prodeji</div>
						<div class="text-2xl font-black text-emerald-700 mt-1">{stats?.acceptedBooks ?? '—'}</div>
						<div class="text-[11px] text-emerald-600 mt-0.5">schválené pokladnou</div>
					</div>

					<div class="bg-white border-2 border-black p-3.5 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
						<div class="text-[10px] font-black uppercase tracking-wider text-amber-700">Nepřijaté knihy</div>
						<div class="text-2xl font-black text-amber-700 mt-1">{stats?.unacceptedBooks ?? '—'}</div>
						<div class="text-[11px] text-amber-600 mt-0.5">zobrazí varování</div>
					</div>
				</div>

				<!-- Information Box -->
				<div class="bg-amber-50 border-2 border-amber-500 p-4 text-xs sm:text-sm text-amber-950 space-y-1">
					<div class="font-black uppercase flex items-center gap-1.5 text-amber-900">
						<AlertCircle class="w-4 h-4 shrink-0" />
						<span>Pravidla odesílání potvrzení</span>
					</div>
					<p>
						E-mail se odešle <strong>pouze prodejcům</strong>, kteří v aktivní burze nabídli alespoň jednu učebnici.
						Každý prodejce uvidí fotky a ceny svých knih (bez technických kódů) a <strong>výhradně zvolený způsob výplaty</strong> (bankovní účet nebo hotovost, bez zmínky o druhé možnosti). Pokud má student nepřijatou učebnici, e-mail navíc obsahuje výrazné upozornění.
					</p>
				</div>

				<!-- Action Controls & Live Preview -->
				<div class="bg-white border-2 border-black p-5 shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] space-y-4">
					<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-4 border-b-2 border-neutral-100">
						<div>
							<h2 class="text-base font-black uppercase tracking-tight text-black">
								Náhled e-mailu & Ověření
							</h2>
							<p class="text-xs text-neutral-500">
								Zkontrolujte vzhled e-mailu a před hromadnou rozesílkou si jej zašlete na test.
							</p>
						</div>

						<!-- Variant Switcher -->
						<div class="flex items-center gap-1 border-2 border-black p-0.5 bg-neutral-100 shrink-0">
							<button
								type="button"
								onclick={() => (previewPayoutOption = 'bank')}
								class="px-2.5 py-1 text-xs font-black uppercase transition-colors cursor-pointer flex items-center gap-1 {previewPayoutOption === 'bank'
									? 'bg-black text-white'
									: 'text-black hover:bg-neutral-200'}"
							>
								<CreditCard class="w-3.5 h-3.5" />
								<span>Účet</span>
							</button>
							<button
								type="button"
								onclick={() => (previewPayoutOption = 'cash')}
								class="px-2.5 py-1 text-xs font-black uppercase transition-colors cursor-pointer flex items-center gap-1 {previewPayoutOption === 'cash'
									? 'bg-black text-white'
									: 'text-black hover:bg-neutral-200'}"
							>
								<Banknote class="w-3.5 h-3.5" />
								<span>Hotovost</span>
							</button>
						</div>
					</div>

					<!-- Test send status messages -->
					{#if testSuccessMessage}
						<div class="bg-emerald-50 border-2 border-emerald-600 p-3 text-xs font-bold text-emerald-800 flex items-center justify-between gap-2">
							<div class="flex items-center gap-2">
								<CheckCircle2 class="w-4 h-4 shrink-0 text-emerald-600" />
								<span>{testSuccessMessage}</span>
							</div>
							<button type="button" onclick={() => (testSuccessMessage = '')} class="cursor-pointer">
								<X class="w-4 h-4" />
							</button>
						</div>
					{/if}
					{#if testErrorMessage}
						<div class="bg-red-50 border-2 border-red-600 p-3 text-xs font-bold text-red-800 flex items-center justify-between gap-2">
							<div class="flex items-center gap-2">
								<AlertTriangle class="w-4 h-4 shrink-0 text-red-600" />
								<span>{testErrorMessage}</span>
							</div>
							<button type="button" onclick={() => (testErrorMessage = '')} class="cursor-pointer">
								<X class="w-4 h-4" />
							</button>
						</div>
					{/if}

					<!-- Preview container -->
					<div class="border-2 border-black bg-neutral-100 overflow-hidden">
						<div class="bg-black text-white px-3 py-1.5 text-xs font-mono font-bold flex items-center justify-between">
							<span class="truncate">Předmět: {previewData?.subject || 'Načítání...'}</span>
							<span class="text-[10px] text-neutral-400 shrink-0 ml-2">ŽIVÝ NÁHLED</span>
						</div>
						<div class="p-2 sm:p-4 bg-neutral-200 flex justify-center">
							{#if previewLoading}
								<div class="py-16 text-center text-xs font-mono font-bold text-neutral-600 flex flex-col items-center gap-2">
									<RefreshCw class="w-5 h-5 animate-spin" />
									<span>Generování náhledu...</span>
								</div>
							{:else if previewData?.html}
								<iframe
									title="E-mail Preview"
									srcdoc={previewData.html}
									class="w-full max-w-[620px] h-[550px] bg-white border-2 border-black shadow-[3px_3px_0px_0px_rgba(0,0,0,0.5)]"
								></iframe>
							{/if}
						</div>
					</div>

					<!-- Action Buttons -->
					<div class="pt-2 flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
						<button
							type="button"
							onclick={sendTestEmail}
							disabled={testSending}
							class="px-4 py-2.5 border-2 border-black bg-white hover:bg-neutral-100 active:scale-95 transition font-black text-xs uppercase tracking-wider flex items-center justify-center gap-2 cursor-pointer disabled:opacity-50"
						>
							<Send class="w-3.5 h-3.5" />
							<span>{testSending ? 'Odesílání testu...' : `Odeslat test na ${auth.user?.email || 'můj e-mail'}`}</span>
						</button>

						<button
							type="button"
							onclick={() => openBulkModal('intake_recap')}
							disabled={!stats || stats.totalSellers === 0}
							class="px-6 py-3 border-2 border-black bg-emerald-500 hover:bg-emerald-400 text-black shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] active:scale-95 transition font-black text-xs uppercase tracking-wider flex items-center justify-center gap-2 cursor-pointer disabled:opacity-50"
						>
							<Mail class="w-4 h-4" />
							<span>Hromadně odeslat potvrzení ({stats?.totalSellers ?? 0} prodejců)</span>
						</button>
					</div>
				</div>
			</div>
		{/if}

		<!-- TAB 2: ZÁVĚREČNÉ VYÚČTOVÁNÍ PO PRODEJI -->
		{#if activeTab === 'sale'}
			<div class="space-y-6">
				<!-- Metrics Bar -->
				<div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
					<div class="bg-white border-2 border-black p-3.5 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
						<div class="text-[10px] font-black uppercase tracking-wider text-neutral-500">Prodejců k vyúčtování</div>
						<div class="text-2xl font-black text-black mt-1">{stats?.totalSellers ?? '—'}</div>
						<div class="text-[11px] text-neutral-500 mt-0.5">s přijatými knihami</div>
					</div>

					<div class="bg-white border-2 border-black p-3.5 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
						<div class="text-[10px] font-black uppercase tracking-wider text-emerald-700">Prodaných knih</div>
						<div class="text-2xl font-black text-emerald-700 mt-1">{stats?.soldBooks ?? '—'}</div>
						<div class="text-[11px] text-emerald-600 mt-0.5">stav: zaplaceno</div>
					</div>

					<div class="bg-white border-2 border-black p-3.5 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
						<div class="text-[10px] font-black uppercase tracking-wider text-neutral-500">Neprodaných knih</div>
						<div class="text-2xl font-black text-neutral-700 mt-1">
							{stats ? stats.acceptedBooks - stats.soldBooks : '—'}
						</div>
						<div class="text-[11px] text-neutral-500 mt-0.5">k vyzvednutí</div>
					</div>

					<div class="bg-white border-2 border-black p-3.5 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
						<div class="text-[10px] font-black uppercase tracking-wider text-emerald-700">K vyplacení celkem</div>
						<div class="text-2xl font-black text-emerald-700 mt-1">
							{stats ? `${stats.totalPayout.toLocaleString('cs-CZ')} Kč` : '—'}
						</div>
						<div class="text-[11px] text-emerald-600 mt-0.5">součet prodaných knih</div>
					</div>
				</div>

				<!-- Information Box -->
				<div class="bg-blue-50 border-2 border-blue-500 p-4 text-xs sm:text-sm text-blue-950 space-y-1">
					<div class="font-black uppercase flex items-center gap-1.5 text-blue-900">
						<AlertCircle class="w-4 h-4 shrink-0" />
						<span>Pravidla závěrečného vyúčtování</span>
					</div>
					<p>
						Tento e-mail se posílá <strong>až po oficiálním skončení prodeje na burze</strong>.
						Každý prodejce v něm najde přesný přehled svých knih se štítky <strong>PRODÁNO</strong> (zelený) nebo <strong>NEPRODÁNO</strong> (šedý), celkovou částku k výplatě a instrukce (vyplacení na účet vs. předání v hotovosti při vrácení neprodaných knih).
					</p>
				</div>

				<!-- Action Controls & Live Preview -->
				<div class="bg-white border-2 border-black p-5 shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] space-y-4">
					<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-4 border-b-2 border-neutral-100">
						<div>
							<h2 class="text-base font-black uppercase tracking-tight text-black">
								Náhled vyúčtování & Test
							</h2>
							<p class="text-xs text-neutral-500">
								Zkontrolujte texty a kalkulaci v e-mailu.
							</p>
						</div>

						<!-- Variant Switcher -->
						<div class="flex items-center gap-1 border-2 border-black p-0.5 bg-neutral-100 shrink-0">
							<button
								type="button"
								onclick={() => (previewPayoutOption = 'bank')}
								class="px-2.5 py-1 text-xs font-black uppercase transition-colors cursor-pointer flex items-center gap-1 {previewPayoutOption === 'bank'
									? 'bg-black text-white'
									: 'text-black hover:bg-neutral-200'}"
							>
								<CreditCard class="w-3.5 h-3.5" />
								<span>Účet</span>
							</button>
							<button
								type="button"
								onclick={() => (previewPayoutOption = 'cash')}
								class="px-2.5 py-1 text-xs font-black uppercase transition-colors cursor-pointer flex items-center gap-1 {previewPayoutOption === 'cash'
									? 'bg-black text-white'
									: 'text-black hover:bg-neutral-200'}"
							>
								<Banknote class="w-3.5 h-3.5" />
								<span>Hotovost</span>
							</button>
						</div>
					</div>

					{#if testSuccessMessage}
						<div class="bg-emerald-50 border-2 border-emerald-600 p-3 text-xs font-bold text-emerald-800 flex items-center justify-between gap-2">
							<div class="flex items-center gap-2">
								<CheckCircle2 class="w-4 h-4 shrink-0 text-emerald-600" />
								<span>{testSuccessMessage}</span>
							</div>
							<button type="button" onclick={() => (testSuccessMessage = '')} class="cursor-pointer">
								<X class="w-4 h-4" />
							</button>
						</div>
					{/if}
					{#if testErrorMessage}
						<div class="bg-red-50 border-2 border-red-600 p-3 text-xs font-bold text-red-800 flex items-center justify-between gap-2">
							<div class="flex items-center gap-2">
								<AlertTriangle class="w-4 h-4 shrink-0 text-red-600" />
								<span>{testErrorMessage}</span>
							</div>
							<button type="button" onclick={() => (testErrorMessage = '')} class="cursor-pointer">
								<X class="w-4 h-4" />
							</button>
						</div>
					{/if}

					<!-- Preview container -->
					<div class="border-2 border-black bg-neutral-100 overflow-hidden">
						<div class="bg-black text-white px-3 py-1.5 text-xs font-mono font-bold flex items-center justify-between">
							<span class="truncate">Předmět: {previewData?.subject || 'Načítání...'}</span>
							<span class="text-[10px] text-neutral-400 shrink-0 ml-2">ŽIVÝ NÁHLED</span>
						</div>
						<div class="p-2 sm:p-4 bg-neutral-200 flex justify-center">
							{#if previewLoading}
								<div class="py-16 text-center text-xs font-mono font-bold text-neutral-600 flex flex-col items-center gap-2">
									<RefreshCw class="w-5 h-5 animate-spin" />
									<span>Generování náhledu...</span>
								</div>
							{:else if previewData?.html}
								<iframe
									title="E-mail Preview"
									srcdoc={previewData.html}
									class="w-full max-w-[620px] h-[550px] bg-white border-2 border-black shadow-[3px_3px_0px_0px_rgba(0,0,0,0.5)]"
								></iframe>
							{/if}
						</div>
					</div>

					<!-- Action Buttons -->
					<div class="pt-2 flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
						<button
							type="button"
							onclick={sendTestEmail}
							disabled={testSending}
							class="px-4 py-2.5 border-2 border-black bg-white hover:bg-neutral-100 active:scale-95 transition font-black text-xs uppercase tracking-wider flex items-center justify-center gap-2 cursor-pointer disabled:opacity-50"
						>
							<Send class="w-3.5 h-3.5" />
							<span>{testSending ? 'Odesílání testu...' : `Odeslat test na ${auth.user?.email || 'můj e-mail'}`}</span>
						</button>

						<button
							type="button"
							onclick={() => openBulkModal('sale_summary')}
							disabled={!stats || stats.totalSellers === 0}
							class="px-6 py-3 border-2 border-black bg-black hover:bg-neutral-800 text-white shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] active:scale-95 transition font-black text-xs uppercase tracking-wider flex items-center justify-center gap-2 cursor-pointer disabled:opacity-50"
						>
							<Mail class="w-4 h-4 text-emerald-400" />
							<span>Hromadně odeslat vyúčtování ({stats?.totalSellers ?? 0} prodejců)</span>
						</button>
					</div>
				</div>
			</div>
		{/if}

		<!-- TAB 3: ŠABLONY E-MAILŮ -->
		{#if activeTab === 'templates'}
			<div class="bg-white border-2 border-black p-5 sm:p-6 shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] space-y-6">
				<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-4 border-b-2 border-neutral-200">
					<div>
						<h2 class="text-base sm:text-lg font-black uppercase tracking-tight text-black flex items-center gap-2">
							<Settings class="w-5 h-5" />
							<span>Editor e-mailových šablon</span>
						</h2>
						<p class="text-xs text-neutral-600 mt-0.5">
							Změny se ukládají přímo do PocketBase kolekce <code class="font-mono bg-neutral-100 px-1 border border-neutral-300">email_templates</code> a okamžitě se projeví při odesílání.
						</p>
					</div>

					<div class="flex items-center gap-2">
						<button
							type="button"
							onclick={() => {
								selectedTemplateKey = 'intake_recap';
								updateTemplateForm();
							}}
							class="px-3 py-1.5 text-xs font-black uppercase border-2 border-black transition cursor-pointer {selectedTemplateKey === 'intake_recap'
								? 'bg-black text-white'
								: 'bg-white text-black hover:bg-neutral-100'}"
						>
							Potvrzení příjmu
						</button>
						<button
							type="button"
							onclick={() => {
								selectedTemplateKey = 'sale_summary';
								updateTemplateForm();
							}}
							class="px-3 py-1.5 text-xs font-black uppercase border-2 border-black transition cursor-pointer {selectedTemplateKey === 'sale_summary'
								? 'bg-black text-white'
								: 'bg-white text-black hover:bg-neutral-100'}"
						>
							Vyúčtování po prodeji
						</button>
					</div>
				</div>

				<!-- Available variables badge reference -->
				<div class="bg-neutral-100 border border-neutral-300 p-3 text-xs space-y-1">
					<div class="font-bold uppercase tracking-wider text-neutral-700 text-[11px]">Dostupné proměnné k dosazení:</div>
					<div class="flex flex-wrap gap-1.5 pt-1">
						<span class="font-mono bg-white border border-neutral-300 px-1.5 py-0.5">{'{name}'} (jméno studenta)</span>
						<span class="font-mono bg-white border border-neutral-300 px-1.5 py-0.5">{'{eventName}'} (název burzy)</span>
						<span class="font-mono bg-white border border-neutral-300 px-1.5 py-0.5">{'{email}'} (e-mail)</span>
						<span class="font-mono bg-white border border-neutral-300 px-1.5 py-0.5">{'{iban}'} (bankovní účet)</span>
						{#if selectedTemplateKey === 'sale_summary'}
							<span class="font-mono bg-white border border-neutral-300 px-1.5 py-0.5">{'{totalPayout}'} (částka k vyplacení)</span>
							<span class="font-mono bg-white border border-neutral-300 px-1.5 py-0.5">{'{soldCount}'} (počet prodaných knih)</span>
							<span class="font-mono bg-white border border-neutral-300 px-1.5 py-0.5">{'{unsoldCount}'} (počet neprodaných)</span>
						{/if}
					</div>
				</div>

				<!-- Form fields -->
				<form
					onsubmit={(e) => {
						e.preventDefault();
						saveCurrentTemplate();
					}}
					class="space-y-4"
				>
					<div>
						<label for="tmpl-name" class="block text-xs font-black uppercase tracking-wider text-black mb-1">
							Interní název šablony
						</label>
						<input
							id="tmpl-name"
							type="text"
							bind:value={templateForm.name}
							class="w-full border-2 border-black p-2 text-sm bg-white font-medium focus:bg-neutral-50 outline-none"
							required
						/>
					</div>

					<div>
						<label for="tmpl-subject" class="block text-xs font-black uppercase tracking-wider text-black mb-1">
							Předmět e-mailu (Subject)
						</label>
						<input
							id="tmpl-subject"
							type="text"
							bind:value={templateForm.subject}
							class="w-full border-2 border-black p-2 text-sm bg-white font-medium focus:bg-neutral-50 outline-none font-mono"
							required
						/>
					</div>

					<div>
						<label for="tmpl-intro" class="block text-xs font-black uppercase tracking-wider text-black mb-1">
							Úvodní text (před přehledem knih)
						</label>
						<textarea
							id="tmpl-intro"
							rows="3"
							bind:value={templateForm.bodyIntro}
							class="w-full border-2 border-black p-2 text-sm bg-white font-medium focus:bg-neutral-50 outline-none"
						></textarea>
						<span class="text-[11px] text-neutral-500">Můžete použít jednoduché HTML tagy jako &lt;br&gt; nebo &lt;strong&gt;.</span>
					</div>

					{#if selectedTemplateKey === 'intake_recap'}
						<div>
							<label for="tmpl-unaccepted" class="block text-xs font-black uppercase tracking-wider text-amber-800 mb-1">
								Text upozornění na nepřijaté knihy (zobrazí se pouze, má-li student nepřijatou knihu)
							</label>
							<textarea
								id="tmpl-unaccepted"
								rows="3"
								bind:value={templateForm.unacceptedWarning}
								class="w-full border-2 border-amber-600 p-2 text-sm bg-amber-50/50 font-medium focus:bg-amber-50 outline-none"
							></textarea>
						</div>
					{/if}

					<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
						<div class="border-2 border-neutral-300 p-3.5 bg-neutral-50 space-y-1.5">
							<label for="tmpl-bank" class="block text-xs font-black uppercase tracking-wider text-black">
								Text pro výplatu na účet
							</label>
							<textarea
								id="tmpl-bank"
								rows="3"
								bind:value={templateForm.payoutBankNote}
								class="w-full border-2 border-black p-2 text-xs bg-white font-medium outline-none"
							></textarea>
							<span class="text-[10px] text-neutral-500">Zobrazí se VÝHRADNĚ uživatelům s bankovním účtem.</span>
						</div>

						<div class="border-2 border-neutral-300 p-3.5 bg-neutral-50 space-y-1.5">
							<label for="tmpl-cash" class="block text-xs font-black uppercase tracking-wider text-black">
								Text pro výplatu v hotovosti
							</label>
							<textarea
								id="tmpl-cash"
								rows="3"
								bind:value={templateForm.payoutCashNote}
								class="w-full border-2 border-black p-2 text-xs bg-white font-medium outline-none"
							></textarea>
							<span class="text-[10px] text-neutral-500">Zobrazí se VÝHRADNĚ uživatelům s hotovostí.</span>
						</div>
					</div>

					<div>
						<label for="tmpl-outro" class="block text-xs font-black uppercase tracking-wider text-black mb-1">
							Závěrečný text a kontakty (Outro)
						</label>
						<textarea
							id="tmpl-outro"
							rows="3"
							bind:value={templateForm.bodyOutro}
							class="w-full border-2 border-black p-2 text-sm bg-white font-medium focus:bg-neutral-50 outline-none"
						></textarea>
					</div>

					<div class="pt-2 flex items-center justify-between">
						{#if templateSaveSuccess}
							<div class="text-xs font-bold text-emerald-700 flex items-center gap-1.5">
								<Check class="w-4 h-4" />
								<span>Šablona byla úspěšně uložena a aktualizována!</span>
							</div>
						{:else}
							<div></div>
						{/if}

						<button
							type="submit"
							disabled={savingTemplate}
							class="px-6 py-2.5 border-2 border-black bg-black text-white hover:bg-neutral-800 font-black text-xs uppercase tracking-wider shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] active:scale-95 transition cursor-pointer disabled:opacity-50"
						>
							{savingTemplate ? 'Ukládání...' : 'Uložit šablonu'}
						</button>
					</div>
				</form>
			</div>
		{/if}
	</div>
</div>

<!-- CONFIRMATION & BULK SEND MODAL -->
{#if showConfirmModal}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs">
		<div class="w-full max-w-lg bg-white border-4 border-black p-6 shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] space-y-5 animate-in fade-in zoom-in-95 duration-150">
			<!-- Modal Header -->
			<div class="flex items-start justify-between gap-3">
				<div class="flex items-center gap-2 text-red-600">
					<AlertTriangle class="w-6 h-6 shrink-0" />
					<h3 class="text-lg font-black uppercase tracking-tight text-black">
						Potvrzení hromadného odeslání
					</h3>
				</div>
				{#if !isBulkSending}
					<button
						type="button"
						onclick={() => (showConfirmModal = false)}
						class="p-1 border border-black hover:bg-neutral-100 cursor-pointer"
					>
						<X class="w-4 h-4" />
					</button>
				{/if}
			</div>

			{#if isBulkSending}
				<!-- Sending in progress state -->
				<div class="py-8 text-center space-y-3">
					<RefreshCw class="w-8 h-8 mx-auto animate-spin text-black" />
					<div class="text-sm font-black uppercase tracking-wider text-black">
						Probíhá hromadné odesílání e-mailů...
					</div>
					<p class="text-xs text-neutral-500">
						E-maily jsou postupně doručovány přes SMTP server. Nezavírejte toto okno.
					</p>
				</div>
			{:else if bulkResult}
				<!-- Result state -->
				<div class="space-y-4">
					<div class="p-4 border-2 {bulkResult.totalErrors === 0 ? 'border-emerald-600 bg-emerald-50 text-emerald-900' : 'border-amber-600 bg-amber-50 text-amber-950'}">
						<div class="font-black text-sm uppercase flex items-center gap-1.5 mb-1">
							{#if bulkResult.totalErrors === 0}
								<CheckCircle2 class="w-5 h-5 text-emerald-600" />
								<span>Odesílání úspěšně dokončeno!</span>
							{:else}
								<AlertCircle class="w-5 h-5 text-amber-600" />
								<span>Odesílání dokončeno s chybami</span>
							{/if}
						</div>
						<div class="text-xs space-y-0.5">
							<div>Úspěšně doručeno: <strong>{bulkResult.totalSent} e-mailů</strong></div>
							{#if bulkResult.totalErrors > 0}
								<div>Chyby: <strong>{bulkResult.totalErrors}</strong></div>
							{/if}
						</div>
					</div>

					{#if bulkResult.errors && bulkResult.errors.length > 0}
						<div class="max-h-36 overflow-y-auto border-2 border-black p-2 bg-neutral-100 text-[11px] font-mono space-y-1">
							{#each bulkResult.errors as err}
								<div class="text-red-700">• {err}</div>
							{/each}
						</div>
					{/if}

					<div class="flex justify-end pt-2">
						<button
							type="button"
							onclick={() => (showConfirmModal = false)}
							class="px-5 py-2 border-2 border-black bg-black text-white font-black text-xs uppercase tracking-wider cursor-pointer"
						>
							Zavřít
						</button>
					</div>
				</div>
			{:else}
				<!-- Prompt Confirmation -->
				<div class="space-y-3 text-xs sm:text-sm text-neutral-700">
					<p>
						Opravdu chcete odeslat
						<strong>
							{bulkActionType === 'intake_recap' ? 'Potvrzení příjmu učebnic' : 'Závěrečné vyúčtování po prodeji'}
						</strong>
						všem <strong>{stats?.totalSellers ?? 0} prodejcům</strong> v aktivní burze?
					</p>
					<div class="bg-neutral-100 border border-neutral-300 p-3 space-y-1 text-xs">
						<div>Akce: <strong>{stats?.activeEvent?.name ?? 'Aktivní burza'}</strong></div>
						<div>Počet příjemců: <strong>{stats?.totalSellers ?? 0} prodejců</strong></div>
						<div>Typ zprávy: <strong>{bulkActionType === 'intake_recap' ? 'Potvrzení příjmu (fotky, ceny, způsob výplaty)' : 'Vyúčtování (prodáno/neprodáno, částka k vyplacení)'}</strong></div>
					</div>
					<p class="text-[11px] text-red-600 font-bold">
						⚠️ Tato akce rozešle skutečné e-maily do schránek studentů. Ujistěte se, že jste předtím otestovali vzhled přes testovací e-mail!
					</p>
				</div>

				<div class="flex items-center justify-end gap-3 pt-2">
					<button
						type="button"
						onclick={() => (showConfirmModal = false)}
						class="px-4 py-2 border-2 border-black bg-white hover:bg-neutral-100 font-black text-xs uppercase tracking-wider cursor-pointer"
					>
						Zrušit
					</button>
					<button
						type="button"
						onclick={executeBulkSend}
						class="px-5 py-2 border-2 border-black bg-red-600 hover:bg-red-700 text-white font-black text-xs uppercase tracking-wider shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] active:scale-95 transition cursor-pointer"
					>
						Ano, odeslat {stats?.totalSellers ?? 0} e-mailů
					</button>
				</div>
			{/if}
		</div>
	</div>
{/if}
