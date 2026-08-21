export interface HealthStatus {
	status: 'ok' | 'degraded';
	database: 'connected' | 'disconnected';
}

export interface User {
	id: string;
	email: string;
	display_name: string;
	role: 'super_admin' | 'admin' | 'profesor' | 'alumno';
	created_at: string;
}

export const ROLE_LABELS: Record<User['role'], string> = {
	super_admin: 'Super Admin',
	admin: 'Administrador',
	profesor: 'Profesor',
	alumno: 'Alumno'
};

export interface Team {
	id: string;
	name: string;
	created_by: string;
	created_at: string;
}

export interface TeamMember {
	team_id: string;
	user_id: string;
	email: string;
	display_name: string;
	role: 'profesor' | 'alumno';
}

export interface TeamWithMembers extends Team {
	members: TeamMember[];
}

export interface Survey {
	id: string;
	title: string;
	description: string | null;
	owner_id: string;
	owner_name?: string;
	team_id: string;
	team_name?: string;
	// analysing/complete/failed los escribe el Analysis Engine (#15) al cerrar
	// la encuesta: primero pasa a analysing mientras corre el job, luego a
	// complete, o a failed si el job no pudo terminar (proveedor caído, JSON
	// inválido, timeout) — POST /analysis/retry vuelve a encolarlo desde ahí.
	// La vista de análisis (#16) existe en los tres estados.
	status: 'draft' | 'open' | 'closed' | 'analysing' | 'complete' | 'failed' | 'archived';
	mode: 'conversational' | 'form' | 'prompt_only';
	system_prompt: string | null;
	available_languages: LanguageCode[];
	default_language: LanguageCode;
	anonymity_level: 'none' | 'partial' | 'full';
	allow_revisit: boolean;
	optional_registration: boolean;
	termination_mode: 'turn_limit' | 'question_coverage' | 'time_estimate' | 'combination';
	turn_limit: number | null;
	time_estimate_minutes: number | null;
	opens_at: string | null;
	closes_at: string | null;
	response_cap: number | null;
	public_token: string;
	qr_png_url: string | null;
	qr_svg_url: string | null;
	created_at: string;
	updated_at: string;
	// stats no viene en /api/surveys — el listado la cruza por survey_id
	// desde /api/surveys/stats, que trae los agregados de todas las encuestas
	// visibles en una sola llamada.
	stats?: SurveyStats;
}

// SurveyStats agrega las respuestas de una encuesta. avg_duration_seconds es
// null mientras no haya ninguna respuesta enviada — no hay duración que
// promediar. expected_responses/missing_responses/coverage_rate son null si
// la encuesta no declaró un response_cap — completion_rate (submitted/started)
// sigue siendo la única métrica disponible en ese caso, aunque en encuestas
// totalmente anónimas no dice cuánta gente real participó (cada visita crea
// una respuesta nueva).
export interface SurveyStats {
	survey_id: string;
	response_count: number;
	completed_count: number;
	completion_rate: number;
	avg_duration_seconds: number | null;
	language_distribution: LanguageCount[];
	expected_responses: number | null;
	missing_responses: number | null;
	coverage_rate: number | null;
}

export interface LanguageCount {
	language: string;
	count: number;
}

export type LanguageCode = 'es' | 'en';

export const LANGUAGE_LABELS: Record<LanguageCode, string> = {
	es: 'Español',
	en: 'Inglés'
};

export const SURVEY_MODES: {
	value: Survey['mode'];
	label: string;
	description: string;
	example: string;
}[] = [
	{
		value: 'form',
		label: 'Preguntas fijas + seguimiento IA',
		description: 'La IA hace tus preguntas en orden y puede profundizar 1-2 niveles en cada una.',
		example: 'Ej: "¿Qué tan útil fue el taller?" → si responde "poco", la IA pregunta por qué antes de seguir.'
	},
	{
		value: 'prompt_only',
		label: 'Solo system prompt',
		description: 'Escribes un system prompt y la IA conduce la conversación libremente.',
		example: 'Ej: "Eres un entrevistador que explora la experiencia del alumno en el semestre..."'
	},
	{
		value: 'conversational',
		label: 'Híbrido (recomendado)',
		description: 'Defines preguntas requeridas y un system prompt; la IA las cubre todas con libertad para adaptar el tono.',
		example: 'Ej: preguntas fijas sobre el curso, más instrucciones de tono para sondear con empatía.'
	}
];

