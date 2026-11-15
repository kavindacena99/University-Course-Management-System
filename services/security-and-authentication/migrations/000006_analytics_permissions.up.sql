INSERT INTO auth_permissions (code, description) VALUES
    ('analytics:student:read', 'View own academic analytics and GPA'),
    ('analytics:lecturer:read', 'View analytics for assigned offerings'),
    ('analytics:admin:read', 'View aggregate academic analytics'),
    ('analytics:admin:student:read', 'View individual student analytics'),
    ('analytics:manage', 'Configure Analytics grade bands and programmes')
ON CONFLICT (code) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO auth_role_permissions (role_code, permission_code) VALUES
    ('STUDENT', 'analytics:student:read'),
    ('LECTURER', 'analytics:lecturer:read'),
    ('ADMIN', 'analytics:admin:read'),
    ('ADMIN', 'analytics:manage')
ON CONFLICT DO NOTHING;
