import { env } from '$env/dynamic/private';
import type { PageServerLoad } from './$types';

const API = env.API_URL ?? 'http://localhost:8080';

export const load: PageServerLoad = async ({ fetch }) => {
	try {
		const res = await fetch(`${API}/api/health`);
		const body = await res.json();
		return { status: body.status, database: body.database };
	} catch {
		return { status: 'degraded', database: 'disconnected' };
	}
};
