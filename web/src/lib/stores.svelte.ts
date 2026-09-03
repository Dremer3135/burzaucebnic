import { pb } from './pocketbase';
import type { User, Event, Book, Payment } from './types';

// ==========================================
// 1. AUTH STORE
// ==========================================
class AuthStore {
	user = $state<User | null>(pb.authStore.model as unknown as User | null);
	isCashier = $derived(this.user?.isCashier ?? false);
	private unsubUser: (() => void) | null = null;

	constructor() {
		if (typeof window !== 'undefined') {
			pb.authStore.onChange(() => {
				this.user = pb.authStore.model as unknown as User | null;
				this.setupUserSubscription();
			});
			this.setupUserSubscription();
		}
	}

	private async setupUserSubscription() {
		if (this.unsubUser) {
			this.unsubUser();
			this.unsubUser = null;
		}

		if (this.user?.id) {
			try {
				this.unsubUser = await pb.collection('users').subscribe<User>(this.user.id, (e) => {
					if (e.action === 'update') {
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
		return this.user;
	}

	async loginWithGoogle() {
		const res = await pb.collection('users').authWithOAuth2({ provider: 'google' });
		this.user = res.record as unknown as User;
		return this.user;
	}

	logout() {
		if (this.unsubUser) {
			this.unsubUser();
			this.unsubUser = null;
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

	// Status helpers relative to current time
	isSellActive(): boolean {
		if (!this.event || !this.event.active) return false;
		const now = new Date().getTime();
		const start = this.event.sellStart ? new Date(this.event.sellStart).getTime() : 0;
		const end = this.event.sellEnd ? new Date(this.event.sellEnd).getTime() : Infinity;
		return now >= start && now <= end;
	}

	isBuyActive(): boolean {
		if (!this.event || !this.event.active) return false;
		const now = new Date().getTime();
		const start = this.event.buyStart ? new Date(this.event.buyStart).getTime() : 0;
		const end = this.event.buyEnd ? new Date(this.event.buyEnd).getTime() : Infinity;
		return now >= start && now <= end;
	}

	isMarketClosed(): boolean {
		return !this.isSellActive() && !this.isBuyActive();
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

	async init(userId: string) {
		if (this.currentUserId === userId && this.books.length > 0) return;
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

	async init() {
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
		try {
			this.unsub = await pb.collection('payments').subscribe<Payment>('*', async (e) => {
				if (e.action === 'create' || e.action === 'update') {
					// Fetch fresh with expand
					try {
						const fresh = await pb.collection('payments').getOne<Payment>(e.record.id, {
							expand: 'buyer,books'
						});
						const idx = this.payments.findIndex((p) => p.id === fresh.id);
						if (idx !== -1) {
							this.payments[idx] = fresh;
						} else {
							this.payments = [fresh, ...this.payments];
						}
					} catch (err) {
						this.refresh();
					}
				} else if (e.action === 'delete') {
					this.payments = this.payments.filter((p) => p.id !== e.record.id);
				}
			});
		} catch (err) {
			console.warn('Could not subscribe to payments collection', err);
		}
	}

	cleanup() {
		if (this.unsub) {
			this.unsub();
			this.unsub = null;
		}
		this.payments = [];
	}
}

export const cashierPayments = new CashierPaymentsStore();
