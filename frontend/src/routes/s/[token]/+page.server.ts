import { error, fail, redirect } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import type { Actions, PageServerLoad, RequestEvent } from './$types';

const API = env.API_URL ?? 'http://localhost:8080';

// El backend reconoce "sin sesión" por la AUSENCIA del header. Mandar
// `session=undefined` sería un token inválido, no un respondiente anónimo.
// deviceId viaja en la misma cookie cuando existe: es lo que el backend usa
// para detectar duplicados en las encuestas pseudónimas.
const authHeaders = (session?: string, deviceId?: string): HeadersInit => {
	const parts: string[] = [];
	if (session) parts.push(`session=${session}`);
	if (deviceId) parts.push(`device_id=${deviceId}`);
	return parts.length ? { Cookie: parts.join('; ') } : {};
};

// ensureDeviceId emite (una vez por navegador) el UUID aleatorio con el que se
// reconoce el dispositivo en las encuestas pseudónimas ('partial'). El backend
// nunca guarda este valor: solo su HMAC (ver internal/fingerprint), así que el
// hash almacenado no permite volver a la persona.
//
// Solo se emite para 'partial'. Una encuesta 'full' no deja NADA en el
// dispositivo ni en la base — ni siquiera este identificador no reversible —
// porque su landing promete exactamente eso; y 'none' ya se deduplica por cuenta.
function ensureDeviceId(cookies: RequestEvent['cookies'], anonymityLevel: string): string | undefined {
	if (anonymityLevel !== 'partial') return undefined;

	const existing = cookies.get('device_id');
	if (existing) return existing;

	const deviceId = crypto.randomUUID();
	cookies.set('device_id', deviceId, {
		path: '/',
		httpOnly: true,
		sameSite: 'lax',
		secure: env.APP_ENV === 'production',
		maxAge: 60 * 60 * 24 * 365 // un año: debe sobrevivir a la encuesta.
	});
	return deviceId;
}

export const load: PageServerLoad = async ({ params, cookies }) => {
	// Quién puede abrir la encuesta lo decide el backend según su anonymity_level:
	// las 'full' (anónimas) tienen link público y se abren sin cuenta; el resto
	// exige sesión y pertenecer al equipo al que se desplegó el link. Por eso NO
	// se gatea aquí antes de preguntar: se pide la encuesta y solo si el backend
	// responde 401 se manda al login.
	const session = cookies.get('session');
	const res = await fetch(`${API}/api/public/surveys/${params.token}`, {
		headers: authHeaders(session)
	});

	if (res.status === 401) {
		const returnTo = `/s/${params.token}`;
		redirect(302, `/login?redirect=${encodeURIComponent(returnTo)}`);
	}
	if (res.status === 403) {
		error(
			403,
			'No perteneces al equipo al que se desplegó esta encuesta. Pídele a quien te la compartió que te agregue a su equipo.'
		);
	}
	if (res.status === 404) error(404, 'Encuesta no encontrada');
	if (!res.ok) error(res.status, 'No se pudo cargar la encuesta');

	const survey = await res.json();

	// Encuestas en borrador o archivadas no son accesibles públicamente: 404.
	// 'closed' sí se muestra (con su mensaje de cierre) y 'open' renderiza el landing.
	if (survey.status === 'draft' || survey.status === 'archived') {
		error(404, 'Encuesta no encontrada');
	}

	// Se emite acá, al entrar al landing, para que la cookie ya exista cuando el
	// respondiente mande el formulario (no-op si no es 'partial' o si ya la tiene).
	ensureDeviceId(cookies, survey.anonymity_level);

	return { survey };
};

export const actions: Actions = {
	start: async (event) => {
		const { params, cookies } = event;
		const form = await event.request.formData();

		// Sin gate de sesión propio: contestar sin cuenta es válido en las
		// encuestas anónimas ('full'). Para las demás el backend responde 401 y
		// se traduce abajo a un mensaje claro.
		const session = cookies.get('session');
		const language = String(form.get('language') ?? '').trim();
		const registeredName = String(form.get('registered_name') ?? '').trim();
		const registeredEmail = String(form.get('registered_email') ?? '').trim();

		const surveyRes = await fetch(`${API}/api/public/surveys/${params.token}`, {
			headers: authHeaders(session)
		});
		if (surveyRes.status === 401) {
			return fail(401, { error: 'Debes iniciar sesión para responder esta encuesta.' });
		}
		if (surveyRes.status === 403) {
			return fail(403, { error: 'No perteneces al equipo al que se desplegó esta encuesta.' });
		}
		if (!surveyRes.ok) return fail(404, { error: 'Encuesta no encontrada.' });
		const survey = await surveyRes.json();

		// En 'partial' este id es lo único que permite detectar que este
		// dispositivo ya contestó — la respuesta no guarda user_id. Normalmente
		// la cookie ya viene del landing; se re-emite acá por si la petición no
		// pasó por load().
		const deviceId = ensureDeviceId(cookies, survey.anonymity_level);

		const body: Record<string, unknown> = {
			language: language || undefined,
			registered_name: registeredName || undefined,
			registered_email: registeredEmail || undefined
			// user_id NO se manda: el backend siempre lo toma de la sesión
			// autenticada (Cookie), nunca de un valor que mande el cliente.
		};

		const res = await fetch(`${API}/api/public/surveys/${params.token}/responses`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', ...authHeaders(session, deviceId) },
			body: JSON.stringify(body)
		});

		if (!res.ok) {
			const resBody = await res.json().catch(() => ({}));
			if (res.status === 401) {
				return fail(401, { error: 'Debes iniciar sesión para responder esta encuesta.' });
			}
			if (res.status === 403) {
				return fail(403, {
					error: 'No perteneces al equipo al que se desplegó esta encuesta, así que no puedes responderla.'
				});
			}
			if (res.status === 409 && resBody.error === 'already responded to this survey') {
				return fail(409, {
					error: 'Ya has respondido esta encuesta. Solo se permite una respuesta por persona.'
				});
			}
			return fail(res.status, { error: resBody.error ?? 'No se pudo iniciar la encuesta.' });
		}

		const result = await res.json();
		return {
			responseId: result.response.id as string,
			resumeToken: (result.resume_token ?? null) as string | null
		};
	}
};
