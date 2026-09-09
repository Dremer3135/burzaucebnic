import PocketBase from 'pocketbase';
import type { Handle, RequestEvent } from '@sveltejs/kit';
import type { User } from '$lib/types';
import { isMaintenanceBreakActive } from '$lib/server/maintenance';

function createRedirectResponse(targetPath: string, event: RequestEvent): Response {
	const response = new Response(null, {
		status: 307,
		headers: {
			Location: targetPath
		}
	});

	response.headers.append(
		'set-cookie',
		event.locals.pb.authStore.exportToCookie({
			httpOnly: false,
			sameSite: 'lax',
			path: '/'
		})
	);

	return response;
}

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

	const pathname = event.url.pathname;
	const user = event.locals.user;

	// Ignore internal system and asset requests
	const isAssetOrSystemPath =
		pathname.startsWith('/_app/') ||
		pathname.startsWith('/api/') ||
		pathname.startsWith('/_/') ||
		pathname === '/favicon.svg' ||
		pathname === '/skrat_logo.svg';

	if (!isAssetOrSystemPath) {
		// In dev mode, /tutorial is always directly accessible without login or maintenance restrictions
		if (import.meta.env.DEV && pathname === '/tutorial') {
			return resolve(event);
		}

		// 1. Unauthenticated users: can ONLY access /, public legal terms, and tutorial
		if (!user) {
			if (pathname !== '/' && pathname !== '/terms' && pathname !== '/privacy' && pathname !== '/tutorial') {
				return createRedirectResponse('/', event);
			}
			if (pathname === '/tutorial' && !import.meta.env.DEV) {
				const isMaintenance = await isMaintenanceBreakActive(event.locals.pb);
				if (isMaintenance) {
					return createRedirectResponse('/maintenance', event);
				}
			}
		} else if (!user.isCashier) {
			// 2. Logged-in regular students:
			const isMaintenance = await isMaintenanceBreakActive(event.locals.pb);
			if (isMaintenance) {
				// During maintenance, students cannot access functional pages; redirect to /maintenance
				if (pathname !== '/maintenance') {
					return createRedirectResponse('/maintenance', event);
				}
			} else {
				// If maintenance break is not active, redirect away from /maintenance
				if (pathname === '/maintenance') {
					return createRedirectResponse('/', event);
				}
			}
		}
		// 3. Cashiers (user.isCashier === true) are completely unaffected
	}

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
