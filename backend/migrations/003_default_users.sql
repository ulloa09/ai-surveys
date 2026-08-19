-- +goose Up

-- Usuarios sembrados para desarrollo local. Los tres comparten la contraseña
INSERT INTO users (email, display_name, role, password_hash) VALUES
    ('superadmin@test.com', 'Super Admin', 'super_admin', '$2a$10$tF2H/w3IHnxKMtb.cgTWDOVZrpdV1xW6gWzK1nqAbbIZNizS90X2O'),
    ('admin@test.com',      'Admin',       'admin',       '$2a$10$neiE6mvRkw6yvbCl0xPfQeawIrI6ogVo8kZIbgT4uMDZJTyzSU5gG'),
    ('member@test.com',     'Viewer',      'viewer',      '$2a$10$neiE6mvRkw6yvbCl0xPfQeawIrI6ogVo8kZIbgT4uMDZJTyzSU5gG');

-- +goose Down
DELETE FROM users WHERE email IN (
    'superadmin@test.com',
    'admin@test.com',
    'member@test.com'
);