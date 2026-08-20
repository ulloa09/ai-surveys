import { apiFetch } from '$lib/server/api';
import type { Survey } from '$lib/types';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies, url }) => {
	const token = cookies.get('session');
	const showArchived = url.searchParams.get('archived') === 'true';
	const path = showArchived ? '/api/surveys?include_archived=true' : '/api/surveys';
	const res = await apiFetch(path, token);
	const surveys: Survey[] = res.ok ? await res.json() : [];
	return { surveys, showArchived };
};
