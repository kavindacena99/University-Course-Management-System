INSERT INTO auth_permissions (code, description) VALUES
    ('assessments:read', 'View authorized assessment and result records'),
    ('assessments:manage', 'Create and manage offering assessments'),
    ('assessments:grade', 'Grade submissions in authorized offerings'),
    ('assessments:release', 'Release grades in authorized offerings'),
    ('assessments:submit', 'Submit work to assessments in active enrollments'),
    ('assessments:student:read', 'View individual student submissions and results'),
    ('assessments:results:import', 'Import approved final course results')
ON CONFLICT (code) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO auth_role_permissions (role_code, permission_code)
SELECT 'ADMIN', code
FROM auth_permissions
WHERE code LIKE 'assessments:%'
  AND code <> 'assessments:student:read'
ON CONFLICT DO NOTHING;

INSERT INTO auth_role_permissions (role_code, permission_code) VALUES
    ('LECTURER', 'assessments:read'),
    ('LECTURER', 'assessments:manage'),
    ('LECTURER', 'assessments:grade'),
    ('LECTURER', 'assessments:release'),
    ('STUDENT', 'assessments:submit')
ON CONFLICT DO NOTHING;
