# Refactoring the Start Message

Refactoring the Start Message to simplify mapping and eliminate the mapping rules 
framework.

Currently the USI rules requires the following rule construct that can be easily moved 
to simple go functions that can be applied when the data is asserted in the rete session.

## Functions required for mapping

### to_upper

Transform the input data into upper case, example:
```
[n=ZZ19, s=100]: (?clm01 rdf:type hc_usi:USIClaim).(?clm01 hc_usi:originalRenderingProviderState ?v)  ->  (?clm01 hc_usi:renderingProviderState (to_upper ?v))
```
In that example, do we still need hc_usi:originalRenderingProviderState?

### to_zip5

Transform input data into a 5 digits zip code, replacing following rules:
```
[MZP010]: (?clm01 rdf:type hc_usi:USIClaim).(?clm01 M_memberZip ?v).[((length_of ?v) == 5)]                            ->  (?clm01 memZipLocal ?v)
[MZP020]: (?clm01 rdf:type hc_usi:USIClaim).(?clm01 M_memberZip ?v).[((length_of ?v) == 9)]                            ->  (?clm01 memZipLocal (?v literal_regex zip5Regex))
[MZP030]: (?clm01 rdf:type hc_usi:USIClaim).(?clm01 M_memberZip ?v).[((length_of ?v) <  5)]                            ->  (?clm01 memZipLocal ((to_int ?v) apply_format FORMAT_STRING))
[MZP040]: (?clm01 rdf:type hc_usi:USIClaim).(?clm01 M_memberZip ?v).[(((length_of ?v) > 5) and ((length_of ?v) < 9))]  ->  (?clm01 memZipLocal (((to_int ?v) apply_format FORMAT_STRING9) literal_regex zip5Regex))

[MZP050]: (?clm01 rdf:type hc_usi:USIClaim).(?clm01 memZipLocal ?v).[not (is_no_value ?v)]  -> (?clm01 hc_usi:memberZip ?v)
```

### reformat0

Transform the input data to be 9 digits, padding 0 on left as needed. Assuming input are digits
only. The number of digits (here 9) is a parameter. Replacing following rules:
```
[RPTID010]: (?clm01 rdf:type hc_usi:USIClaim).(?clm01 M_renderingProviderTaxID ?v)  ->  (?clm01 rprovTinLocal ((to_int ?v) apply_format FORMAT_STRING9))
[RPTID020]: (?clm01 rdf:type hc_usi:USIClaim).(?clm01 rprovTinLocal ?v).[not (is_no_value ?v)]  ->  (?clm01 hc_usi:renderingProviderTaxID ?v)
```

### apply_regex

Transform the input data by applying a regex expression. This often use a default value
if the regex does not match. Note will need to combine 2 regex for hcpcsRegex into a
single regex. Example of rule:
```
[XHCPCS010]: (?config rdf:type usi_sm:RRConfig).(?config usi_sm:hcpcsRegex ?hcpcsRegex).(?clm01 rdf:type hc_usi:USIClaim).(?clm01 M_hcpcs ?v)  ->  (?clm01 hcpcsLocal (?v literal_regex ?hcpcsRegex))
[XHCPCS020]: (?clm01 rdf:type hc_usi:USIClaim).(?clm01 hcpcsLocal ?v).[not (is_no_value ?v)] -> (?clm01 hc_usi:hcpcs ?v)
```

### scale_units

Transform input data, assumed to be castable to double, by scaling the number by applying
a divisor and rounding up to next integral number (ceiling). The divisor is an argument.
The rounder is 0.5.
This would replace the following rules, note this is using a default:
```
[XUNTS010]: (?config rdf:type usi_sm:RRConfig).(?config usi_sm:amtDivisor ?amtDivisor).(?config usi_sm:amtRounder ?amtRounder).(?clm01 rdf:type hc_usi:USIClaim).(?clm01 M_units ?v)  ->  (?clm01 hc_usi:units (to_real ((to_int (((to_real ?v) / ?amtDivisor) + ?amtRounder)))))
```

### parse_amount

Transform the input data by applying a divisor (as argument which can be 1), the input is assumed to be an amount, a negative amount may be expressed with parentesis.
Replacing the following rules:
```
[XCPAMT010]: (?clm01 rdf:type hc_usi:USIClaim).(?clm01 M_amountCopay ?v)  ->  (?clm01 tempAmtCopay (parse_usd_currency ?v))
[XCPAMT020]: (?config rdf:type usi_sm:RRConfig).(?config usi_sm:amtDivisor ?amtDivisor).(?clm01 rdf:type hc_usi:USIClaim).(?clm01 tempAmtCopay ?v).[not (is_no_value ?v)]  -> (?clm01 hc_usi:amountCopay ((to_real ?v) / ?amtDivisor))
[XCPAMT030]: (?clm01 rdf:type hc_usi:USIClaim).(?clm01 tempAmtCopay ?v).[(is_no_value ?v)]      -> (?clm01 hc_usi:amountCopay d00)
```

## Adding default value

Many data element requires default value when the data is either not available or invalid in the input.

## Specifying the mapping and data transformation as a table

