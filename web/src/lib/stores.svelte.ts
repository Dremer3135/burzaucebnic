import { pb } from './pocketbase';
import type { User, Event, Book, Payment, CodeStatus, CodeValidationResult } from './types';

// ==========================================
// 1. AUTH STORE
// ==========================================
class AuthStore {
	user = $state<User | null>(pb.authStore.model as unknown as User | null);
	isCashier = $derived(this.user?.isCashier ?? false);
	private unsubUser: (() => void) | null = null;
	private currentSubscribedUserId: string | null = null;

	constructor() {
		if (typeof window !== 'undefined') {
			pb.authStore.onChange(() => {
				const newUser = pb.authStore.model as unknown as User | null;
				const userChanged = this.user?.id !== newUser?.id;
				this.user = newUser;
				if (userChanged) {
					this.setupUserSubscription();
				}
			});
			this.setupUserSubscription();
		}
	}

	async updateProfile(data: Partial<User>) {
		if (!this.user?.id) throw new Error('Not logged in');
		const updated = await pb.collection('users').update<User>(this.user.id, data);
		pb.authStore.save(pb.authStore.token, updated as any);
		this.user = updated;
		return this.user;
	}

	private async setupUserSubscription() {
		if (this.currentSubscribedUserId === this.user?.id && this.unsubUser) {
			return;
		}

		if (this.unsubUser) {
			this.unsubUser();
			this.unsubUser = null;
			this.currentSubscribedUserId = null;
		}

		if (this.user?.id) {
			const targetId = this.user.id;
			this.currentSubscribedUserId = targetId;
			try {
				this.unsubUser = await pb.collection('users').subscribe<User>(targetId, (e) => {
					if (e.action === 'update' && e.record.id === this.user?.id) {
						this.user = e.record;
					}
				});
			} catch (err) {
				console.warn('Could not subscribe to user record updates', err);
			}
		}
	}

	async loginWithPassword(email: string, pass: string) {
		const res = await pb.collection('users').authWithPassword(email, pass);
		this.user = res.record as unknown as User;
		this.setupUserSubscription();
		return this.user;
	}

	async loginWithGoogle() {
		const res = await pb.collection('users').authWithOAuth2({ provider: 'google' });
		this.user = res.record as unknown as User;
		this.setupUserSubscription();
		return this.user;
	}

	logout() {
		if (this.unsubUser) {
			this.unsubUser();
			this.unsubUser = null;
			this.currentSubscribedUserId = null;
		}
		pb.authStore.clear();
		this.user = null;
	}
}

export const auth = new AuthStore();

// ==========================================
// 2. ACTIVE EVENT STORE
// ==========================================
class EventStore {
	event = $state<Event | null>(null);
	isLoading = $state(true);
	private unsubEvent: (() => void) | null = null;

	constructor() {
		if (typeof window !== 'undefined') {
			this.init();
		}
	}

	async init() {
		await this.fetchActive();
		this.subscribe();
	}

	async fetchActive() {
		this.isLoading = true;
		try {
			const list = await pb.collection('events').getFullList<Event>({
				filter: 'active = true',
				sort: '-id'
			});
			this.event = list.length > 0 ? list[0] : null;
		} catch (err) {
			console.error('Failed to fetch active event', err);
			this.event = null;
		} finally {
			this.isLoading = false;
		}
	}

	private async subscribe() {
		try {
			this.unsubEvent = await pb.collection('events').subscribe<Event>('*', async () => {
				await this.fetchActive();
			});
		} catch (err) {
			console.warn('Could not subscribe to events collection', err);
		}
	}

	isMarketActive(): boolean {
		return !!this.event?.active;
	}

	isMarketClosed(): boolean {
		return !this.event?.active;
	}

	isMaintenance(): boolean {
		return !!(this.event?.maintenance_break ?? (this.event as any)?.maintenence_break);
	}

	getDefaultRoute(): string {
		if (!this.event || !this.event.active) return '/';
		return this.event.defaultPage === 'seeprice' ? '/seeprice' : '/sell';
	}
}

export const eventStore = new EventStore();

// ==========================================
// 3. SELLER BOOKS STORE (For /sell view)
// ==========================================
class SellerBooksStore {
	books = $state<Book[]>([]);
	isLoading = $state(false);
	private unsub: (() => void) | null = null;
	private currentUserId: string | null = null;
	private isInitialized = false;

