-- Sets every email address in a jetsapi user_email column to the stack's admin.
-- A client configuration file records who exported it, and that is a real
-- person's address, which the screens would otherwise show. Names that are not
-- addresses ('admin', 'system') are left alone. Run by up.sh with -v admin=<email>.
SET shots.admin = :'admin';
DO $$
DECLARE
  t record;
  admin text := current_setting('shots.admin');
BEGIN
  FOR t IN
    SELECT c.table_name
      FROM information_schema.columns c
      JOIN information_schema.tables tb
        ON tb.table_schema = c.table_schema AND tb.table_name = c.table_name
     WHERE c.table_schema = 'jetsapi' AND c.column_name = 'user_email'
       AND tb.table_type = 'BASE TABLE'
  LOOP
    EXECUTE format('UPDATE jetsapi.%I SET user_email = $1 WHERE user_email LIKE ''%%@%%'' AND user_email <> $1',
                   t.table_name)
      USING admin;
  END LOOP;
END
$$;
