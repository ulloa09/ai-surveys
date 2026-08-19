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
