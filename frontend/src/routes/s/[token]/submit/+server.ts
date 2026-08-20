// src/routes/s/[token]/submit/+server.ts
// Proxy para el submit final de la respuesta.
// No requiere sesión — el respondiente se identifica por response_id.
import { json } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import type { RequestHandler } from './$types';

const API = env.API_URL ?? 'http://localhost:8080';

export const POST: RequestHandler = async ({ request }) => {
	const body = await request.json();
	const { responseId } = body;

	if (!responseId) {
		return json({ error: 'responseId es requerido' }, { status: 400 });
	}

	const res = await fetch(`${API}/api/responses/${responseId}/submit`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' }
	});

	const data = await res.json().catch(() => ({}));
	return json(data, { status: res.status });
};
