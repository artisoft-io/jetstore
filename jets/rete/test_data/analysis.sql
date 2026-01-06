
SELECT DISTINCT s.value as subject, p.value as predicate, o.value as object FROM triples t3, resources s, resources p, resources o, rule_sequences rs, main_rule_sets mrs WHERE t3.subject_key=s.key and t3.predicate_key=p.key AND t3.object_key=o.key AND t3.source_file_key = mrs.ruleset_file_key AND mrs.rule_sequence_key = rs.key AND rs.name = 'MSK' order by subject, predicate, object

select rn.vertex, rn.parent_vertex, rn.consequent_seq, rn.normalizedLabel --, wc.source_file_name, wc.key
FROM rete_nodes rn, workspace_control wc
WHERE rn.source_file_key = wc.key
AND wc.source_file_name = 'jet_rules/eligibility/map_eligibility_main.jr'
order by rn.vertex, rn.consequent_seq, rn.normalizedLabel