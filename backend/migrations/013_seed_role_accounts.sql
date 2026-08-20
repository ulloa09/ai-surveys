-- +goose Up

-- Reemplaza el seed de desarrollo para que exista exactamente una cuenta de
-- prueba por cada uno de los 4 roles reales de la plataforma. 'member@test.com'
-- (rol viejo 'viewer', ya no vigente) se elimina y se reemplaza por cuentas
-- 'profesor' y 'alumno' explícitas. superadmin@test.com y admin@test.com ya
-- estaban correctas y no cambian. Las 4 comparten la contraseña '12345678'.
DELETE FROM users WHERE email = 'member@test.com';

INSERT INTO users (email, display_name, role, password_hash) VALUES
    ('profesor@test.com', 'Profesor', 'profesor', '$2a$10$nyXKqej.SMH5M5chg2KXHuh7xKbEMmVzcherWsitBEbT4FFf.nRGO'),
    ('alumno@test.com',   'Alumno',   'alumno',   '$2a$10$2/ebhGhv08zSG73qCLaIpeFmZ9T..oecNV90lMixXZOZp6zRODCoi');

-- +goose Down
DELETE FROM users WHERE email IN ('profesor@test.com', 'alumno@test.com');

INSERT INTO users (email, display_name, role, password_hash) VALUES
    ('member@test.com', 'Viewer', 'alumno', '$2a$10$neiE6mvRkw6yvbCl0xPfQeawIrI6ogVo8kZIbgT4uMDZJTyzSU5gG');
