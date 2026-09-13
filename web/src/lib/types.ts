export interface User {
	id: string;
	email: string;
	name?: string;
	avatar?: string;
	isCashier?: boolean;
	buy?: string[];
	payoutToBank?: boolean;
	iban?: string;
	onboardingComplete?: boolean;
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
	maintenance_break?: boolean;
}

export type BookStatus = 'available' | 'checkout' | 'bought' | 'returned';

export interface Book {
	id: string;
	seller: string;
	buyer?: string;
	event: string;
	price: number;
	photo: string;
	status: BookStatus;
	accepted?: boolean;
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

export type CodeStatus = 'checking' | 'available' | 'used' | 'user' | 'invalid';

export interface CodeValidationResult {
	code: string;
	status: 'available' | 'used' | 'user' | 'invalid' | 'checking';
	message?: string;
}

export type PaymentMethod = 'qr' | 'cash';
export type PaymentStatus = 'pending' | 'completed' | 'cancelled';
export type ConfirmationType = 'manual' | 'automatic';

export interface Payment {
	id: string;
	variableSymbol: number;
	buyer: string;
	books: string[];
	totalAmount: number;
	method: PaymentMethod;
	status: PaymentStatus;
	confirmation_type?: ConfirmationType;
	fio_transaction_id?: string;
	cashier?: string;
	created: string;
	updated: string;
	expand?: {
		buyer?: User;
		books?: Book[];
		cashier?: User;
	};
}

export interface EmailTemplate {
	id: string;
	key: 'intake_recap' | 'sale_summary';
	name: string;
	subject: string;
	bodyIntro: string;
	unacceptedWarning?: string;
	payoutBankNote: string;
	payoutCashNote: string;
	bodyOutro: string;
	created?: string;
	updated?: string;
}

export interface EmailCampaignStats {
	activeEvent: {
		id: string;
		name: string;
	} | null;
	totalSellers: number;
	totalBooks: number;
	acceptedBooks: number;
	unacceptedBooks: number;
	soldBooks: number;
	totalPayout: number;
}

export type ReturnMethod = 'cash' | 'bank';

export interface MoneyReturn {
	id: string;
	seller: string;
	event: string;
	amount: number;
	method: ReturnMethod;
	fio_transaction_id?: string;
	variableSymbol?: string;
	cashier?: string;
	created: string;
	updated: string;
	expand?: {
		seller?: User;
		event?: Event;
		cashier?: User;
	};
}

export interface SellerReturnSummary {
	id: string;
	name: string;
	email: string;
	payoutToBank: boolean;
	iban: string;
	totalEarned: number;
	totalPaid: number;
	totalUnpaid: number;
	paidInCash: number;
	paidInBank: number;
	unsoldCount: number;
	soldCount: number;
	returnedCount: number;
}

export interface SellerReturnDetails {
	seller: {
		id: string;
		name: string;
		email: string;
		payoutToBank: boolean;
		iban: string;
	};
	event: {
		id: string;
		name: string;
	};
	balances: SellerReturnSummary;
	unsoldBooks: Array<{
		id: string;
		price: number;
		photo: string;
		status: BookStatus;
		accepted: boolean;
		collectionId: string;
		collectionName: string;
	}>;
	soldBooks: Array<{
		id: string;
		price: number;
		photo: string;
		status: BookStatus;
		accepted: boolean;
		buyer?: {
			id: string;
			name: string;
			email: string;
		};
		collectionId: string;
		collectionName: string;
	}>;
	returnedBooks: Array<{
		id: string;
		price: number;
		photo: string;
		status: BookStatus;
		accepted: boolean;
		collectionId: string;
		collectionName: string;
	}>;
	moneyReturns: Array<{
		id: string;
		amount: number;
		method: ReturnMethod;
		fio_transaction_id?: string;
		variableSymbol?: string;
		cashier?: {
			id: string;
			name: string;
			email: string;
		};
		created: string;
	}>;
}

export interface FioPayoutSyncResult {
	matchedCount: number;
	totalTransactionsFetched: number;
	matchedPayouts: Array<{
		sellerId: string;
		sellerName: string;
		sellerEmail: string;
		amount: number;
		fioTransactionId: string;
		variableSymbol: string;
		account: string;
		bankCode: string;
	}>;
	cooldownSeconds: number;
	message: string;
}

