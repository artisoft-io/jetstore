// Tests for rule_reading.mjs: run with `npm test` in docs/, or `node --test scripts/`.
//
// The fixture is a workspace.db built from jets/workspace_schema.sql and holding the
// compiled network rows of ten jets_ws rules, as the compiler wrote them (jets_ws's
// workspace.db, compiled 2026-10-22), plus rules made up here: one to exercise what the
// reading does not render, six that call substring_of and five that call
// replace_char_of. The rules' text (jet_rules.label) is deliberately left out: the
// reading comes from the network alone. Each expected reading was checked by hand
// against its rule's text, which is quoted above it in a comment.
import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { DatabaseSync } from 'node:sqlite';
import { after, before, test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { readRule } from './rule_reading.mjs';

const REPO_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');
const SCHEMA = join(REPO_ROOT, 'jets', 'workspace_schema.sql');

const FIXTURE = {
	jet_rules: {
		columns: ["key","name","source_file_key"],
		rows: [
			[1,"HC_MedServiceDateOverride10",1],
			[16,"AM2_NewMedEvent10",1],
			[17,"AM2_NewMedEvent15",1],
			[30,"AM_PGuardNDC10",1],
			[35,"CE_MedNbrDays10",1],
			[36,"CE_RxDateRange10",1],
			[57,"BP_RxFillDates10",1],
			[113,"UT_LOOP1_10",10],
			[114,"UT_LOOP1_20",10],
			[122,"UT_REENT_10",11],
		],
	},
	rule_terms: {
		columns: ["rule_key","rete_node_key","is_antecedent"],
		rows: [
			[1,2,1],
			[1,3,1],
			[1,166,0],
			[16,20,1],
			[16,21,1],
			[16,22,1],
			[16,187,0],
			[17,20,1],
			[17,23,1],
			[17,24,1],
			[17,25,1],
			[17,26,1],
			[17,27,1],
			[17,188,0],
			[17,189,0],
			[17,190,0],
			[17,191,0],
			[17,192,0],
			[17,193,0],
			[17,194,0],
			[30,58,1],
			[30,59,1],
			[30,212,0],
			[35,76,1],
			[35,78,1],
			[35,79,1],
			[35,229,0],
			[36,80,1],
			[36,81,1],
			[36,230,0],
			[36,231,0],
			[36,232,0],
			[36,233,0],
			[57,118,1],
			[57,260,0],
			[57,261,0],
			[113,352,1],
			[113,353,1],
			[113,354,1],
			[113,364,0],
			[114,355,1],
			[114,356,1],
			[114,365,0],
			[114,366,0],
			[122,375,1],
			[122,376,1],
			[122,380,0],
		],
	},
	rete_nodes: {
		columns: ["key","vertex","type","subject_key","predicate_key","object_key","obj_expr_key","filter_expr_key","parent_vertex","source_file_key","is_negation","consequent_seq"],
		rows: [
			[2,1,"antecedent",285,27,114,null,null,0,1,0,0],
			[3,2,"antecedent",286,120,287,null,null,1,1,0,0],
			[20,19,"antecedent",285,27,194,null,null,0,1,0,0],
			[21,20,"antecedent",286,229,287,null,null,19,1,0,0],
			[22,21,"antecedent",286,196,290,null,null,20,1,0,0],
			[23,22,"antecedent",286,195,287,null,null,19,1,0,0],
			[24,23,"antecedent",286,229,290,null,null,22,1,0,0],
			[25,24,"antecedent",286,196,294,null,null,23,1,0,0],
			[26,25,"antecedent",321,322,296,null,null,24,1,0,0],
			[27,26,"antecedent",295,102,300,null,null,25,1,0,0],
			[58,57,"antecedent",285,27,159,null,7,0,1,0,0],
			[59,58,"antecedent",286,131,287,null,null,57,1,0,0],
			[76,75,"antecedent",285,27,149,null,null,0,1,0,0],
			[78,77,"antecedent",286,145,287,null,null,75,1,0,0],
			[79,78,"antecedent",286,146,290,null,null,77,1,0,0],
			[80,79,"antecedent",285,27,159,null,null,0,1,0,0],
			[81,80,"antecedent",286,226,250,null,null,79,1,0,0],
			[118,117,"antecedent",285,27,173,null,null,0,1,0,0],
			[166,2,"consequent",286,94,null,8,null,0,1,null,1],
			[187,21,"consequent",288,321,null,26,null,0,1,null,1],
			[188,26,"consequent",299,27,null,30,null,0,1,null,1],
			[189,26,"consequent",299,144,null,31,null,0,1,null,2],
			[190,26,"consequent",299,233,null,29,null,0,1,null,3],
			[191,26,"consequent",289,141,null,32,null,0,1,null,4],
			[192,26,"consequent",299,234,null,35,null,0,1,null,5],
			[193,26,"consequent",299,235,null,37,null,0,1,null,6],
			[194,26,"consequent",299,226,null,38,null,0,1,null,7],
			[212,58,"consequent",286,164,null,71,null,0,1,null,1],
			[229,78,"consequent",286,147,null,88,null,0,1,null,1],
			[230,80,"consequent",286,145,null,90,null,0,1,null,1],
			[231,80,"consequent",286,146,null,91,null,0,1,null,2],
			[232,80,"consequent",286,160,null,93,null,0,1,null,3],
			[233,80,"consequent",286,161,null,95,null,0,1,null,4],
			[260,117,"consequent",286,175,null,106,null,0,1,null,1],
			[261,117,"consequent",286,273,null,107,null,0,1,null,2],
			[352,2,"antecedent",285,27,25,null,null,0,10,0,0],
			[353,3,"antecedent",286,331,287,null,null,2,10,0,0],
			[354,4,"antecedent",286,333,288,null,null,3,10,1,0],
			[355,5,"antecedent",285,27,25,null,155,0,10,0,0],
			[356,6,"antecedent",286,14,287,null,null,5,10,0,0],
			[364,4,"consequent",286,332,null,159,null,0,10,null,1],
			[365,6,"consequent",286,334,null,159,null,0,10,null,1],
			[366,6,"consequent",286,335,null,161,null,0,10,null,2],
			[375,1,"antecedent",285,27,338,null,null,0,11,0,0],
			[376,2,"antecedent",286,339,287,null,171,1,11,0,0],
			[380,2,"consequent",286,340,null,167,null,0,11,null,1],
		],
	},
	expressions: {
		columns: ["key","type","arg0_key","arg1_key","op","source_file_key"],
		rows: [
			[1,"resource",286,null,null,1],
			[6,"resource",126,null,null,1],
			[7,"binary",1,6,"exist_not",1],
			[8,"resource",288,null,null,1],
			[9,"resource",232,null,null,1],
			[19,"resource",259,null,null,1],
			[26,"unary",9,null,"create_entity",1],
			[29,"resource",293,null,null,1],
			[30,"resource",155,null,null,1],
			[31,"resource",238,null,null,1],
			[32,"resource",299,null,null,1],
			[33,"resource",202,null,null,1],
			[34,"resource",301,null,null,1],
			[35,"binary",33,34,"multi_lookup",1],
			[36,"resource",219,null,null,1],
			[37,"binary",36,34,"lookup",1],
			[38,"resource",233,null,null,1],
			[41,"resource",321,null,null,1],
			[70,"resource",253,null,null,1],
			[71,"binary",70,8,"+",1],
			[87,"binary",41,8,"-",1],
			[88,"binary",87,19,"+",1],
			[89,"resource",260,null,null,1],
			[90,"binary",1,89,"min_of",1],
			[91,"binary",1,89,"max_of",1],
			[92,"resource",261,null,null,1],
			[93,"binary",1,92,"sum_values",1],
			[94,"resource",262,null,null,1],
			[95,"binary",1,94,"sum_values",1],
			[105,"resource",174,null,null,1],
			[106,"binary",1,105,"size_of",1],
			[107,"binary",1,105,"join_values",1],
			[151,"resource",286,null,null,10],
			[152,"resource",331,null,null,10],
			[153,"binary",151,152,"size_of",10],
			[154,"resource",232,null,null,10],
			[155,"binary",153,154,">",10],
			[159,"resource",288,null,null,10],
			[160,"resource",336,null,null,10],
			[161,"binary",151,160,"sorted_head",10],
			[164,"resource",9,null,null,11],
			[165,"resource",6,null,null,11],
			[166,"binary",164,165,"exist_not",11],
			[167,"resource",288,null,null,11],
			[168,"unary",167,null,"not",11],
			[169,"resource",259,null,null,11],
			[170,"binary",168,169,"!=",11],
			[171,"binary",166,170,"and",11],
		],
	},
	resources: {
		columns: ["key","type","id","value","is_binded","source_file_key"],
		rows: [
			[5,"resource","jets:entity_property","jets:entity_property",null,0],
			[6,"resource","jets:exception","jets:exception",null,0],
			[9,"resource","jets:iState","jets:iState",null,0],
			[14,"resource","jets:loop","jets:loop",null,0],
			[16,"resource","jets:operator","jets:operator",null,0],
			[25,"resource","jets:State","jets:State",null,0],
			[26,"resource","jets:value_property","jets:value_property",null,0],
			[27,"resource","rdf:type","rdf:type",null,0],
			[94,"resource","hc:Service_Date","hc:Service_Date",null,0],
			[102,"resource","hc:Diagnosis_Codes","hc:Diagnosis_Codes",null,0],
			[114,"resource","hc:MedicalClaim","hc:MedicalClaim",null,0],
			[120,"resource","hc:Service_Date_From","hc:Service_Date_From",null,0],
			[126,"resource","hc:Drug_Name","hc:Drug_Name",null,0],
			[131,"resource","hc:NDC","hc:NDC",null,0],
			[136,"resource","hc:Days_Supply","hc:Days_Supply",null,0],
			[137,"resource","hc:Quantity_Dispensed","hc:Quantity_Dispensed",null,0],
			[141,"resource","cintel:has_Medical_Events","cintel:has_Medical_Events",null,0],
			[144,"resource","cintel:Type","cintel:Type",null,0],
			[145,"resource","cintel:Date_From","cintel:Date_From",null,0],
			[146,"resource","cintel:Date_To","cintel:Date_To",null,0],
			[147,"resource","cintel:Span_Days","cintel:Span_Days",null,0],
			[149,"resource","cintel:Medical_Event","cintel:Medical_Event",null,0],
			[155,"resource","cintel:Medical_EventV2","cintel:Medical_EventV2",null,0],
			[159,"resource","cintel:Pharmacy_Event","cintel:Pharmacy_Event",null,0],
			[160,"resource","cintel:Total_Days_Supply","cintel:Total_Days_Supply",null,0],
			[161,"resource","cintel:Total_Quantity_Dispensed","cintel:Total_Quantity_Dispensed",null,0],
			[164,"resource","cintel:Pharmacy_Flags","cintel:Pharmacy_Flags",null,0],
			[173,"resource","cintel:Briefing_Pharmacy_Event","cintel:Briefing_Pharmacy_Event",null,0],
			[174,"resource","cintel:Fill_Date","cintel:Fill_Date",null,0],
			[175,"resource","cintel:Fill_Count","cintel:Fill_Count",null,0],
			[194,"resource","am:AnalysisRoot","am:AnalysisRoot",null,0],
			[195,"resource","am:hasPatientProfile","am:hasPatientProfile",null,0],
			[196,"resource","am:hasMedicalClaims","am:hasMedicalClaims",null,0],
			[202,"resource","ElixhauserConditionsLookup","ElixhauserConditionsLookup",null,0],
			[219,"resource","DiagnosisDescriptionLookup","DiagnosisDescriptionLookup",null,0],
			[226,"volatile_resource","tag","tag",null,0],
			[229,"volatile_resource","medicalEventIndex","medicalEventIndex",null,0],
			[232,"int",null,"0",null,0],
			[233,"volatile_resource","hasMedicalClaim","hasMedicalClaim",null,0],
			[234,"volatile_resource","lookupElixhauserConditions","lookupElixhauserConditions",null,0],
			[235,"volatile_resource","lookupDiagnosisDesc","lookupDiagnosisDesc",null,0],
			[238,"text",null,"medical_claim",null,0],
			[250,"volatile_resource","hasPharmacyClaim","hasPharmacyClaim",null,0],
			[253,"text",null,"Unresolved NDC in DrugInfoLookup: ",null,0],
			[259,"int",null,"1",null,0],
			[260,"resource","serviceDatesConfig","serviceDatesConfig",null,0],
			[261,"resource","daysSupplyConfig","daysSupplyConfig",null,0],
			[262,"resource","totalQtyDispensedConfig","totalQtyDispensedConfig",null,0],
			[273,"volatile_resource","fillDateList","fillDateList",null,0],
			[285,"var","?x01",null,0,0],
			[286,"var","?x01",null,1,0],
			[287,"var","?x02",null,0,0],
			[288,"var","?x02",null,1,0],
			[289,"var","?x02",null,1,0],
			[290,"var","?x03",null,0,0],
			[293,"var","?x04",null,1,0],
			[294,"var","?x04",null,0,0],
			[295,"var","?x04",null,1,0],
			[296,"var","?x05",null,0,0],
			[299,"var","?x05",null,1,0],
			[300,"var","?x06",null,0,0],
			[301,"var","?x06",null,1,0],
			[321,"var","?x03",null,1,0],
			[322,"var","?x04",null,1,0],
			[329,"resource","ut_loop1:weight","ut_loop1:weight",null,0],
			[331,"resource","ut_loop1:matchList","ut_loop1:matchList",null,0],
			[332,"resource","ut_loop1:priorityList","ut_loop1:priorityList",null,0],
			[333,"resource","ut_loop1:mergeList","ut_loop1:mergeList",null,0],
			[334,"resource","ut_loop1:matchLoop","ut_loop1:matchLoop",null,0],
			[335,"resource","ut_loop1:topMatch","ut_loop1:topMatch",null,0],
			[336,"resource","ut_loop1:iterConfig","ut_loop1:iterConfig",null,0],
			[337,"text",null,"<",null,0],
			[338,"resource","ut_reent:Main","ut_reent:Main",null,0],
			[339,"resource","ut_reent:value","ut_reent:value",null,0],
			[340,"resource","ut_reent:kept","ut_reent:kept",null,0],
		],
	},
	triples: {
		columns: ["subject_key","predicate_key","object_key","source_file_key"],
		rows: [
			[260,5,250,1],
			[260,26,94,1],
			[261,5,250,1],
			[261,26,136,1],
			[262,5,250,1],
			[262,26,137,1],
			[336,5,332,10],
			[336,26,329,10],
			[336,16,337,10],
		],
	},
	domain_classes: {
		columns: ["key","name","source_file_key"],
		rows: [
			[4,"jets:State",2],
			[14,"hc:MedicalClaim",3],
			[19,"cintel:Medical_Event",4],
			[22,"cintel:Medical_EventV2",4],
			[23,"cintel:Pharmacy_Event",4],
			[26,"cintel:Briefing_Pharmacy_Event",4],
			[28,"am:AnalysisRoot",5],
			[30,"ut_reent:Main",11],
		],
	},
	data_properties: {
		columns: ["key","domain_class_key","name","type"],
		rows: [
			[6,2,"rdf:type","resource"],
			[10,4,"jets:exception","text"],
			[13,4,"jets:loop","int"],
			[73,11,"hc:Service_Date","date"],
			[80,12,"hc:Diagnosis_Codes","text"],
			[96,14,"hc:Service_Date_From","date"],
			[101,15,"hc:Drug_Name","text"],
			[106,15,"hc:NDC","text"],
			[110,16,"hc:Days_Supply","double"],
			[111,16,"hc:Quantity_Dispensed","double"],
			[114,18,"cintel:Type","text"],
			[115,18,"cintel:Date_From","date"],
			[116,18,"cintel:Date_To","date"],
			[117,18,"cintel:Span_Days","int"],
			[125,23,"cintel:Total_Days_Supply","double"],
			[126,23,"cintel:Total_Quantity_Dispensed","double"],
			[129,23,"cintel:Pharmacy_Flags","text"],
			[136,26,"cintel:Fill_Date","date"],
			[137,26,"cintel:Fill_Count","int"],
			[153,29,"ut_loop1:weight","int"],
			[155,30,"ut_reent:value","text"],
			[156,30,"ut_reent:kept","text"],
		],
	},
	object_properties: {
		columns: ["key","domain_class_key","name","type"],
		rows: [
			[1,17,"cintel:has_Medical_Events","resource"],
			[5,28,"am:hasPatientProfile","resource"],
			[6,28,"am:hasMedicalClaims","resource"],
		],
	},
	lookup_tables: {
		columns: ["key","name","source_file_key"],
		rows: [
			[1,"ElixhauserConditionsLookup",6],
			[4,"DiagnosisDescriptionLookup",8],
		],
	},
};

// Rules of the fixture's own. FIXTURE_Unrendered has two variables of one class, a
// filter calling a binary operator no engine has, and a consequent calling a unary one.
// The FIXTURE_Substring rules share two antecedents,
//   (?clm rdf:type hc:MedicalClaim).(?clm hc:Drug_Name ?name)
// and each sets hc:NDC to (config substring_of ?name), the configuration resource
// declared in the rule's main file, as triple(firstFiveConfig, jets:from, 0); and so on.
// noConfig has no triples, textLengthConfig a jets:length that is text, and
// ruleSetConfig a jets:length that a rule of the same file writes. The
// FIXTURE_ReplaceChars rules share the same antecedents and set hc:NDC to
// (config replace_char_of ?name), as declared by triple(dashDotConfig,
// jets:replace_chars, "-.") and triple(dashDotConfig, jets:replace_with, "_"); and so on.
// noConfig has no triples, numberCharsConfig a jets:replace_chars that is an int,
// repeatedWithConfig two jets:replace_with, emptyCharsConfig no characters to replace,
// accentConfig a character outside ASCII, expandingConfig a replacement holding a
// character the engine replaces after the first, and ruleReplaceConfig a
// jets:replace_with that a rule of the same file writes. Rules with the same
// antecedents share the terminal vertex, so their consequents are numbered along it.
const MADE_UP = {
	jet_rules: [
		[900, 'FIXTURE_Unrendered', 1],
		[910, 'FIXTURE_SubstringFirst', 1],
		[911, 'FIXTURE_SubstringDropLast', 1],
		[912, 'FIXTURE_SubstringFrom', 1],
		[913, 'FIXTURE_SubstringTrimBoth', 1],
		[914, 'FIXTURE_SubstringUnconfigured', 1],
		[915, 'FIXTURE_SubstringSetByRule', 1],
		[920, 'FIXTURE_ReplaceChars', 1],
		[921, 'FIXTURE_ReplaceCharsRemoved', 1],
		[922, 'FIXTURE_ReplaceCharsNamed', 1],
		[923, 'FIXTURE_ReplaceCharsUnconfigured', 1],
		[924, 'FIXTURE_ReplaceCharsSetByRule', 1],
	],
	rule_terms: [
		[900, 9001, 1],
		[900, 9002, 1],
		[900, 9003, 0],
		...[910, 911, 912, 913, 914, 915, 920, 921, 922, 923, 924].flatMap((rule) => [
			[rule, 9301, 1],
			[rule, 9302, 1],
		]),
		[910, 9310, 0],
		[911, 9311, 0],
		[912, 9312, 0],
		[913, 9313, 0],
		[914, 9314, 0],
		[914, 9315, 0],
		[915, 9316, 0],
		[915, 9317, 0],
		[920, 9320, 0],
		[920, 9321, 0],
		[921, 9322, 0],
		[922, 9323, 0],
		[922, 9324, 0],
		...[9325, 9326, 9327, 9328, 9329, 9330].map((node) => [923, node, 0]),
		[924, 9331, 0],
		[924, 9332, 0],
	],
	rete_nodes: [
		[9001, 900, 'antecedent', 285, 27, 114, null, null, 0, 1, 0, 0],
		[9002, 901, 'antecedent', 287, 27, 114, null, 9101, 900, 1, 0, 0],
		[9003, 901, 'consequent', 286, 94, null, 9104, null, 0, 1, null, 1],
		[9301, 910, 'antecedent', 285, 27, 114, null, null, 0, 1, 0, 0],
		[9302, 911, 'antecedent', 286, 126, 287, null, null, 910, 1, 0, 0],
		[9310, 911, 'consequent', 286, 131, null, 9410, null, 0, 1, null, 1],
		[9311, 911, 'consequent', 286, 131, null, 9411, null, 0, 1, null, 2],
		[9312, 911, 'consequent', 286, 131, null, 9412, null, 0, 1, null, 3],
		[9313, 911, 'consequent', 286, 131, null, 9413, null, 0, 1, null, 4],
		[9314, 911, 'consequent', 286, 131, null, 9414, null, 0, 1, null, 5],
		[9315, 911, 'consequent', 286, 131, null, 9415, null, 0, 1, null, 6],
		[9316, 911, 'consequent', 9207, 9202, null, 9420, null, 0, 1, null, 7],
		[9317, 911, 'consequent', 286, 131, null, 9416, null, 0, 1, null, 8],
		[9320, 911, 'consequent', 286, 131, null, 9451, null, 0, 1, null, 9],
		[9321, 911, 'consequent', 286, 131, null, 9462, null, 0, 1, null, 10],
		[9322, 911, 'consequent', 286, 131, null, 9452, null, 0, 1, null, 11],
		[9323, 911, 'consequent', 286, 131, null, 9453, null, 0, 1, null, 12],
		[9324, 911, 'consequent', 286, 131, null, 9454, null, 0, 1, null, 13],
		[9325, 911, 'consequent', 286, 131, null, 9461, null, 0, 1, null, 14],
		[9326, 911, 'consequent', 286, 131, null, 9455, null, 0, 1, null, 15],
		[9327, 911, 'consequent', 286, 131, null, 9456, null, 0, 1, null, 16],
		[9328, 911, 'consequent', 286, 131, null, 9457, null, 0, 1, null, 17],
		[9329, 911, 'consequent', 286, 131, null, 9458, null, 0, 1, null, 18],
		[9330, 911, 'consequent', 286, 131, null, 9459, null, 0, 1, null, 19],
		[9331, 911, 'consequent', 9232, 9222, null, 9463, null, 0, 1, null, 20],
		[9332, 911, 'consequent', 286, 131, null, 9460, null, 0, 1, null, 21],
	],
	expressions: [
		[9101, 'binary', 9102, 9103, 'some_other_op', 1],
		[9102, 'resource', 253, null, null, 1],
		[9103, 'resource', 288, null, null, 1],
		[9104, 'unary', 9103, null, 'some_future_op', 1],
		[9400, 'resource', 288, null, null, 1],
		[9401, 'resource', 9203, null, null, 1],
		[9402, 'resource', 9204, null, null, 1],
		[9403, 'resource', 9205, null, null, 1],
		[9404, 'resource', 9209, null, null, 1],
		[9405, 'resource', 9206, null, null, 1],
		[9406, 'resource', 9208, null, null, 1],
		[9407, 'resource', 9207, null, null, 1],
		[9410, 'binary', 9401, 9400, 'substring_of', 1],
		[9411, 'binary', 9402, 9400, 'substring_of', 1],
		[9412, 'binary', 9403, 9400, 'substring_of', 1],
		[9413, 'binary', 9404, 9400, 'substring_of', 1],
		[9414, 'binary', 9405, 9400, 'substring_of', 1],
		[9415, 'binary', 9406, 9400, 'substring_of', 1],
		[9416, 'binary', 9407, 9400, 'substring_of', 1],
		[9420, 'resource', 9211, null, null, 1],
		// replace_char_of: a leaf per configuration resource, 9431 to 9441, and its call, 9451 to 9461.
		...[9223, 9224, 9225, 9226, 9227, 9228, 9229, 9230, 9231, 9232, 9206].flatMap((cfg, i) => [
			[9431 + i, 'resource', cfg, null, null, 1],
			[9451 + i, 'binary', 9431 + i, 9400, 'replace_char_of', 1],
		]),
		[9462, 'binary', 9451, 9400, '+', 1],
		[9463, 'resource', 9241, null, null, 1],
	],
	resources: [
		[9201, 'resource', 'jets:from', 'jets:from', null, 0],
		[9202, 'resource', 'jets:length', 'jets:length', null, 0],
		[9203, 'resource', 'firstFiveConfig', 'firstFiveConfig', null, 0],
		[9204, 'resource', 'dropLastThreeConfig', 'dropLastThreeConfig', null, 0],
		[9205, 'resource', 'fromTwoConfig', 'fromTwoConfig', null, 0],
		[9206, 'resource', 'noConfig', 'noConfig', null, 0],
		[9207, 'resource', 'ruleSetConfig', 'ruleSetConfig', null, 0],
		[9208, 'resource', 'textLengthConfig', 'textLengthConfig', null, 0],
		[9209, 'resource', 'trimBothConfig', 'trimBothConfig', null, 0],
		[9211, 'int', null, '5', null, 0],
		[9212, 'int', null, '-3', null, 0],
		[9213, 'int', null, '2', null, 0],
		[9214, 'int', null, '4', null, 0],
		[9216, 'int', null, '-1', null, 0],
		[9217, 'text', null, '5', null, 0],
		[9221, 'resource', 'jets:replace_chars', 'jets:replace_chars', null, 0],
		[9222, 'resource', 'jets:replace_with', 'jets:replace_with', null, 0],
		[9223, 'resource', 'dashDotConfig', 'dashDotConfig', null, 0],
		[9224, 'resource', 'dashRemoveConfig', 'dashRemoveConfig', null, 0],
		[9225, 'resource', 'tabQuoteConfig', 'tabQuoteConfig', null, 0],
		[9226, 'resource', 'slashSpaceConfig', 'slashSpaceConfig', null, 0],
		[9227, 'resource', 'numberCharsConfig', 'numberCharsConfig', null, 0],
		[9228, 'resource', 'repeatedWithConfig', 'repeatedWithConfig', null, 0],
		[9229, 'resource', 'emptyCharsConfig', 'emptyCharsConfig', null, 0],
		[9230, 'resource', 'accentConfig', 'accentConfig', null, 0],
		[9231, 'resource', 'expandingConfig', 'expandingConfig', null, 0],
		[9232, 'resource', 'ruleReplaceConfig', 'ruleReplaceConfig', null, 0],
		[9240, 'text', null, '-.', null, 0],
		[9241, 'text', null, '_', null, 0],
		[9242, 'text', null, '-', null, 0],
		[9243, 'text', null, '', null, 0],
		[9244, 'text', null, '\t"', null, 0],
		[9245, 'text', null, ' ', null, 0],
		[9246, 'text', null, '/ ', null, 0],
		[9247, 'text', null, '\u00e9', null, 0],
		[9248, 'text', null, 'e', null, 0],
		[9249, 'text', null, '.-', null, 0],
		[9250, 'text', null, '-x', null, 0],
	],
	triples: [
		[9203, 9201, 232, 1],
		[9203, 9202, 9211, 1],
		[9204, 9201, 232, 1],
		[9204, 9202, 9212, 1],
		[9205, 9201, 9213, 1],
		[9205, 9202, 9214, 1],
		[9207, 9201, 232, 1],
		[9207, 9202, 9214, 1],
		[9208, 9201, 232, 1],
		[9208, 9202, 9217, 1],
		[9209, 9201, 259, 1],
		[9209, 9202, 9216, 1],
		[9223, 9221, 9240, 1],
		[9223, 9222, 9241, 1],
		[9224, 9221, 9242, 1],
		[9224, 9222, 9243, 1],
		[9225, 9221, 9244, 1],
		[9225, 9222, 9245, 1],
		[9226, 9221, 9246, 1],
		[9226, 9222, 9242, 1],
		[9227, 9221, 259, 1],
		[9227, 9222, 9241, 1],
		[9228, 9221, 9242, 1],
		[9228, 9222, 9241, 1],
		[9228, 9222, 9242, 1],
		[9229, 9221, 9243, 1],
		[9229, 9222, 9241, 1],
		[9230, 9221, 9247, 1],
		[9230, 9222, 9248, 1],
		[9231, 9221, 9249, 1],
		[9231, 9222, 9250, 1],
		[9232, 9221, 9242, 1],
		[9232, 9222, 9241, 1],
	],
};

/** The FIXTURE_Substring rules' antecedents, read. */
const SUBSTRING_WHEN = ['there is a MedicalClaim', 'the MedicalClaim has a Drug_Name'];

const EXPECTED = [
	// [HC_MedServiceDateOverride10]:(?clm01 rdf:type hc:MedicalClaim).(?clm01 hc:Service_Date_From ?date1)
	// -> (?clm01 hc:Service_Date ?date1);
	{
		key: 1,
		name: "HC_MedServiceDateOverride10",
		when: [
			"there is a MedicalClaim",
			"the MedicalClaim has a Service_Date_From",
		],
		then: [
			"the MedicalClaim has the Service_Date_From as its Service_Date",
		],
	},
	// [AM2_NewMedEvent10]:(?root rdf:type am:AnalysisRoot).(?root _0:medicalEventIndex ?index).(?root
	// am:hasMedicalClaims ?clm01) -> (?index ?clm01 create_entity(int(0)));
	{
		key: 16,
		name: "AM2_NewMedEvent10",
		when: [
			"there is an AnalysisRoot",
			"the AnalysisRoot has a medicalEventIndex",
			"the AnalysisRoot has a hasMedicalClaims",
		],
		then: [
			"the medicalEventIndex relates the hasMedicalClaims to a new entity",
		],
	},
	// [AM2_NewMedEvent15]:(?root rdf:type am:AnalysisRoot).(?root am:hasPatientProfile ?profile).(?root
	// _0:medicalEventIndex ?index).(?root am:hasMedicalClaims ?clm01).(?index ?clm01 ?entity).(?clm01
	// hc:Diagnosis_Codes ?code) -> (?entity rdf:type cintel:Medical_EventV2).(?entity cintel:Type
	// text(medical_claim)).(?entity _0:hasMedicalClaim ?clm01).(?profile cintel:has_Medical_Events
	// ?entity).(?entity _0:lookupElixhauserConditions (ElixhauserConditionsLookup multi_lookup
	// ?code)).(?entity _0:lookupDiagnosisDesc (DiagnosisDescriptionLookup lookup ?code)).(?entity _0:tag
	// _0:hasMedicalClaim);
	{
		key: 17,
		name: "AM2_NewMedEvent15",
		when: [
			"there is an AnalysisRoot",
			"the AnalysisRoot has a hasPatientProfile",
			"the AnalysisRoot has a medicalEventIndex",
			"the AnalysisRoot has a hasMedicalClaims",
			"the medicalEventIndex relates the hasMedicalClaims to a value",
			"the hasMedicalClaims has a Diagnosis_Codes",
		],
		then: [
			"the value is a Medical_EventV2",
			"the value has \"medical_claim\" as its Type",
			"the value has the hasMedicalClaims as its hasMedicalClaim",
			"the hasPatientProfile has the value as its has_Medical_Events",
			"the value has the rows of ElixhauserConditionsLookup for the Diagnosis_Codes as its lookupElixhauserConditions",
			"the value has the row of DiagnosisDescriptionLookup for the Diagnosis_Codes as its lookupDiagnosisDesc",
			"the value has hasMedicalClaim as its tag",
		],
	},
	// [AM_PGuardNDC10]:(?entity rdf:type cintel:Pharmacy_Event).[(?entity exist_not
	// hc:Drug_Name)].(?entity hc:NDC ?ndc) -> (?entity cintel:Pharmacy_Flags (text(Unresolved NDC in
	// DrugInfoLookup: ) + ?ndc));
	{
		key: 30,
		name: "AM_PGuardNDC10",
		when: [
			"there is a Pharmacy_Event, where the Pharmacy_Event has no Drug_Name",
			"the Pharmacy_Event has a NDC",
		],
		then: [
			"the Pharmacy_Event has \"Unresolved NDC in DrugInfoLookup: \" followed by the NDC as its Pharmacy_Flags",
		],
	},
	// [CE_MedNbrDays10]:(?entity rdf:type cintel:Medical_Event).(?entity cintel:Date_From ?from).(?entity
	// cintel:Date_To ?to) -> (?entity cintel:Span_Days ((?to - ?from) + int(1)));
	{
		key: 35,
		name: "CE_MedNbrDays10",
		when: [
			"there is a Medical_Event",
			"the Medical_Event has a Date_From",
			"the Medical_Event has a Date_To",
		],
		then: [
			"the Medical_Event has (the Date_To minus the Date_From) plus 1 as its Span_Days",
		],
	},
	// [CE_RxDateRange10, s=10]:(?entity rdf:type cintel:Pharmacy_Event).(?entity _0:tag
	// _0:hasPharmacyClaim) -> (?entity cintel:Date_From (?entity min_of serviceDatesConfig)).(?entity
	// cintel:Date_To (?entity max_of serviceDatesConfig)).(?entity cintel:Total_Days_Supply (?entity
	// sum_values daysSupplyConfig)).(?entity cintel:Total_Quantity_Dispensed (?entity sum_values
	// totalQtyDispensedConfig));
	{
		key: 36,
		name: "CE_RxDateRange10",
		when: [
			"there is a Pharmacy_Event",
			"the Pharmacy_Event has hasPharmacyClaim as its tag",
		],
		then: [
			"the Pharmacy_Event has the smallest Service_Date across the Pharmacy_Event's hasPharmacyClaim as its Date_From",
			"the Pharmacy_Event has the largest Service_Date across the Pharmacy_Event's hasPharmacyClaim as its Date_To",
			"the Pharmacy_Event has the sum of Days_Supply across the Pharmacy_Event's hasPharmacyClaim as its Total_Days_Supply",
			"the Pharmacy_Event has the sum of Quantity_Dispensed across the Pharmacy_Event's hasPharmacyClaim as its Total_Quantity_Dispensed",
		],
	},
	// [BP_RxFillDates10, s=10]:(?bevent rdf:type cintel:Briefing_Pharmacy_Event) -> (?bevent
	// cintel:Fill_Count (?bevent size_of cintel:Fill_Date)).(?bevent _0:fillDateList (?bevent join_values
	// cintel:Fill_Date));
	{
		key: 57,
		name: "BP_RxFillDates10",
		when: [
			"there is a Briefing_Pharmacy_Event",
		],
		then: [
			"the Briefing_Pharmacy_Event has the number of the Briefing_Pharmacy_Event's Fill_Date as its Fill_Count",
			"the Briefing_Pharmacy_Event has the Briefing_Pharmacy_Event's Fill_Date values joined with \", \" as its fillDateList",
		],
	},
	// [UT_LOOP1_10, s=100]:(?s rdf:type jets:State).(?s ut_loop1:matchList ?m).not(?s ut_loop1:mergeList
	// ?m) -> (?s ut_loop1:priorityList ?m);
	{
		key: 113,
		name: "UT_LOOP1_10",
		when: [
			"there is a State",
			"the State has a matchList",
			"the State has no mergeList equal to the matchList",
		],
		then: [
			"the State has the matchList as its priorityList",
		],
	},
	// [UT_LOOP1_20, s=10]:(?s rdf:type jets:State).[((?s size_of ut_loop1:matchList) > int(0))].(?s
	// jets:loop ?n) -> (?s ut_loop1:matchLoop ?n).(?s ut_loop1:topMatch (?s sorted_head
	// ut_loop1:iterConfig));
	{
		key: 114,
		name: "UT_LOOP1_20",
		when: [
			"there is a State, where the number of the State's matchList is greater than 0",
			"the State has a loop",
		],
		then: [
			"the State has the loop as its matchLoop",
			"the State has the one of the State's priorityList with the smallest weight as its topMatch",
		],
	},
	// [UT_REENT_10]:(?s rdf:type ut_reent:Main).(?s ut_reent:value ?v).[((jets:iState exist_not
	// jets:exception) and (not(?v) != int(1)))] -> (?s ut_reent:kept ?v);
	{
		key: 122,
		name: "UT_REENT_10",
		when: [
			"there is a Main",
			"the Main has a value, where jets:iState has no exception and (not the value) is not 1",
		],
		then: [
			"the Main has the value as its kept",
		],
	},
];

let dir;
let db;

function buildFixture(path) {
	const w = new DatabaseSync(path);
	w.exec(readFileSync(SCHEMA, 'utf8'));
	const insert = (table, columns, rows) => {
		const stmt = w.prepare(
			`INSERT INTO ${table} (${columns.join(', ')}) VALUES (${columns.map(() => '?').join(', ')})`,
		);
		for (const row of rows) stmt.run(...row);
	};
	for (const [table, { columns, rows }] of Object.entries(FIXTURE)) {
		insert(table, columns, rows);
		if (MADE_UP[table]) insert(table, columns, MADE_UP[table]);
	}
	w.close();
	return new DatabaseSync(path, { readOnly: true });
}

before(() => {
	dir = mkdtempSync(join(tmpdir(), 'rule-reading-'));
	db = buildFixture(join(dir, 'workspace.db'));
});

after(() => {
	db?.close();
	if (dir) rmSync(dir, { recursive: true, force: true });
});

for (const rule of EXPECTED) {
	test(`reads ${rule.name}`, () => {
		assert.deepEqual(readRule(db, rule.key), {
			when: rule.when,
			then: rule.then,
			partial: false,
			unrendered: [],
		});
	});
}

test('says what it does not render, and marks the reading partial', () => {
	assert.deepEqual(readRule(db, 900), {
		when: [
			'there is a first MedicalClaim',
			'there is a second MedicalClaim, where (not rendered: some_other_op)',
		],
		then: ['the first MedicalClaim has (not rendered: some_future_op) as its Service_Date'],
		partial: true,
		unrendered: ['some_future_op', 'some_other_op'],
	});
});

// substring_of (jets/rete/expr_op_strings.h, SubstringOfVisitor): jets:from characters
// are skipped, then jets:length kept, or when it is negative, that many dropped from the end.
const substringReading = (then) => ({ when: SUBSTRING_WHEN, then, partial: false, unrendered: [] });

test('reads substring_of with a positive length', () => {
	assert.deepEqual(
		readRule(db, 910),
		substringReading(['the MedicalClaim has the first 5 characters of the Drug_Name as its NDC']),
	);
});

test('reads substring_of with a negative length', () => {
	assert.deepEqual(
		readRule(db, 911),
		substringReading(['the MedicalClaim has the Drug_Name without its last 3 characters as its NDC']),
	);
});

test('reads substring_of from a position other than the start', () => {
	assert.deepEqual(
		readRule(db, 912),
		substringReading(['the MedicalClaim has the 4 characters of the Drug_Name after its first 2 characters as its NDC']),
	);
	assert.deepEqual(
		readRule(db, 913),
		substringReading(['the MedicalClaim has the Drug_Name without its first character and its last character as its NDC']),
	);
});

test('names the configuration of a substring_of it cannot read, rather than guess', () => {
	// No triples, and a jets:length that is text: the engine would raise an error for both.
	assert.deepEqual(
		readRule(db, 914),
		substringReading([
			'the MedicalClaim has the substring of the Drug_Name configured by noConfig as its NDC',
			'the MedicalClaim has the substring of the Drug_Name configured by textLengthConfig as its NDC',
		]),
	);
	// A rule writes jets:length, so the value in the file is not the one in force.
	assert.deepEqual(
		readRule(db, 915),
		substringReading([
			'ruleSetConfig has 5 as its length',
			'the MedicalClaim has the substring of the Drug_Name configured by ruleSetConfig as its NDC',
		]),
	);
});

// replace_char_of (jets/rete/expr_op_strings.h, ReplaceCharOfVisitor): every occurrence
// of each character of jets:replace_chars is replaced by jets:replace_with, in turn.
// These rules share the FIXTURE_Substring rules' antecedents, so substringReading fits.
test('reads replace_char_of', () => {
	assert.deepEqual(
		readRule(db, 920),
		substringReading([
			'the MedicalClaim has the Drug_Name with every "-" and "." replaced by "_" as its NDC',
			// The result is text, so + around it joins text.
			'the MedicalClaim has the Drug_Name with every "-" and "." replaced by "_" followed by the Drug_Name as its NDC',
		]),
	);
});

test('reads replace_char_of with an empty replacement as a removal', () => {
	assert.deepEqual(
		readRule(db, 921),
		substringReading(['the MedicalClaim has the Drug_Name with every "-" removed as its NDC']),
	);
});

test('names the characters of replace_char_of a quote would hide', () => {
	assert.deepEqual(
		readRule(db, 922),
		substringReading([
			'the MedicalClaim has the Drug_Name with every tab and double quote replaced by a space as its NDC',
			'the MedicalClaim has the Drug_Name with every "/" and space replaced by "-" as its NDC',
		]),
	);
});

test('names the configuration of a replace_char_of it cannot read, rather than guess', () => {
	// No triples, replace_chars an int, replace_with given twice, no characters to
	// replace (the engines disagree on it), a character outside ASCII (one engine
	// replaces bytes, the other characters), and a replacement holding "-", which the
	// engine then replaces again.
	assert.deepEqual(
		readRule(db, 923),
		substringReading(
			['noConfig', 'numberCharsConfig', 'repeatedWithConfig', 'emptyCharsConfig', 'accentConfig', 'expandingConfig'].map(
				(cfg) => `the MedicalClaim has the Drug_Name with characters replaced as configured by ${cfg} as its NDC`,
			),
		),
	);
	// A rule writes jets:replace_with, so the value in the file is not the one in force.
	assert.deepEqual(
		readRule(db, 924),
		substringReading([
			'ruleReplaceConfig has "_" as its replace_with',
			'the MedicalClaim has the Drug_Name with characters replaced as configured by ruleReplaceConfig as its NDC',
		]),
	);
});

test('gives the same text from the same database', () => {
	const keys = [...EXPECTED.map((r) => r.key), 900, 910, 911, 912, 913, 914, 915, 920, 921, 922, 923, 924];
	const first = keys.map((k) => readRule(db, k));
	const again = keys.map((k) => readRule(db, k));
	const other = buildFixture(join(dir, 'again.db'));
	try {
		assert.deepEqual(again, first);
		assert.deepEqual(
			keys.map((k) => readRule(other, k)),
			first,
		);
	} finally {
		other.close();
	}
});

test('refuses a rule the database does not hold', () => {
	assert.throws(() => readRule(db, 4242), /no rule with key 4242/);
});
