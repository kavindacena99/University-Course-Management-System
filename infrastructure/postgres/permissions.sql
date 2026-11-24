\set ON_ERROR_STOP on
GRANT USAGE ON SCHEMA public TO :"runtime";
GRANT SELECT ON ALL TABLES IN SCHEMA public TO :"runtime";
-- Never transiently grant migration-history writes, even during repeat deployments.
SELECT format('GRANT INSERT, UPDATE, DELETE ON TABLE %I.%I TO %I', schemaname, tablename,
              regexp_replace(current_user::text, '_migrator$', '_app'))
FROM pg_tables WHERE schemaname='public' AND tablename <> 'schema_migrations'
\gexec
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO :"runtime";
REVOKE INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER, MAINTAIN ON public.schema_migrations FROM :"runtime";
GRANT SELECT ON public.schema_migrations TO :"runtime";
