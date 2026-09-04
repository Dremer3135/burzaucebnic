import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ locals }) => {
	if (locals.user) {
		let targetRoute: string | null = null;
		try {
			const events = await locals.pb.collection('events').getFullList({
				filter: 'active = true',
				sort: '-created'
			});
			if (events.length > 0 && events[0].active) {
				targetRoute = events[0].defaultPage === 'seeprice' ? '/seeprice' : '/sell';
			}
		} catch (err: any) {
			console.error('Failed to fetch active event in +page.server.ts', err);
		}

		if (targetRoute) {
			throw redirect(303, targetRoute);
		}
	}
	return {};
};