	async init(userId: string) {
		if (this.isInitialized && this.currentUserId === userId) return;
		this.isInitialized = true;
		this.currentUserId = userId;
		await this.refresh();
		this.subscribe();
	}

	async refresh() {
		if (!this.currentUserId) return;
		this.isLoading = true;
		try {
			const res = await pb.collection('books').getFullList<Book>({
				filter: `seller = "${this.currentUserId}"`,
				sort: '-id'
			});
			this.books = res;
		} catch (err) {
			console.error('Failed to load seller books', err);
		} finally {
			this.isLoading = false;
		}
	}

	private async subscribe() {
		if (this.unsub) {
			this.unsub();
			this.unsub = null;
		}
		try {
			this.unsub = await pb.collection('books').subscribe<Book>('*', (e) => {
				if (!this.currentUserId) return;

				if (e.action === 'create') {
					if (e.record.seller === this.currentUserId) {
						// Add to top if not already present
						if (!this.books.some((b) => b.id === e.record.id)) {
							this.books = [e.record, ...this.books];
						}
					}
				} else if (e.action === 'update') {
					if (e.record.seller === this.currentUserId) {
						this.books = this.books.map((b) => (b.id === e.record.id ? e.record : b));
					} else {
						this.books = this.books.filter((b) => b.id !== e.record.id);
					}
				} else if (e.action === 'delete') {
					this.books = this.books.filter((b) => b.id !== e.record.id);
				}
			});
		} catch (err) {
			console.warn('Could not subscribe to seller books', err);
		}
	}

	cleanup() {
		if (this.unsub) {
			this.unsub();
			this.unsub = null;
		}
		this.books = [];
		this.currentUserId = null;
		this.isInitialized = false;
	}
}

export const sellerBooks = new SellerBooksStore();

// ==========================================
// 4. CASHIER PAYMENTS STORE (For /cashier/payments)
// ==========================================
class CashierPaymentsStore {
	payments = $state<Payment[]>([]);
	isLoading = $state(false);
	private unsub: (() => void) | null = null;
	private unsubReconnect: (() => void) | null = null;
	private isInitialized = false;

	async init() {
		if (this.isInitialized) return;
		this.isInitialized = true;
		await this.refresh();
		this.subscribe();
	}

	async refresh() {
		this.isLoading = true;
		try {
			const res = await pb.collection('payments').getFullList<Payment>({
				expand: 'buyer,books',
				sort: '-id'
			});
			this.payments = res;

			// Ensure buyer emails are resolved even if emailVisibility was false in PB
			const missingBuyerIds = new Set<string>();
			for (const p of res) {
				if (p.buyer && !p.expand?.buyer?.email) {
					missingBuyerIds.add(p.buyer);
				}
			}

			if (missingBuyerIds.size > 0) {
				const resolved = await Promise.all(
					Array.from(missingBuyerIds).map(async (id) => {
						try {
							const info = await pb.send<{ type: string; user?: { id: string; name: string; email: string } }>(
								`/api/cashier/lookup-code?code=${encodeURIComponent(id)}`,
								{ method: 'GET' }
							);
							return info?.type === 'user' && info.user ? info.user : null;
						} catch {
							return null;
						}
					})
				);

				const userMap = new Map<string, { id: string; name: string; email: string }>();
				for (const u of resolved) {
					if (u) userMap.set(u.id, u);
				}

				if (userMap.size > 0) {
					this.payments = this.payments.map((p) => {
						const u = userMap.get(p.buyer);
						if (u) {
							const buyerObj = p.expand?.buyer
								? { ...p.expand.buyer, email: u.email, name: p.expand.buyer.name || u.name }
								: (u as any);
							return {
								...p,
								expand: {
									...p.expand,
									buyer: buyerObj
								}
							};
						}
						return p;
					});
				}
			}
		} catch (err) {
			console.error('Failed to load cashier payments', err);
		} finally {
			this.isLoading = false;
		}
	}

