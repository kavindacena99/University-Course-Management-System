INSERT INTO auth_permissions (code, description) VALUES
    ('enrollments:read', 'View authorized student enrollment records'),
    ('enrollments:manage', 'Create and cancel student enrollments')
ON CONFLICT (code) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO auth_role_permissions (role_code, permission_code)
SELECT 'ADMIN', code
FROM auth_permissions
WHERE code IN ('enrollments:read', 'enrollments:manage')
ON CONFLICT DO NOTHING;

INSERT INTO auth_role_permissions (role_code, permission_code)
VALUES ('LECTURER', 'enrollments:read')
ON CONFLICT DO NOTHING;
