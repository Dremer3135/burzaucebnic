import type { Book } from './types';
import { pb } from './pocketbase';

export interface CartItem {
	book: Book;
	isAvailable: boolean; // if changed by someone else in real-time
	addedAt: number;
}

class CartStore {
	items = $state<CartItem[]>([]);
	activeSubscriptions = new Map<string, () => void>();

	constructor() {
		if (typeof window !== 'undefined') {
			this.loadFromStorage();
			pb.authStore.onChange(() => {
				this.clearSubscriptions();
				this.loadFromStorage();
			});
		}
	}

	private getStorageKey(): string {
		const userId = pb.authStore.model?.id || 'guest';
		return `burza_cart_${userId}`;
	}

	loadFromStorage() {
		try {
			const saved = localStorage.getItem(this.getStorageKey());
			if (saved) {
				const parsed = JSON.parse(saved) as CartItem[];
				this.items = parsed;
				this.refreshItemsStatus();
			} else {
				this.items = [];
			}
		} catch (err) {
			console.error('Failed to load cart from localStorage', err);
			this.items = [];
		}
	}

	saveToStorage() {
		if (typeof window === 'undefined') return;
		try {
			localStorage.setItem(this.getStorageKey(), JSON.stringify(this.items));
		} catch (err) {
			console.error('Failed to save cart to localStorage', err);
		}
	}

	private clearSubscriptions() {
		for (const unsub of this.activeSubscriptions.values()) {
			unsub();
		}
		this.activeSubscriptions.clear();
	}


	async refreshItemsStatus() {
		for (const item of this.items) {
			try {
				const freshBook = await pb.collection('books').getOne<Book>(item.book.id);
				item.book = freshBook;
				item.isAvailable = freshBook.status === 'available';
				this.subscribeToBook(item.book.id);
			} catch (err) {
				// Record probably deleted or unavailable
				item.isAvailable = false;
			}
		}
		this.saveToStorage();
	}

	subscribeToBook(bookId: string) {
		if (this.activeSubscriptions.has(bookId)) return;

		pb.collection('books')
			.subscribe<Book>(bookId, (e) => {
				const idx = this.items.findIndex((i) => i.book.id === bookId);
				if (idx !== -1) {
					if (e.action === 'delete') {
						this.items[idx].isAvailable = false;
					} else if (e.action === 'update') {
						this.items[idx].book = e.record;
						this.items[idx].isAvailable = e.record.status === 'available';
					}
					this.saveToStorage();
				}
			})
			.then((unsubscribe) => {
				this.activeSubscriptions.set(bookId, unsubscribe);
			})
			.catch((err) => {
				console.warn(`Could not subscribe to cart book ${bookId}`, err);
			});
	}

	addBook(book: Book) {
		const existing = this.items.find((i) => i.book.id === book.id);
		if (existing) return;

		this.items.push({
			book,
			isAvailable: book.status === 'available',
			addedAt: Date.now()
		});
		this.subscribeToBook(book.id);
		this.saveToStorage();
	}

	removeBook(bookId: string) {
		this.items = this.items.filter((i) => i.book.id !== bookId);
		const unsub = this.activeSubscriptions.get(bookId);
		if (unsub) {
			unsub();
			this.activeSubscriptions.delete(bookId);
		}
		this.saveToStorage();
	}

	clear() {
		for (const unsub of this.activeSubscriptions.values()) {
			unsub();
		}
		this.activeSubscriptions.clear();
		this.items = [];
		this.saveToStorage();
	}

	restoreItems(items: CartItem[]) {
		for (const item of items) {
			this.addBook(item.book);
		}
	}


	get count(): number {
		return this.items.length;
	}

	get totalPrice(): number {
		return this.items.reduce((sum, item) => sum + (item.book.price || 0), 0);
	}

	get hasUnavailable(): boolean {
		return this.items.some((item) => !item.isAvailable);
	}

	has(bookId: string): boolean {
		return this.items.some((i) => i.book.id === bookId);
	}
}

export const cart = new CartStore();
