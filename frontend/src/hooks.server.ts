import type { Handle } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';

const API_URL = env.API_URL ?? 'http://localhost:8080';

export const handle: Handle = async ({ event, resolve }) => {
	event.locals.user = null;

	const token = event.cookies.get('session');
	if (token) {
		try {
			const res = await fetch(`${API_URL}/api/admin/me`, {
				headers: { Cookie: `session=${token}` }
			});
			if (res.ok) {
				event.locals.user = await res.json();
			}
		} catch {
			// Backend unreachable — treat as unauthenticated
		}
	}

	return resolve(event);
};
