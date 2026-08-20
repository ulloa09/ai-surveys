// src/routes/s/[token]/turn/+server.ts
// Proxy SSE: recibe el mensaje del respondiente y hace streaming de la
// respuesta del engine al cliente. No requiere sesión — el respondiente
// se identifica por response_id.
// GET: devuelve el historial de turnos para reconstruir el chat (allow_revisit).
import { env } from '$env/dynamic/private';
import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

const API = env.API_URL ?? 'http://localhost:8080';

export const GET: RequestHandler = async ({ url }) => {
	const responseId = url.searchParams.get('responseId');
	if (!responseId) return json({ turns: [] });

	const res = await fetch(`${API}/api/responses/${responseId}/turns`);
	if (!res.ok) return json({ turns: [] });

	return json(await res.json());
};

export const POST: RequestHandler = async ({ request }) => {
	const body = await request.json();
	const { responseId, message } = body;

	if (!responseId || !message) {
		return new Response(JSON.stringify({ error: 'responseId y message son requeridos' }), {
			status: 400,
			headers: { 'Content-Type': 'application/json' }
		});
	}

	const res = await fetch(`${API}/api/responses/${responseId}/turns`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ message })
	});

	if (!res.ok) {
		const errBody = await res.json().catch(() => ({}));
		return new Response(JSON.stringify(errBody), {
			status: res.status,
			headers: { 'Content-Type': 'application/json' }
		});
	}

	// Pasar el stream SSE tal cual al cliente
	return new Response(res.body, {
		status: 200,
		headers: {
			'Content-Type': 'text/event-stream',
			'Cache-Control': 'no-cache',
			Connection: 'keep-alive'
		}
	});
};