	private async subscribe() {
		if (this.unsub) {
			this.unsub();
			this.unsub = null;
		}
		if (this.unsubReconnect) {
			this.unsubReconnect();
			this.unsubReconnect = null;
		}

		try {
			// Auto-refresh when realtime reconnects (e.g. mobile wakes up from background)
			this.unsubReconnect = await pb.realtime.subscribe('PB_CONNECT', () => {
				this.refresh();
			});

			this.unsub = await pb.collection('payments').subscribe<Payment>(
				'*',
				async (e) => {
					if (e.action === 'create') {
						let record = e.record;
						if (!record.expand?.buyer || !record.expand?.books) {
							try {
								record = await pb.collection('payments').getOne<Payment>(e.record.id, {
									expand: 'buyer,books'
								});
							} catch (err) {
								console.error('Failed to expand created payment', err);
							}
						}
						if (!this.payments.some((p) => p.id === record.id)) {
							this.payments = [record, ...this.payments];
						}
					} else if (e.action === 'update') {
						let record = e.record;
						if (!record.expand?.buyer || !record.expand?.books) {
							try {
								record = await pb.collection('payments').getOne<Payment>(e.record.id, {
									expand: 'buyer,books'
								});
							} catch (err) {
								console.error('Failed to expand updated payment', err);
							}
						}
						this.payments = this.payments.map((p) => (p.id === record.id ? record : p));
					} else if (e.action === 'delete') {
						this.payments = this.payments.filter((p) => p.id !== e.record.id);
					}
				},
				{ expand: 'buyer,books' }
			);
		} catch (err) {
			console.warn('Could not subscribe to payments collection', err);
		}
	}

	cleanup() {
		if (this.unsub) {
			this.unsub();
			this.unsub = null;
		}
		if (this.unsubReconnect) {
			this.unsubReconnect();
			this.unsubReconnect = null;
		}
		this.payments = [];
		this.isInitialized = false;
	}
}

export const cashierPayments = new CashierPaymentsStore();

// ==========================================
// 5. PRICE STORE (In-memory cache for /seeprice)
// ==========================================
export interface CachedPrice {
	id: string;
	price: number;
	status: string;
}

interface CacheEntry {
	data: CachedPrice | null;
	cachedAt: number;
}

class PriceStore {
	private cache = new Map<string, CacheEntry>();
	private inFlight = new Map<string, Promise<CachedPrice | null>>();
	private unsub: (() => void) | null = null;
	private ttlMs: number;

	constructor(ttlMs = 10_000) {
		this.ttlMs = ttlMs;
		if (typeof window !== 'undefined') {
			this.subscribe();
		}
	}

	private async subscribe() {
		try {
			this.unsub = await pb.collection('books').subscribe<Book>('*', (e) => {
				if (e.action === 'create' || e.action === 'update') {
					this.cache.set(e.record.id, {
						data: {
							id: e.record.id,
							price: e.record.price,
							status: e.record.status
						},
						cachedAt: Date.now()
					});
				} else if (e.action === 'delete') {
					this.cache.set(e.record.id, {
						data: null,
						cachedAt: Date.now()
					});
				}
			});
		} catch (err) {
			console.warn('Could not subscribe to books in PriceStore', err);
		}
	}

	get(id: string): CachedPrice | null | undefined {
		const entry = this.cache.get(id);
		return entry !== undefined ? entry.data : undefined;
	}

	isFresh(id: string): boolean {
		const entry = this.cache.get(id);
		if (!entry) return false;
		return Date.now() - entry.cachedAt < this.ttlMs;
	}

	has(id: string): boolean {
		return this.isFresh(id);
	}

	hasEntry(id: string): boolean {
		return this.cache.has(id);
	}

	set(id: string, data: CachedPrice | null) {
		this.cache.set(id, {
			data,
			cachedAt: Date.now()
		});
	}

	isUsed(id: string): boolean | null {
		const entry = this.cache.get(id);
		if (!entry) return null;
		return entry.data !== null;
	}

	async fetchPrice(id: string, force = false): Promise<CachedPrice | null> {
		const entry = this.cache.get(id);
		if (!force && entry && Date.now() - entry.cachedAt < this.ttlMs) {
			return entry.data;
		}

		if (this.inFlight.has(id)) {
			return this.inFlight.get(id)!;
		}

		const promise = (async () => {
			try {
				const res = await pb.send<{ id: string; price: number; status: string }>(
					`/api/book-price?id=${encodeURIComponent(id)}`,
					{ method: 'GET' }
				);
				const data: CachedPrice = { id: res.id, price: res.price, status: res.status };
				this.cache.set(id, { data, cachedAt: Date.now() });
				return data;
			} catch (err: any) {
				// Record not found (404) or user ID (400) means not a book
				if (err?.status === 404 || err?.status === 400) {
					this.cache.set(id, { data: null, cachedAt: Date.now() });
					return null;
				}
				// On other network/auth errors, retain previous cached data if present
				if (entry) {
					return entry.data;
				}
				return null;
			} finally {
				this.inFlight.delete(id);
			}
		})();

		this.inFlight.set(id, promise);
		return promise;
	}

