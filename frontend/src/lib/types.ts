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
