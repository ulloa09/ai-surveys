// src/routes/s/[token]/answers/+server.ts
// Proxy público para GET y POST de answers — sin sesión requerida.
import { json } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import type { RequestHandler } from './$types';

const API = env.API_URL ?? 'http://localhost:8080';

// GET /s/[token]/answers?responseId=xxx&withValues=true
// Sin withValues: devuelve { answered: string[] } — IDs de preguntas respondidas.
// Con withValues: devuelve { answers: [{questionId, value}] } para allow_revisit.
export const GET: RequestHandler = async ({ url }) => {
	const responseId = url.searchParams.get('responseId');
	const withValues = url.searchParams.get('withValues') === 'true';
	if (!responseId) return json(withValues ? { answers: [] } : { answered: [] });

	const backendUrl = `${API}/api/responses/${responseId}/answers${withValues ? '?withValues=true' : ''}`;
	const res = await fetch(backendUrl);
	if (!res.ok) return json(withValues ? { answers: [] } : { answered: [] });

	const data = await res.json().catch(() => (withValues ? { answers: [] } : { answered: [] }));
	return json(data);
};

// POST /s/[token]/answers
// Body: { responseId, questionId, value }
// Registra directamente una respuesta estructurada en el engine.
export const POST: RequestHandler = async ({ request }) => {
	const body = await request.json().catch(() => null);
	if (!body) return json({ error: 'Body inválido' }, { status: 400 });

	const { responseId, questionId, value } = body;
	if (!responseId || !questionId || value === undefined || value === '') {
		return json({ error: 'responseId, questionId y value son requeridos' }, { status: 400 });
	}

	const res = await fetch(`${API}/api/responses/${responseId}/answers`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ question_id: questionId, value })
	});

	const data = await res.json().catch(() => ({}));
	return json(data, { status: res.status });
};
