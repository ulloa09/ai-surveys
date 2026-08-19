import { apiFetch } from '$lib/server/api';
import type { Survey } from '$lib/types';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies }) => {
	const token = cookies.get('session');
	const res = await apiFetch('/api/surveys', token);
	const surveys: Survey[] = res.ok ? await res.json() : [];
	return { surveys };
};
