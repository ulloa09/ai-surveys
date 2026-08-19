-- +goose Up

-- Fase 3 (RBAC) — colapsa team_role a solo los 2 roles de grupo/clase que
-- realmente se usan: 'profesor' y 'alumno'. Los valores 'owner'/'editor'/
-- 'viewer' (colaboración de staff) quedan eliminados: admin/super_admin ya
-- administran cualquier equipo vía bypass de rol global (ver Rbac.go), sin
-- necesidad de ser miembros de team_members.
--
-- Antes de recrear el tipo, dejamos cada fila de team_members consistente
-- con el rol real de la cuenta (users.role):
--   - si el usuario es profesor/alumno, su rol de equipo pasa a coincidir
--     con su rol de cuenta (corrige asignaciones inconsistentes hechas con
--     el selector viejo, ej. un profesor agregado como "viewer").
--   - si el usuario es admin/super_admin, se elimina su fila de
--     team_members — ya no se modela como membresía.
UPDATE team_members tm
SET role = u.role::text::team_role
FROM users u
WHERE tm.user_id = u.id AND u.role IN ('profesor', 'alumno');

DELETE FROM team_members tm
USING users u
WHERE tm.user_id = u.id AND u.role IN ('admin', 'super_admin');

CREATE TYPE team_role_new AS ENUM ('profesor', 'alumno');

ALTER TABLE team_members ALTER COLUMN role DROP DEFAULT;
ALTER TABLE team_members ALTER COLUMN role TYPE team_role_new USING role::text::team_role_new;
ALTER TABLE team_members ALTER COLUMN role SET DEFAULT 'alumno';

DROP TYPE team_role;
ALTER TYPE team_role_new RENAME TO team_role;

-- +goose Down

-- Revertir es best-effort: las filas de admin/super_admin eliminadas por el
-- Up no se pueden restaurar. Recreamos el enum de 5 valores para que el
-- código viejo (owner/editor/viewer) vuelva a compilar contra el schema.
CREATE TYPE team_role_old AS ENUM ('owner', 'editor', 'viewer', 'profesor', 'alumno');

ALTER TABLE team_members ALTER COLUMN role DROP DEFAULT;
ALTER TABLE team_members ALTER COLUMN role TYPE team_role_old USING role::text::team_role_old;
ALTER TABLE team_members ALTER COLUMN role SET DEFAULT 'viewer';

DROP TYPE team_role;
ALTER TYPE team_role_old RENAME TO team_role;
