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
	anonymity_level: 'none' | 'partial' | 'full';
	allow_revisit: boolean;
	optional_registration: boolean;
	created_at: string;
	updated_at: string;
}

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