export const TERMINATION_MODES: { value: Survey['termination_mode']; label: string; description: string }[] = [
	{
		value: 'turn_limit',
		label: 'Límite de turnos',
		description: 'La conversación termina tras un número máximo de intercambios (por defecto 12).'
	},
	{
		value: 'question_coverage',
		label: 'Cobertura de preguntas',
		description: 'Termina cuando todas las preguntas requeridas fueron cubiertas.'
	},
	{
		value: 'time_estimate',
		label: 'Duración estimada',
		description: 'Se muestra al respondiente una duración esperada; la IA ajusta el ritmo para cumplirla.'
	},
	{
		value: 'combination',
		label: 'Combinación',
		description: 'Los tres criterios están activos a la vez; el primero en cumplirse termina la conversación.'
	}
];

export const STATUS_LABELS: Record<Survey['status'], string> = {
	draft: 'Borrador',
	open: 'Abierta',
	closed: 'Cerrada',
	analysing: 'Analizando',
	complete: 'Analizada',
	failed: 'Error en análisis',
	archived: 'Archivada'
};

export const ANONYMITY_LABELS: Record<Survey['anonymity_level'], string> = {
	none: 'Identificada',
	partial: 'Pseudónima',
	full: 'Anónima'
};

export interface Question {
	id: string;
	survey_id: string;
	type: QuestionType;
	text: string;
	required: boolean;
	ai_followup: boolean;
	options: QuestionOptions | null;
	order_index: number;
	created_at: string;
}

export type QuestionType =
	| 'open_ended'
	| 'single_choice'
	| 'multi_choice'
	| 'true_false'
	| 'linear_scale'
	| 'ranking'
	| 'matrix';

// Opciones por tipo de pregunta.
// El backend guarda esto como JSONB y lo devuelve tal cual.

export interface ChoiceOption {
	label: string;
	value: string;
}

export interface ChoiceOptions {
	choices: ChoiceOption[];
}

export interface ScaleOptions {
	min: number;
	max: number;
	min_label?: string;
	max_label?: string;
}

export interface RankingOptions {
	items: ChoiceOption[];
}

export interface MatrixOptions {
	rows: string[];
	columns: string[];
}

export type QuestionOptions = ChoiceOptions | ScaleOptions | RankingOptions | MatrixOptions;

export const QUESTION_TYPE_LABELS: Record<QuestionType, string> = {
	open_ended: 'Respuesta abierta',
	single_choice: 'Opción única',
	multi_choice: 'Opción múltiple',
	true_false: 'Verdadero / Falso',
	linear_scale: 'Escala lineal',
	ranking: 'Ranking',
	matrix: 'Matriz'
};

export const QUESTION_TYPE_DESCRIPTIONS: Record<QuestionType, string> = {
	open_ended: 'El participante escribe libremente. La IA puede hacer seguimiento.',
	single_choice: 'El participante elige una opción de una lista.',
	multi_choice: 'El participante puede elegir varias opciones.',
	true_false: 'Pregunta de sí o no.',
	linear_scale: 'El participante elige un número en un rango, por ejemplo del 1 al 5.',
	ranking: 'El participante ordena una lista de elementos.',
	matrix: 'El participante evalúa varios elementos en una escala.'
};

export interface SurveyResponse {
	id: string;
	survey_id: string;
	user_id: string | null;
	fingerprint_hash: string | null;
	status: 'in_progress' | 'submitted' | 'abandoned';
	language: LanguageCode;
	started_at: string;
	submitted_at: string | null;
	current_question_index: number;
	turn_count: number;
	registered_name: string | null;
	registered_email: string | null;
}

// ── Análisis (#16) ────────────────────────────────────────────────────────
// Reflejan la respuesta de GET /api/surveys/{id}/analysis, que produce el
// Analysis Engine (#15).

// Porcentajes (0.0-1.0) de respuestas en cada categoría de sentimiento.
export interface SentimentDistribution {
	positive: number;
	neutral: number;
	negative: number;
}

