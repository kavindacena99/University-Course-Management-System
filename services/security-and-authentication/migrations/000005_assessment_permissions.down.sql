DELETE FROM auth_role_permissions
WHERE permission_code LIKE 'assessments:%';

DELETE FROM auth_permissions
WHERE code LIKE 'assessments:%';
