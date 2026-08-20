// src/routes/s/[token]/open-answer/+server.ts
// Registra respuesta open_ended sin avanzar el índice
// (el engine avanza después del followup)
import { json } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import type { RequestHandler } from './$types';

const API = env.API_URL ?? 'http://localhost:8080';

export const POST: RequestHandler = async ({ request }) => {
	const body = await request.json().catch(() => null);
	if (!body) return json({ error: 'Body inválido' }, { status: 400 });

	const { responseId, questionId, value } = body;
	if (!responseId || !questionId || !value) {
		return json({ error: 'Faltan campos' }, { status: 400 });
	}

	const res = await fetch(`${API}/api/responses/${responseId}/open-answer`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ question_id: questionId, value })
	});

	const data = await res.json().catch(() => ({}));
	return json(data, { status: res.status });
};