// Tema recurrente entre las respuestas a una pregunta.
export interface TopicCluster {
	tag: string;
	count: number;
}

// Cantidad de respuestas que eligieron un valor en una pregunta estructurada.
// label es cómo se le presentó la opción al participante ("Muy lento"), no el
// valor interno que se guarda ("muy_lento").
export interface OptionCount {
	value: string;
	label: string;
	count: number;
}

// Respuesta que la IA marcó como atípica para su pregunta. Solo las preguntas
// abiertas tienen outliers: en una escala, una calificación baja no es una
// respuesta atípica.
export interface OutlierAnswer {
	response_id: string;
	raw_value: string;
	sentiment_label: string | null;
}

// Sección de una pregunta en la vista de análisis. analysed_at es null
// mientras el job todavía no llega a esta pregunta: en ese caso el resumen
// viene vacío y los agregados en cero.
// topic_clusters/outliers solo se llenan en preguntas abiertas, y
// answer_distribution solo en las estructuradas — son los dos agregados
// equivalentes, y la vista muestra el que corresponda al tipo de pregunta.
export interface QuestionAnalysis {
	question_id: string;
	text: string;
	type: QuestionType;
	order_index: number;
	answer_count: number;
	summary_text: string;
	sentiment_distribution: SentimentDistribution;
	topic_clusters: TopicCluster[];
	answer_distribution: OptionCount[];
	outliers: OutlierAnswer[];
	analysed_at: string | null;
}

export interface SurveyAnalysis {
	survey_id: string;
	status: Survey['status'];
	analysed_at: string | null;
	stats: SurveyStats;
	questions: QuestionAnalysis[];
}

// La vista de análisis solo existe una vez que el Analysis Engine arrancó.
// El backend responde 404 en cualquier otro estado; el dashboard usa esto
// para esconder el link en vez de ofrecer una página que no carga.
export function hasAnalysis(status: Survey['status'] | undefined): boolean {
	return status === 'analysing' || status === 'complete' || status === 'failed';
}

// ── Visor de respuestas individuales ────────────────────────────────────
// Reflejan la respuesta de GET /api/surveys/{id}/responses: la data cruda ya
// guardada en `answers`, sin depender de que el Analysis Engine haya
// corrido — a diferencia de SurveyAnalysis, que son resúmenes por IA.

export interface ResponseQuestion {
	question_id: string;
	text: string;
	type: QuestionType;
	order_index: number;
}

// Una respuesta individual. label es el email a mostrar cuando hay identidad
// (cuenta autenticada en encuestas anonymity_level='none', o un email que el
// participante compartió voluntariamente); si es null, anon_number lleva el
// número de orden de envío para mostrar "Anónimo #N".
export interface RespondentResponse {
	response_id: string;
	label: string | null;
	anon_number: number | null;
	language: string;
	submitted_at: string | null;
	answers: RespondentAnswer[];
}

export interface RespondentAnswer {
	question_id: string;
	raw_value: string;
	// display_value ya resuelve las opciones (single_choice/multi_choice/
	// ranking/matrix) al label configurado en la pregunta — es lo que hay que
	// mostrar y usar para agrupar en las estadísticas, no raw_value.
	display_value: string;
	sentiment_label: string | null;
	is_outlier: boolean;
}

export interface SurveyResponses {
	survey_id: string;
	questions: ResponseQuestion[];
	responses: RespondentResponse[];
}

// Etiqueta a mostrar para una respuesta, según las reglas del backend.
export function respondentLabel(r: RespondentResponse): string {
	if (r.label) return `Respuesta de: ${r.label}`;
	return `Respuesta anónima #${r.anon_number}`;
}

// ── Transcript de conversación (prompt_only / conversational) ──────────────
// Reflejan GET /api/responses/{id} (services.ResponseDetail). answers vive en
// la tabla `answers` y ya se ve arriba (RespondentResponse); turns vive en la
// tabla `turns` — es lo único que existe en encuestas prompt_only, que no
// tienen preguntas fijas.

export interface Turn {
	role: 'user' | 'assistant';
	content: string;
}

export interface ResponseDetail {
	turns: Turn[];
}
