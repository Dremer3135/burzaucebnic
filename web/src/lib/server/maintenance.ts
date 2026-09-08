import type PocketBase from 'pocketbase';
import type { Event } from '$lib/types';

let cachedBreak: boolean = false;
let lastCheck = 0;
const CACHE_TTL_MS = 1500;

/**
 * Checks whether maintenance_break is active for any currently active event.
 * Results are cached in memory for 1.5 seconds to avoid hammering PocketBase.
 */
export async function isMaintenanceBreakActive(pb: PocketBase): Promise<boolean> {
	const now = Date.now();
	if (now - lastCheck < CACHE_TTL_MS) {
		return cachedBreak;
	}

	try {
		const events = await pb.collection('events').getFullList<Event>({
			filter: 'active = true',
			sort: '-id'
		});

		if (events.length > 0) {
			const ev = events[0] as any;
			cachedBreak = Boolean(ev.maintenance_break ?? ev.maintenence_break);
		} else {
			cachedBreak = false;
		}
		lastCheck = now;
	} catch (err) {
		console.error('Failed to check maintenance break in events collection:', err);
	}

	return cachedBreak;
}
