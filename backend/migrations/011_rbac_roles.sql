-- +goose NO TRANSACTION

-- +goose Up

-- ALTER TYPE ... ADD VALUE cannot run inside a transaction in PostgreSQL.
-- Fase 3 (RBAC): el rol global 'viewer' se renombra conceptualmente a 'alumno'
-- y se agrega 'profesor' como nuevo rol intermedio (Coordinador = 'admin').
ALTER TYPE user_role ADD VALUE IF NOT EXISTS 'profesor' AFTER 'admin';
ALTER TYPE user_role ADD VALUE IF NOT EXISTS 'alumno' AFTER 'profesor';

-- team_role se reutiliza como el rol de un usuario dentro de un equipo/grupo
-- de clase: agregamos 'profesor' y 'alumno' junto a owner/editor/viewer, que
-- siguen usándose para la colaboración de staff en encuestas.
ALTER TYPE team_role ADD VALUE IF NOT EXISTS 'profesor';
ALTER TYPE team_role ADD VALUE IF NOT EXISTS 'alumno';

-- Migrar datos existentes: todo usuario 'viewer' pasa a ser 'alumno'.
-- Esta sentencia corre después de los ALTER TYPE de arriba porque el modo
-- "NO TRANSACTION" de goose ejecuta cada statement en su propio autocommit,
-- así que 'alumno' ya está confirmado como valor válido del enum.
UPDATE users SET role = 'alumno' WHERE role = 'viewer';

-- +goose Down

-- Revertir la migración de datos (best effort — no distingue alumnos creados
-- después de esta migración de los que ya eran viewer antes).
UPDATE users SET role = 'viewer' WHERE role = 'alumno';

-- NOTE: PostgreSQL no soporta remover valores de un enum una vez agregados.
-- 'profesor' y 'alumno' permanecen en user_role/team_role tras el rollback.
