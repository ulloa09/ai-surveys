// src/routes/s/[token]/questions/+server.ts
// Devuelve las preguntas de la encuesta al frontend del respondiente.
//
// GET /api/public/surveys/{token}/questions se queda público (no expone nada
// sensible por sí solo) — este proxy no pre-valida la encuesta contra
// GET /api/public/surveys/{token}: quien llega hasta acá ya pasó por el
// landing/conversación, que gatean login + membresía de equipo antes de
// renderizar. Repetir la validación aquí solo rompería la carga por falta de
// cookie (esta función corre client-side).
import { json } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import type { RequestHandler } from './$types';

const API = env.API_URL ?? 'http://localhost:8080';

export const GET: RequestHandler = async ({ params }) => {
	const questionsRes = await fetch(`${API}/api/public/surveys/${params.token}/questions`);
	if (questionsRes.ok) {
		return json(await questionsRes.json());
	}

	// Fallback: devolver array vacío — el engine funciona sin preguntas cargadas
	// en el frontend (las tiene en el backend)
	return json([]);
};
