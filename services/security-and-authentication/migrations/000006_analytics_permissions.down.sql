DELETE FROM auth_role_permissions
 WHERE permission_code LIKE 'analytics:%';

DELETE FROM auth_permissions
 WHERE code LIKE 'analytics:%';
