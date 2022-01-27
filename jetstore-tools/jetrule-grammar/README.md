# [JetStore](https://github.com/artisoft-io/jetstore/) Rule Grammar

This section contains the [AntlR4](https://github.com/antlr/antlr4) grammar 
definition of [JetStore](https://github.com/artisoft-io/jetstore/) canonical rule language.

---

## Components of the target rule language

We are looking to use a canonical rule language that is based on 
[SparQL](https://www.w3.org/TR/sparql11-overview/) language. 
As a guide to write out the grammar, we list here same sample rules in the next section
in increasing order of complexity.

### Importing Other Files
Rule file can import other files using the import command. This command is part of the 
pre-processing and line containing the import command is replaced by the content
of the file located by the argument of the import command. The import command syntax is:

```
import "file location"
```

### Defining Comments
Comments start with `#` and extend to the end of the line. Example:
```
################################################
# Import Section
# ---------------------------------------------
import "file1"
import "file2"  # This is a comment
```

### Using Lookup Tables
Lookup table are defined using a simple structure. The lookup table data
reside in the JetStore master node database in a regular table. 
The schema of the table (column name and type) is obtained from the database.
Here's an example:
```
lookup_table NdcInfoTable {
  table_name: code_table22,
  key: [key_column1, key_column2],
  columns: [val_column1, val_col2, val_col3]
};
```
where NdcInfoTable is the resource name of the table, meaning it is the name
of the resource used to reference the lookup in the `lookup` function as explained
below. The `table_name` field indicate the name of the table in the in the JetStore 
master node database where the data reside.

### JetStore Inferrence Engine Configuration
JetStore configuration for the inferrence engine is using a simple structure:
```
jetstore_configuration {
  mode: release,           # release or debug
  max_iterations: 1,       # to support looping mechanisim
  max_rule_fireing: 50000  # to stop conflicting rules that contains vicious cycle
};
```
This configuration is not in the grammar, this is external configuration settings.

### Defining Constants
We often need to have reference to defined constant.
All classes and properties of the Domain Model have a corresponding
resource defined.
Other resources can be defined to be used as volatile properties,
meaning they are not part of the persistent model. 

#### Defining Literals
```
  double half = 0.5;
  text AgeGroup1 = "AGE_GROUP1", AgeGroup2 = "AGE_GROUP2", AgeGroup3 = "AGE_GROUP3";
  int token1 = -574;
  uint limit1 = 6574;
  long longToken1 = -574;
  ulong longLimit1 = 6574;

  date date1 = "1/13/2022", date2 = today();
  time aptTime1 = "1/13/2022 01:20:30.123";
  time aptTime2 = now();
  duration elapsedTime1 = "1234567:20:30.123";

  text NpiFormat = "%010u";
  text ClaimIdFormat = "%018u";
  text MBR_ID_H_FORMAT = "%06u";
  text MBR_ID_L_FORMAT = "%03u";
```

#### Defining Resources
Defining named resources:
```
  resource nbr_providers = "nbr_providers", ck:Value = "ck:value";
```
Defining resources for use as volatile properties:
```
  volatile_resource allClaimsInPeriod = "allClaimsInPeriod";
  volatile_resource allMedicalClaimsInPeriod = "allMedicalClaimsInPeriod";
```
Which is same as (this is for backward compatibility and may change):
```
  resource allClaimsInPeriod = "_0:allClaimsInPeriod";
  resource allMedicalClaimsInPeriod = "_0:allMedicalClaimsInPeriod";
```

### Defining Constant Triples
We often need to have reference to non-mutable triples. These are define
at the scope of the RuleSet. They can make reference to functions and resources previously defined:
```
resource iValue = create_uuid_resource();
const_triple t3(iValue, rdf:type, ck:Value);
const_triple t3(iValue, ck:key, "Value1");
```

### Sample JetStore Rules

Here's about the simplest rule you can write:
```
[rule1,s=10,o=false]: (?s jet:has_node ?n1) -> (?s jet:sub_node ?n1);
```
An example with more antecedent terms:
```
[rule2]:
  (?rc1 rdf:type jet:ReportCard).
  (?pat1 linkedRC ?rc1).
  (?clm01 jet:claimForPatient ?pat1).
  (?clm01 rdf:type jet:MedicalClaim)
  ->
  (?rc1 allClaimsInPeriod ?clm01).
  (?rc1 allMedicalClaimsInPeriod ?clm01)
;
```

#### Negation of Antecedent
Negation of an antecedent term can be expressed using the keyword `not` before the
antecedent term:
```
[not_rule]:
  (?rc1 rdf:type jet:Claim).
  not (?crc1 rdf:type jet:MedicalClaim)
  ->
  (?rc1 rdf:type jet:NonMedicalClaim)
;
```

#### Filter Attached to Antecedent
Also, antecedent terms can have filter expression to filter out some terms being matched and 
also expression to to evaluate the objects in the consequent terms:
```
[rule3]:
  (?rc rdf:type jet:ReportCard).
  (?rc allOpioidClaims1yrs ?clm01).
  (?clm01 jet:daysOfSupply ?days).
  (?clm01 jet:fillQuantity ?qty).
  (?clm01 medd_factor ?factor).
  (?clm01 drug_strenght ?strength).
  [?days > 0]
  ->
  (?clm01 jet:claimMEDD (?factor * ?strength * ?qty  / ?days) )
;
```
#### Expression of Operators
Expression are built using unary and binary operators and functions.
The oprators and functions in expression do not have a priority assigned to 
them, as a results, the
expression perform a fold left. Therefore the expression is equivalent to
```
(((?factor * ?strength) * ?qty)  / ?days)
```
Filter expression can use built-in operators, for example the operator `exist` is a 
binary  operator that can
be used as `(s exist p)` which returns true if the the subject s has a triple 
involving the predicate p. 
Similarily the operator `exist_not` is a binary operator that returns true
when the subject s does not have any triples with predicate p: `(s exist_not p)`.
Here's an example
```
[rule2]:
  (?rc1 rdf:type jet:ReportCard).
  [?rc1 exist_not allClaimsInPeriod]
  ->
  (?rc1 has_no_claims true)
;
```
The same rule can be written as:
```
[rule2]:
  (?rc1 rdf:type jet:ReportCard).
  [not (?rc1 exist allClaimsInPeriod)]
  ->
  (?rc1 has_no_claims true)
;
```
where `not` is a unary operator.

#### Using Functions
An important built-in function is a table lookup function, simply called `lookup`:
```
[rule3]:
  (?rc rdf:type jet:ReportCard).
  (?rc jet:pharmacyClaim ?clm01).
  (?clm01 jet:drugNDC ?ndc)
  ->
  (?clm01 drugInfoRow (lookup(NdcInfoTable, ?ndc)) )
;
```
The lookup function return an entity of type `jet:Row`. The extra
parenthesis are not needed when the object of a consequent term is the result of a function
or a unary operator call, so the previous rule can be written as:
```
[rule3]:
  (?rc rdf:type jet:ReportCard).
  (?rc jet:pharmacyClaim ?clm01).
  (?clm01 jet:drugNDC ?ndc)
  ->
  (?clm01 drugInfoRow lookup(NdcInfoTable, ?ndc) )
;
```
#### Creating Entities and Inlining Literals
```
[PRC01]:
  (?state rdf:type jet:WorkingMemory).
  (?state provider_row ?row).
  (?row NPI ?npi)
  ->
  (?row newProvider create_entity (jet:Provider, "text(P)" + ?npi))
;
```
#### Throwing Exception
```
[Throw01]:
  (?state rdf:type jet:WorkingMemory).
  (?state has_value ?v).
  [?v > 1.0]
  ->
  (?row jet:throw jet:exception("Error, v > 1.0")
;
```
#### Reporting Errors and Warnings
```
[Err1]:
  (?state rdf:type jet:WorkingMemory).
  (?state has_value ?v).
  [?v > 1.0]
  ->
  (?row jet:log jet:error("Error, v > 1.0")
;
[Warn1]:
  (?state rdf:type jet:WorkingMemory).
  (?state has_value ?v).
  [?v > 1.0]
  ->
  (?row jet:log jet:warning("Error, v > 1.0")
;
```

### JetStore Operators

#### JetStore Binary Operators

  | Operators with literal as operand |
  | ------------------------------------ |
  | < |
  | <= |
  | > |
  | >= |
  | == |
  | != |
  | and |
  | or |
  | bit_and |
  | bit_or |
  | xor |
  | + |
  | - |
  | * |
  | / |
  | min |
  | max |
  | pow |
  | % |
  | eq_case |
  | eq_no_case |
  | starts_with |
  | ends_with |
  | contains |
  | strip_char |
  | is_type |
  | get_age_as_of |
  | levenshtein_distance |
  | literal_regex |
  | apply_format |
  | has |
  | has_not |

  | Operators with resource as operand |
  | ------------------------------------ |
  | same_as |
  | different_from |
  | exist |
  | exist_not |
  | get_cardinality |
  | sum_values |
  | avg_values |
  | min_value |
  | max_value |
  | min_multi_value |
  | max_multi_value |
  | sorted_head |
  | regex |
  | lookup |
  | multi_lookup |
  | to_regex_date |
  | format |
  | pick_one |
  | cast_to_range_of |

#### JetStore Unary Operators

  | Operators with literal as operand |
  | ------------------------------------ |
  | floor |
  | ceil |
  | not |
  | abs |
  | sqrt |
  | exp |
  | log |
  | log10 |
  | to_upper |
  | to_lower |
  | length_of |
  | trim |
  | get_type |
  | get_key |
  | get_age |
  | to_metaphone |
  | to_date |
  | to_time |
  | to_duration |
  | to_int |
  | to_uint |
  | to_long |
  | to_ulong |
  | to_double |
  | to_string |
  | parse_usd_currency |
  | to_bool |
  | rand_int |
  | rand_uint |
  | rand_1 |
  | rand_long |
  | rand_ulong |

  | Operators with resource as operand |
  | ------------------------------------ |
  | is_bnode |
  | is_literal |
  | is_resource |
  | is_no_value |
  | is_literal |
  | is_finite |
  | is_infinte |
  | is_nan |
  | wrap |
  | get_name |

#### JetStore Functions

  | Functions |
  | --------- |
  | regex |
  | lookup |
  | lookup_rand |
  | multi_lookup |
  | multi_lookup_rand |
  | format |
  | today |
  | now |
  | create_entity |
  | create_uuid_entity |
  | date_details |
  | time_details |
  | date_time_details |
  | create_bnode |
  | create_uuid_resource |
  | create_resource |
  | create_literal |
