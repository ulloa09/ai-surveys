// src/routes/s/[token]/response-status/+server.ts
// Devuelve el status actual de una respuesta. El frontend lo usa para
// detectar si el engine marcó la respuesta como pending_submission
// por límite de turnos u otro trigger de terminación.
import { json } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import type { RequestHandler } from './$types';

const API = env.API_URL ?? 'http://localhost:8080';

export const GET: RequestHandler = async ({ url }) => {
	const responseId = url.searchParams.get('responseId');
	if (!responseId) return json({ status: 'unknown' });

	const res = await fetch(`${API}/api/responses/${responseId}/status`);
	if (!res.ok) return json({ status: 'unknown' });

	const data = await res.json().catch(() => ({ status: 'unknown' }));
	return json(data);
};
