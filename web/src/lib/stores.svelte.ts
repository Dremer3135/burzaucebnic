import { pb } from './pocketbase';
import type { User, Event, Book, Payment } from './types';

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
				if (this.user?.id !== newUser?.id) {
					this.user = newUser;
					this.setupUserSubscription();
				}
			});
			this.setupUserSubscription();
		}
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
				sort: '-created'
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
				sort: '-created'
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
				sort: '-created'
			});
			this.payments = res;
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

class PriceStore {
	private cache = new Map<string, CachedPrice | null>();
	private inFlight = new Map<string, Promise<CachedPrice | null>>();

	get(id: string): CachedPrice | null | undefined {
		return this.cache.get(id);
	}

	has(id: string): boolean {
		return this.cache.has(id);
	}

	set(id: string, data: CachedPrice | null) {
		this.cache.set(id, data);
	}

	isUsed(id: string): boolean | null {
		if (!this.cache.has(id)) return null;
		return this.cache.get(id) !== null;
	}

	async fetchPrice(id: string): Promise<CachedPrice | null> {
		if (this.cache.has(id)) {
			return this.cache.get(id)!;
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
				this.cache.set(id, data);
				return data;
			} catch (err: any) {
				// Record not found (404) means code is free
				if (err?.status === 404) {
					this.cache.set(id, null);
					return null;
				}
				// On other network/auth errors, do not cache as non-existent
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

export const priceStore = new PriceStore();
