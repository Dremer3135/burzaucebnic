import PocketBase from 'pocketbase';
import type { Handle } from '@sveltejs/kit';
import type { User } from '$lib/types';

export const handle: Handle = async ({ event, resolve }) => {
	const pbPort = process.env.PB_PORT || '8090';
	const pbInternalUrl = process.env.PB_INTERNAL_URL || process.env.PB_BACKEND_URL || `http://127.0.0.1:${pbPort}`;
	event.locals.pb = new PocketBase(pbInternalUrl);

	// Load auth state from request cookie
	const cookie = event.request.headers.get('cookie') || '';
	event.locals.pb.authStore.loadFromCookie(cookie);

	try {
		if (event.locals.pb.authStore.isValid) {
			await event.locals.pb.collection('users').authRefresh();
		} else {
			event.locals.pb.authStore.clear();
		}
	} catch (_) {
		event.locals.pb.authStore.clear();
	}

	event.locals.user = (event.locals.pb.authStore.record as unknown as User) || null;

	const response = await resolve(event);

	// Append refreshed cookie to response headers
	response.headers.append(
		'set-cookie',
		event.locals.pb.authStore.exportToCookie({
			httpOnly: false,
			sameSite: 'lax',
			path: '/'
		})
	);

	return response;
};
