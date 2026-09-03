export interface User {
	id: string;
	email: string;
	name?: string;
	avatar?: string;
	isCashier?: boolean;
	buy?: string[];
	created: string;
	updated: string;
}

export interface Event {
	id: string;
	name: string;
	active: boolean;
	sellStart: string;
	sellEnd: string;
	buyStart: string;
	buyEnd: string;
	bankAccount?: string;
	iban?: string;
	currency: string;
}

export type BookStatus = 'available' | 'checkout' | 'bought';

export interface Book {
	id: string;
	code: string;
	seller: string;
	buyer?: string;
	event: string;
	price: number;
	photo: string;
	status: BookStatus;
	checkoutExpiresAt?: string;
	created: string;
	updated: string;
	expand?: {
		seller?: User;
		buyer?: User;
		event?: Event;
	};
}

export type PaymentMethod = 'qr' | 'cash';
export type PaymentStatus = 'pending' | 'completed' | 'cancelled';

export interface Payment {
	id: string;
	variableSymbol: number;
	buyer: string;
	books: string[];
	totalAmount: number;
	method: PaymentMethod;
	status: PaymentStatus;
	cashier?: string;
	created: string;
	updated: string;
	expand?: {
		buyer?: User;
		books?: Book[];
		cashier?: User;
	};
}