	clear() {
		this.cache.clear();
		this.inFlight.clear();
	}
}

export const priceStore = new PriceStore(10_000);
 
// ==========================================
// 6. BOOK CODE STORE (Validation & cache for /sell)
// ==========================================
class BookCodeStore {
	private cache = new Map<string, { data: CodeValidationResult; cachedAt: number }>();
	private inFlight = new Map<string, Promise<CodeValidationResult>>();
	private unsub: (() => void) | null = null;
	private ttlMs: number;

	constructor(ttlMs = 15_000) {
		this.ttlMs = ttlMs;
		if (typeof window !== 'undefined') {
			this.subscribe();
		}
	}

	private async subscribe() {
		try {
			this.unsub = await pb.collection('books').subscribe<Book>('*', (e) => {
				if (e.action === 'create') {
					this.set(e.record.id, 'used', 'Kód knihy je již použit.');
				} else if (e.action === 'delete') {
					this.set(e.record.id, 'available', 'Kód je volný.');
				}
			});
		} catch (err) {
			console.warn('Could not subscribe to books in BookCodeStore', err);
		}
	}

	getStatus(code: string, currentUserId?: string): CodeStatus {
		const clean = code?.trim();
		if (!clean) return 'checking';
		if (currentUserId && clean === currentUserId) return 'user';
		const entry = this.cache.get(clean);
		if (!entry || Date.now() - entry.cachedAt > this.ttlMs) return 'checking';
		return entry.data.status;
	}

	has(code: string): boolean {
		const clean = code?.trim();
		if (!clean) return false;
		const entry = this.cache.get(clean);
		if (!entry) return false;
		return Date.now() - entry.cachedAt < this.ttlMs;
	}

	set(code: string, status: 'available' | 'used' | 'user' | 'invalid', message?: string) {
		const clean = code?.trim();
		if (!clean) return;
		this.cache.set(clean, {
			data: { code: clean, status, message },
			cachedAt: Date.now()
		});
	}

	async validateCode(code: string, currentUserId?: string): Promise<CodeValidationResult> {
		const cleanCode = code.trim();
		if (!cleanCode) {
			return { code: cleanCode, status: 'checking', message: 'Prázdný kód' };
		}

		if (currentUserId && cleanCode === currentUserId) {
			const res: CodeValidationResult = {
				code: cleanCode,
				status: 'user',
				message: 'Toto je kód uživatele, nikoliv učebnice.'
			};
			this.set(cleanCode, 'user', res.message);
			return res;
		}

		const entry = this.cache.get(cleanCode);
		if (entry && Date.now() - entry.cachedAt < this.ttlMs) {
			return entry.data;
		}

		if (this.inFlight.has(cleanCode)) {
			return this.inFlight.get(cleanCode)!;
		}

		const promise = (async () => {
			try {
				const res = await pb.send<CodeValidationResult>(
					`/api/check-book-code?code=${encodeURIComponent(cleanCode)}`,
					{ method: 'GET' }
				);
				this.cache.set(cleanCode, { data: res, cachedAt: Date.now() });
				return res;
			} catch (err: any) {
				const msg = String(err?.message || '').toLowerCase();
				const errData = err?.response?.data || err?.data;
				if (
					msg.includes('uživatel') ||
					msg.includes('user') ||
					errData?.type === 'user_id' ||
					errData?.error === 'user_id' ||
					err?.response?.type === 'user_id'
				) {
					const res: CodeValidationResult = {
						code: cleanCode,
						status: 'user',
						message: 'Toto je kód uživatele, nikoliv učebnice.'
					};
					this.cache.set(cleanCode, { data: res, cachedAt: Date.now() });
					return res;
				}
				if (err?.status === 404) {
					const res: CodeValidationResult = { code: cleanCode, status: 'available' };
					this.cache.set(cleanCode, { data: res, cachedAt: Date.now() });
					return res;
				}
				const fallback: CodeValidationResult = { code: cleanCode, status: 'checking', message: err?.message };
				return fallback;
			} finally {
				this.inFlight.delete(cleanCode);
			}
		})();

		this.inFlight.set(cleanCode, promise);
		return promise;
	}

	clear() {
		this.cache.clear();
		this.inFlight.clear();
	}
}

export const bookCodeStore = new BookCodeStore();
