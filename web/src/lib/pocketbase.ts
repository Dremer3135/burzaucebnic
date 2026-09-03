import PocketBase from 'pocketbase';
import { writable } from 'svelte/store';
import type { User, Event, Book } from './types';

// Use same host/origin so Vite's proxy forwards /api and /_ to PocketBase,
// perfectly avoiding mixed-content HTTPS vs HTTP issues on mobile!
const pbUrl = typeof window !== 'undefined' ? window.location.origin : 'http://127.0.0.1:8090';
export const pb = new PocketBase(pbUrl);

export const currentUser = writable<User | null>(pb.authStore.model as unknown as User | null);
export const activeEvent = writable<Event | null>(null);

pb.authStore.onChange((auth) => {
	currentUser.set(pb.authStore.model as unknown as User | null);
});

export async function loginWithPassword(email: string, pass: string) {
	const authData = await pb.collection('users').authWithPassword(email, pass);
	currentUser.set(authData.record as unknown as User);
	return authData.record as unknown as User;
}

export async function loginWithGoogle() {
	const authData = await pb.collection('users').authWithOAuth2({
		provider: 'google'
	});
	currentUser.set(authData.record as unknown as User);
	return authData.record as unknown as User;
}

export function logout() {
	pb.authStore.clear();
	currentUser.set(null);
}

export async function fetchActiveEvent(): Promise<Event | null> {
	try {
		const records = await pb.collection('events').getFullList<Event>({
			filter: 'active = true',
			sort: '-created'
		});
		const event = records.length > 0 ? records[0] : null;
		activeEvent.set(event);
		return event;
	} catch (err) {
		console.error('Failed to fetch active event', err);
		activeEvent.set(null);
		return null;
	}
}

export function getBookThumbnailUrl(book: { id: string; collectionId?: string; collectionName?: string; photo: string }): string {
	if (!book || !book.photo) return '';
	const record = { collectionName: 'books', ...book };
	return pb.files.getURL(record as any, book.photo, { thumb: '100x150' });
}

export function getBookFullImageUrl(book: { id: string; collectionId?: string; collectionName?: string; photo: string }): string {
	if (!book || !book.photo) return '';
	const record = { collectionName: 'books', ...book };
	return pb.files.getURL(record as any, book.photo);
}

