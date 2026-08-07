// JetRule domain model in TypeScript.
//
// This module is a Data Model representation of the JetRule grammar
// defined by the JetRule ANTLR grammar
// (jets/compilerv2/compiler/JetRule.g4).

export type JetruleModel = 
  | {ctype: 'directive'; id:string; name:string; value:string}
  | {ctype: 'config'; id:string; max_looping:number}
  | {ctype: 'config'; id:string; max_rule_exec:number}
  | {ctype: 'config'; id:string; input_types:string[]}
  | {ctype: 'literal'; id:string; name:string; type:LiteralType; value:string}
  | {ctype: 'class'; id:string; name:string; data_properties:DataProperties[], object_properties:ObjectProperties[]}
  | {ctype: 'resource'; id:string; name:string; type:'resoure'|'volatile_resource'}
  | {ctype: 'rule'; id:string; name:string; properties:Record<string, string>; when:Antecedent[]; then:Consequent[]}
  ;

export type DataProperties = {id:string; name:string; type:LiteralType|'resource'; is_array:boolean; is_required:boolean};
export type ObjectProperties = {id:string; name:string; type:'resource'; is_array:boolean; is_required:boolean};

export type Antecedent = {id:string; is_not:boolean; subject:string; predicate:string; object:AntecedentObject};

export type LiteralType = 'int' | 'uint' | 'double' | 'date' | 'text' | 'bool';
