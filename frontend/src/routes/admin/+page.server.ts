import { redirect } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import type { Actions, PageServerLoad } from './$types';

const API_URL = env.API_URL ?? 'http://localhost:8080';

export const load: PageServerLoad = async ({ locals }) => {
	return { user: locals.user };
};

export const actions: Actions = {
	logout: async ({ cookies, fetch }) => {
		const token = cookies.get('session');
		if (token) {
			try {
				await fetch(`${API_URL}/api/auth/logout`, {
					method: 'POST',
					headers: { Cookie: `session=${token}` }
				});
			} catch {
				// Backend unreachable — clear the cookie regardless
			}
		}

		cookies.delete('session', { path: '/' });
		redirect(302, '/login');
	}
};
