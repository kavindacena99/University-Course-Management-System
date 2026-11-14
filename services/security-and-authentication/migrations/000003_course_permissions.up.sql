INSERT INTO auth_permissions (code, description) VALUES
    ('courses:manage', 'Create, update, publish, archive, and restore courses'),
    ('offerings:manage', 'Create and manage course offerings')
ON CONFLICT (code) DO UPDATE
    SET description = EXCLUDED.description;

INSERT INTO auth_role_permissions (role_code, permission_code)
SELECT 'ADMIN', code
FROM auth_permissions
WHERE code IN ('courses:manage', 'offerings:manage')
ON CONFLICT DO NOTHING;

INSERT INTO auth_role_permissions (role_code, permission_code)
VALUES ('LECTURER', 'offerings:manage')
ON CONFLICT DO NOTHING;
