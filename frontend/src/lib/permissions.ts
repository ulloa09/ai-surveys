// permissions.ts
// Hoja de permisos centralizada. Cada rol tiene un conjunto de capacidades
// que los componentes usan para decidir qué renderizar.
// NUNCA uses el rol directamente en los componentes — siempre usa perms.
//
// Fase 3 (RBAC) — matriz de roles:
//   super_admin: gestión total (usuarios, equipos, encuestas, datos globales).
//   admin (Coordinador): encuestas y equipos propios/administrados; datos
//     (a futuro) limitados a sus equipos.
//   profesor: solo lectura de encuestas/estado de sus grupos; datos
//     (a futuro) limitados a sus grupos. Nunca crea/edita/borra.
//   alumno: SIN acceso a dashboards, analytics ni reportes — solo responde
//     encuestas asignadas via el flujo público /s/{token}.

export type Role = 'super_admin' | 'admin' | 'profesor' | 'alumno';

export interface Permissions {
  // Equipos / grupos
  canCreateTeam: boolean;       // crear un equipo/grupo nuevo
  canViewTeams: boolean;        // ver la pestaña de equipos
  canInviteMember: boolean;     // agregar miembros (asignar profesor, inscribir alumno)
  canRemoveMember: boolean;     // remover miembros de un equipo
  canMoveMembers: boolean;      // mover un alumno/miembro entre equipos

  // Usuarios (Fase 2 — exclusivo de super_admin)
  canManageUsers: boolean;      // ver y administrar la seccion de usuarios

  // Encuestas
  canViewSurveys: boolean;      // ver la pestaña/listado de encuestas
  canCreateSurvey: boolean;     // crear encuesta nueva
  canEditSurvey: boolean;       // editar título, descripción, configuración
  canDeleteSurvey: boolean;     // borrar encuesta sin respuestas
  canDuplicateSurvey: boolean;  // duplicar una encuesta
  canArchiveSurvey: boolean;    // archivar — solo super_admin

  // Preguntas
  canEditQuestions: boolean;    // agregar, editar, reordenar, borrar preguntas

  // Datos: resultados, estadísticas, insights de IA (Fase 3 — placeholders,
  // la lógica real y el scoping por equipo/grupo llegan después).
  canViewResults: boolean;         // ver reportes/estadísticas/insights
  canExportResults: boolean;       // exportar datos
  canViewGlobalAnalytics: boolean; // estadísticas de toda la plataforma — solo super_admin
}

const permissionsByRole: Record<Role, Permissions> = {
  super_admin: {
    canCreateTeam: true,
    canViewTeams: true,
    canInviteMember: true,
    canRemoveMember: true,
    canMoveMembers: true,

    canManageUsers: true,

    canViewSurveys: true,
    canCreateSurvey: true,
    canEditSurvey: true,
    canDeleteSurvey: true,
    canDuplicateSurvey: true,
    canArchiveSurvey: true,

    canEditQuestions: true,

    canViewResults: true,
    canExportResults: true,
    canViewGlobalAnalytics: true,
  },

  admin: {
    canCreateTeam: true,
    canViewTeams: true,
    canInviteMember: true,
    canRemoveMember: true,
    canMoveMembers: true,

    canManageUsers: false,      // solo super_admin administra usuarios

    canViewSurveys: true,
    canCreateSurvey: true,
    canEditSurvey: true,
    canDeleteSurvey: true,
    canDuplicateSurvey: true,
    canArchiveSurvey: false,    // solo super_admin archiva

    canEditQuestions: true,

    canViewResults: true,       // a futuro: limitado a sus equipos
    canExportResults: true,
    canViewGlobalAnalytics: false,
  },

  profesor: {
    canCreateTeam: false,
    canViewTeams: true,         // ve los grupos de los que es miembro
    canInviteMember: false,
    canRemoveMember: false,
    canMoveMembers: false,

    canManageUsers: false,

    canViewSurveys: true,       // solo lectura — encuestas de sus grupos
    canCreateSurvey: false,
    canEditSurvey: false,
    canDeleteSurvey: false,
    canDuplicateSurvey: false,
    canArchiveSurvey: false,

    canEditQuestions: false,

    canViewResults: true,       // limitado a sus grupos (lo aplica el backend)
    // Exportar (#17): el profesor SÍ puede bajar los datos, pero solo de los
    // equipos de los que es miembro — el archivo se arma con el mismo recorte
    // por equipo que ya usa el dashboard (ver services.ExportService). No es
    // una excepción al "solo lectura" del rol: exportar es leer.
    canExportResults: true,
    canViewGlobalAnalytics: false,
  },

  alumno: {
    canCreateTeam: false,
    canViewTeams: false,        // sin acceso a dashboards
    canInviteMember: false,
    canRemoveMember: false,
    canMoveMembers: false,

    canManageUsers: false,

    canViewSurveys: false,      // responde via /s/{token}, no ve el panel de gestión
    canCreateSurvey: false,
    canEditSurvey: false,
    canDeleteSurvey: false,
    canDuplicateSurvey: false,
    canArchiveSurvey: false,

    canEditQuestions: false,

    canViewResults: false,      // ABSOLUTAMENTE sin acceso a dashboards/analytics/reportes
    canExportResults: false,
    canViewGlobalAnalytics: false,
  },
};

// Funcion principal. Usa esto en tus pages y componentes:
// const perms = getPermissions(data.user.role);
export function getPermissions(role: string): Permissions {
  return permissionsByRole[role as Role] ?? permissionsByRole.alumno;
}
