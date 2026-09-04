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
	defaultPage?: 'sell' | 'seeprice';
	bankAccount?: string;
	iban?: string;
	currency: string;
}

export type BookStatus = 'available' | 'checkout' | 'bought';

export interface Book {
	id: string;
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

export interface BookPriceResponse {
	id: string;
	price: number;
	status: BookStatus;
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
