\copy (
SELECT t.table_schema || '.' || t.table_name as table_name,
       COALESCE(json_object_agg(ccu.column_name, ccu.table_schema || '.' || ccu.table_name) FILTER (WHERE ccu.column_name IS NOT NULL), '{}') fks,
       pg_total_relation_size(t.table_schema || '.' || t.table_name) as relation_size
  FROM information_schema.tables AS t
  LEFT JOIN information_schema.table_constraints AS tc ON (t.table_schema = tc.table_schema AND t.table_name = tc.table_name AND tc.constraint_type = 'FOREIGN KEY')
  LEFT JOIN information_schema.constraint_column_usage AS ccu ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema
 WHERE t.table_schema IN ('public') -- ('public', 'active_learning', 'model_playground')
 GROUP BY 1
) TO '/tmp/table_sizes.csv' (FORMAT csv, HEADER);
