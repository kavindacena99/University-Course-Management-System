DELETE FROM auth_permissions
WHERE code IN ('enrollments:read', 'enrollments:manage');
