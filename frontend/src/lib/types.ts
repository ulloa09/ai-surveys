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
	status: 'draft' | 'open' | 'closed' | 'archived';
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
	created_at: string;
	updated_at: string;
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