INPUT_COLUMN | DATA_PROPERTY | FUNCTION | ARGUMENT | DEFAULT|
-------------|---------------|----------|----------|--------|
"1-HierarchyLevel1(MostSummarized)" | "hc_usi:planID" | | | |
"100-PaidAmount" | "hc_usi:AmountPaid" | parse_amount | 100 | "0" |
"22-Employee'sZipCode" | "hc_usi:memberZip" | to_zip5 | | |
"23-Employee'sState" | "hc_usi:memberState" | to_upper | | |
"46-ServicingProvider'sTaxIDNumber(TIN)" | "hc_usi:renderingProviderTaxID" | reformat0 | 9 | |
"68-Line-LevelProcedureCode(CPT,HCPCS,ADA,CDT)*" | "hc_usi:cptCode" | apply_regex | "(\d{5})" | |
"68-Line-LevelProcedureCode(CPT,HCPCS,ADA,CDT)*" | "hc_usi:hcpcs" | apply_regex | "([A-Z]\d{4}|0\d{3}T)" | |
"81-Number/UnitsofService" | "hc_usi:units" | scale_units | 100 | |
"84-NetSubmittedExpense***" | "hc_usi:amountBilled" | parse_amount | 100 | 0 |
"96-DeductibleAmount" | "hc_usi:amountDeductible" | parse_amount | 100 | 0 |

## Rule configuration parameters
Rules can be configures with parameters that can be specified at runtime. These parameters
are associated with a named subject. These configuration parameters are added to the
metadata triples contained in the workspace db.

The configuration parameters are specified as a table:

SUBJECT | PREDICATE | OBJECT | TYPE
--------|-----------|--------|-----
"usi_sm:RRConfig" | "usi_sm:isCommercialIndicator" | "true" | "bool"
"usi_sm:RRConfig" | "usi_sm:startDOS" | "2019-10-01" | "date"
"usi_sm:RRConfig" | "usi_sm:startDOS" | "2019-10-01" | "date"
"usi_sm:RRConfig" | "usi_sm:codesInContractExclusion" | "CODE1" | "text"
"usi_sm:RRConfig" | "usi_sm:codesInContractExclusion" | "CODE2" | "text"

## Database Schema for Process Execution Context (Start Message)
Main table is `pipeline_config`:
  - `client` is a client name or code for reference only.
  - `description` for contextual information
  - `main_entity_rdf_type` is main entity for the load or merge process

The `process_input` table defines the input tables in which
we loaded the csv files:
  - `table_name` is the table name where the data is
  - `entity_rdf_type` is the rdf type of the entity being loaded

`process_mapping` table defines the mapping and data transformations being made
as described above.

`rule_config` tables defines the rule configuration triples as described above.

`process_merge` table defines the merge of transactional tables into a main
table:
  - `entity_rdf_type` is the entity being merged.
  - `query_rdf_property_list` comma-separated list of rdf property to select
  - `grouping_rdf_property` property used for synchronizing the merge with the
  main table.

```
DROP TABLE IF EXISTS pipeline_config;
CREATE TABLE IF NOT EXISTS pipeline_config (
    key SERIAL PRIMARY KEY  ,
    client text  ,
    description text  ,
    main_entity_rdf_type text ,
    last_update timestamp without time zone DEFAULT now() NOT NULL
);

DROP TABLE IF EXISTS process_input;
CREATE TABLE IF NOT EXISTS process_input (
    key SERIAL PRIMARY KEY  ,
    process_key integer REFERENCES pipeline_config ON DELETE CASCADE ,
    table_name text  ,
    entity_rdf_type text ,
    UNIQUE (process_key, table_name)
);
CREATE INDEX IF NOT EXISTS process_input_process_key_idx ON process_input (process_key);

DROP TABLE IF EXISTS process_mapping;
CREATE TABLE IF NOT EXISTS process_mapping (
    process_input_key integer REFERENCES process_input ON DELETE CASCADE ,
    input_column text  ,
    data_property text  ,
    function_name text  ,
    argument text  ,
    default text ,
    PRIMARY KEY (process_input_key, input_column, data_property)
);
CREATE INDEX IF NOT EXISTS process_mapping_process_input_key_idx ON process_mapping (process_input_key);

DROP TABLE IF EXISTS rule_config;
CREATE TABLE IF NOT EXISTS rule_config (
    process_key integer REFERENCES pipeline_config ON DELETE CASCADE ,
    subject text  ,
    predicate text  ,
    object text  ,
    type text  ,
    default text 
);
CREATE INDEX IF NOT EXISTS rule_config_process_key_idx ON rule_config (process_key);

DROP TABLE IF EXISTS process_merge;
CREATE TABLE IF NOT EXISTS process_merge (
    process_key integer REFERENCES pipeline_config ON DELETE CASCADE ,
    entity_rdf_type text  ,
    query_rdf_property_list text ,
    grouping_rdf_property text
);
CREATE INDEX IF NOT EXISTS process_merge_process_key_idx ON process_merge (process_key);
```

## Running the server process with logging

```bash
GLOG_v=1 ./server -dsn="postgresql://postgres:<PWD>@<IP>:5432/postgres"  -lookupDb test_data/lookup_test1.db -outTables=hc__claim -pcKey=1 -ruleset=workspace_test1.jr -sessId=sess1 -workspaceDb=test_data/workspace_test1.db -poolSize=1
```
